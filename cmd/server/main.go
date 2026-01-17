package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dbpkg "kuberde/pkg/db"
	"kuberde/pkg/models"
	"kuberde/pkg/repositories"

	"github.com/Nerzal/gocloak/v13"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	agentSessions = make(map[string]*yamux.Session)
	sessionsMu    sync.RWMutex

	oidcVerifier     *oidc.IDTokenVerifier
	oauth2Config     oauth2.Config
	keycloakRealmURL string // Keycloak public realm URL for logout

	agentStats = make(map[string]*AgentStats)
	statsMu    sync.RWMutex

	k8sClientset  kubernetes.Interface
	dynamicClient dynamic.Interface
	frpAgentGVR   = schema.GroupVersionResource{Group: "kuberde.io", Version: "v1beta1", Resource: "rdeagents"}

	scaleUpMutex      sync.Mutex              // Prevent concurrent scale-up attempts for same agent
	scaleUpInProgress = make(map[string]bool) // Track which agents are being scaled up

	db *gorm.DB

	// Repository instances
	resourceConfigRepo repositories.ResourceConfigRepository
	userQuotaRepo      repositories.UserQuotaRepository

	// Domain configuration
	frpURL      = "https://frp.byai.uk" // Default service URL
	agentDomain = "frp.byai.uk"         // Default agent domain (used for *.frp.byai.uk)

	// Agent configuration
	agentServerURL  = "ws://kuberde-server:8080/ws" // Default WebSocket URL for agents
	agentAuthSecret = "kuberde-agents-auth"         // Default auth secret name for agents

	// Namespace configuration
	kuberdeNamespace = "kuberde" // Default namespace for PVCs and RDEAgents

	// Keycloak Admin Client
	keycloakClient *gocloak.GoCloak
	adminToken     *gocloak.JWT
	adminTokenMu   sync.RWMutex
)

type contextKey string

const idTokenKey contextKey = "idToken"

// isSecureDeployment checks if the deployment uses HTTPS
func isSecureDeployment() bool {
	return strings.HasPrefix(frpURL, "https://")
}

func init() {
	// Read service URL from environment
	if url := os.Getenv("KUBERDE_PUBLIC_URL"); url != "" {
		frpURL = url
	}

	// Read agent domain from environment
	if domain := os.Getenv("KUBERDE_AGENT_DOMAIN"); domain != "" {
		agentDomain = domain
	} else {
		// If not set, derive from KUBERDE_PUBLIC_URL
		if u, err := url.Parse(frpURL); err == nil {
			agentDomain = u.Hostname()
		}
	}

	// Read agent server URL from environment
	if serverURL := os.Getenv("KUBERDE_AGENT_SERVER_URL"); serverURL != "" {
		agentServerURL = serverURL
	}

	// Read agent auth secret from environment
	if authSecret := os.Getenv("KUBERDE_AGENT_AUTH_SECRET"); authSecret != "" {
		agentAuthSecret = authSecret
	}

	// Read namespace from environment, or auto-detect from service account
	if ns := os.Getenv("KUBERDE_NAMESPACE"); ns != "" {
		kuberdeNamespace = ns
	} else {
		// Try to read from service account namespace file (when running in K8s)
		if nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			kuberdeNamespace = strings.TrimSpace(string(nsBytes))
		}
		// Otherwise, use default "kuberde"
	}
}

// getRootDomain extracts the root domain for cookie sharing
// Examples:
//   - "frp.byai.uk" -> ".frp.byai.uk"
//   - "www.kuberde.com" -> ".kuberde.com"
//   - "api.kuberde.com" -> ".kuberde.com"
//   - "localhost" -> "localhost"
func getRootDomain(hostname string) string {
	if hostname == "localhost" || strings.Contains(hostname, "127.0.0.1") {
		return hostname
	}

	// For proper cookie sharing, prepend dot for wildcard domain
	if !strings.HasPrefix(hostname, ".") {
		return "." + hostname
	}
	return hostname
}

// getAgentDomainSuffix returns the suffix for agent subdomains
// Examples:
//   - "frp.byai.uk" -> ".frp.byai.uk"
//   - "kuberde.com" -> ".kuberde.com"
//   - "agent.kuberde.com" -> ".agent.kuberde.com"
func getAgentDomainSuffix() string {
	if !strings.HasPrefix(agentDomain, ".") {
		return "." + agentDomain
	}
	return agentDomain
}

type AgentStats struct {
	Online            bool      `json:"online"`
	LastActivity      time.Time `json:"last_activity"`
	HasActiveSession  bool      `json:"hasActiveSession"` // For TTL operator
	ActiveConnections int       `json:"activeConnections"`
}

func (s *AgentStats) UpdateActivity() {
	s.LastActivity = time.Now()
}

func updateActiveConnections(agentID string, delta int) {
	statsMu.Lock()
	defer statsMu.Unlock()
	if stats, ok := agentStats[agentID]; ok {
		stats.ActiveConnections += delta
		if stats.ActiveConnections < 0 {
			stats.ActiveConnections = 0
		}
		// If user connects, that's activity
		if delta > 0 {
			stats.LastActivity = time.Now()
		}
	}
}

func runActivityMonitor() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		statsMu.Lock()
		for _, stats := range agentStats {
			if stats.Online && stats.ActiveConnections > 0 {
				stats.LastActivity = time.Now()
			}
		}
		statsMu.Unlock()
	}
}

// --- Database Models ---

type TunnelConnection struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agentId"`
	UserID    string    `gorm:"index" json:"user"`
	ClientIP  string    `json:"clientIp"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	BytesIn   int64     `json:"bytesIn"`  // Bytes sent TO agent (User -> Agent)
	BytesOut  int64     `json:"bytesOut"` // Bytes received FROM agent (Agent -> User)
	Active    bool      `json:"active"`
	Protocol  string    `json:"protocol"` // "tcp", "http", "ssh"
}

type TrafficCounter struct {
	ReadWriter io.ReadWriter
	BytesRead  *int64
	BytesWrite *int64
}

func (t *TrafficCounter) Read(p []byte) (n int, err error) {
	n, err = t.ReadWriter.Read(p)
	if n > 0 && t.BytesRead != nil {
		atomic.AddInt64(t.BytesRead, int64(n))
	}
	return
}

func (t *TrafficCounter) Write(p []byte) (n int, err error) {
	n, err = t.ReadWriter.Write(p)
	if n > 0 && t.BytesWrite != nil {
		atomic.AddInt64(t.BytesWrite, int64(n))
	}
	return
}

// --- Agent Naming Helper Functions ---

// sanitizeK8sName converts a string to a valid Kubernetes resource name
// - Converts to lowercase
// - Replaces invalid characters with hyphens
// - Removes consecutive hyphens
// - Trims leading/trailing hyphens
// - Limits length to maxLen (default 50 if maxLen <= 0)
func sanitizeK8sName(name string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 50
	}

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters (anything not a-z, 0-9, or -) with hyphen
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	name = result.String()

	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Trim leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Limit length
	if len(name) > maxLen {
		name = name[:maxLen]
		name = strings.TrimRight(name, "-")
	}

	// Ensure not empty
	if name == "" {
		name = "default"
	}

	return name
}

// generateAgentName creates a unique agent name using userName, workspaceName, serviceName and hash
// Format: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash8}
// The hash ensures uniqueness even if names collide
func generateAgentName(userID, userName, workspaceID, workspaceName, serviceName string) string {
	// Sanitize each component
	userPart := sanitizeK8sName(userName, 20)
	workspacePart := sanitizeK8sName(workspaceName, 20)
	servicePart := sanitizeK8sName(serviceName, 20)

	// Generate 8-character hash from IDs to ensure uniqueness
	hashInput := userID + workspaceID + serviceName
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := fmt.Sprintf("%x", hash[:4]) // First 4 bytes = 8 hex chars

	// Construct name: kuberde-agent-{user}-{workspace}-{service}-{hash}
	agentName := fmt.Sprintf("kuberde-agent-%s-%s-%s-%s", userPart, workspacePart, servicePart, hashStr)

	return agentName
}

// generateWorkspacePVCName generates a PVC name for a workspace
// Format: kuberde-{userName}-{workspaceName}-{hash8}
func generateWorkspacePVCName(userID, userName, workspaceID, workspaceName string) string {
	// Sanitize each component
	userPart := sanitizeK8sName(userName, 20)
	workspacePart := sanitizeK8sName(workspaceName, 20)

	// Generate 8-character hash from IDs to ensure uniqueness
	hashInput := userID + workspaceID
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := fmt.Sprintf("%x", hash[:4]) // First 4 bytes = 8 hex chars

	// Construct name: kuberde-{user}-{workspace}-{hash}
	pvcName := fmt.Sprintf("kuberde-%s-%s-%s", userPart, workspacePart, hashStr)

	return pvcName
}

// --- OIDC & Auth Logic ---

// Custom Transport to route Public URL requests to Internal K8s Service
type K8sInternalTransport struct {
	Transport   http.RoundTripper
	PublicHost  string
	InternalURL string
}

func (t *K8sInternalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == t.PublicHost {
		// Replace Host with internal service
		// Parse internal URL to get scheme and host
		// Assuming InternalURL is simpler like "http://keycloak:8080"
		switch {
		case strings.HasPrefix(t.InternalURL, "http://"):
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(t.InternalURL, "http://")
		case strings.HasPrefix(t.InternalURL, "https://"):
			req.URL.Scheme = "https"
			req.URL.Host = strings.TrimPrefix(t.InternalURL, "https://")
		default:
			// Fallback
			req.URL.Scheme = "http"
			req.URL.Host = t.InternalURL
		}

		// Set forwarding headers based on the public URL scheme
		// For HTTP deployments, use "http"; for HTTPS, use "https"
		if strings.HasPrefix(t.PublicHost, "https://") || strings.Contains(t.PublicHost, ":443") {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
		req.Header.Set("X-Forwarded-Host", t.PublicHost)
	}
	return t.Transport.RoundTrip(req)
}

func initAuth() {
	ctx := context.Background()

	// Internal URL: http://keycloak:8080
	keycloakURL := os.Getenv("KEYCLOAK_URL")
	if keycloakURL == "" {
		keycloakURL = "http://localhost:8080"
	}

	// External URL: https://sso.byai.uk
	keycloakPublicURL := os.Getenv("KEYCLOAK_PUBLIC_URL")
	if keycloakPublicURL == "" {
		keycloakPublicURL = keycloakURL
	}

	publicRealmURL := fmt.Sprintf("%s/realms/kuberde", keycloakPublicURL)
	keycloakRealmURL = publicRealmURL // Set global for logout
	log.Printf("Initializing OIDC provider with Public URL: %s (internally routed to %s)", publicRealmURL, keycloakURL)

	// Create custom HTTP client that routes Public URL -> Internal URL
	// Extract Host from Public URL
	publicHost := ""
	if strings.HasPrefix(keycloakPublicURL, "https://") {
		publicHost = strings.TrimPrefix(keycloakPublicURL, "https://")
	} else {
		publicHost = strings.TrimPrefix(keycloakPublicURL, "http://")
	}
	// Remove path/port if any needed?
	// SplitHostPort logic is tricky if port is missing.
	// Just use string matching for simplicity.

	// Ensure we handle port if present in Public URL (unlikely for sso.byai.uk but possible)
	// Actually, req.URL.Host includes port if present.

	// Simplified extraction:
	// "https://sso.byai.uk" -> "sso.byai.uk"
	// "http://127.0.0.1:8080" -> "127.0.0.1:8080"

	customTransport := &K8sInternalTransport{
		Transport:   http.DefaultTransport,
		PublicHost:  publicHost,
		InternalURL: keycloakURL,
	}

	customClient := &http.Client{
		Transport: customTransport,
	}

	// Inject client into context
	oidcCtx := oidc.ClientContext(ctx, customClient)

	var provider *oidc.Provider
	var err error

	// Retry loop for OIDC provider initialization (3 attempts)
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		provider, err = oidc.NewProvider(oidcCtx, publicRealmURL)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			log.Printf("WARNING: Failed to initialize OIDC provider (attempt %d/%d): %v. Retrying in 5s...", i+1, maxRetries, err)
			time.Sleep(5 * time.Second)
		}
	}

	if err != nil {
		log.Fatalf("FATAL: Failed to initialize OIDC provider after %d retries: %v. Pod will restart.", maxRetries, err)
	}

	oidcVerifier = provider.Verifier(&oidc.Config{
		SkipIssuerCheck:   true, // We are matching issuer now, but safe to keep
		SkipClientIDCheck: true,
	})

	// Configure OAuth2 for Code Flow
	// We use the Provider's endpoint, which contains the Public URL (Issuer)
	// This is correct for the browser to redirect to.
	oauth2Config = oauth2.Config{
		ClientID:    "kuberde-cli",
		RedirectURL: fmt.Sprintf("%s/auth/callback", frpURL),
		Endpoint:    provider.Endpoint(),
		Scopes:      []string{oidc.ScopeOpenID, "profile", "email"},
	}

	log.Println("OIDC Auth initialized successfully")
}

// initKeycloakAdmin initializes the Keycloak admin client for user management
func initKeycloakAdmin() error {
	keycloakURL := os.Getenv("KEYCLOAK_URL")
	if keycloakURL == "" {
		keycloakURL = "http://keycloak:8080"
	}

	clientID := os.Getenv("KEYCLOAK_ADMIN_CLIENT_ID")
	clientSecret := os.Getenv("KEYCLOAK_ADMIN_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("missing Keycloak admin credentials (KEYCLOAK_ADMIN_CLIENT_ID or KEYCLOAK_ADMIN_CLIENT_SECRET)")
	}

	keycloakClient = gocloak.NewClient(keycloakURL)

	// Get initial admin token
	token, err := keycloakClient.LoginClient(
		context.Background(),
		clientID,
		clientSecret,
		"kuberde",
	)
	if err != nil {
		return fmt.Errorf("failed to login as admin: %w", err)
	}

	adminTokenMu.Lock()
	adminToken = token
	adminTokenMu.Unlock()

	// Start background token refresh
	go refreshAdminToken(clientID, clientSecret)

	log.Println("Keycloak admin client initialized successfully")
	return nil
}

// refreshAdminToken periodically refreshes the admin token
func refreshAdminToken(clientID, clientSecret string) {
	ticker := time.NewTicker(20 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		token, err := keycloakClient.LoginClient(
			context.Background(),
			clientID,
			clientSecret,
			"kuberde",
		)
		if err != nil {
			log.Printf("Failed to refresh admin token: %v", err)
			continue
		}

		adminTokenMu.Lock()
		adminToken = token
		adminTokenMu.Unlock()

		log.Println("Admin token refreshed successfully")
	}
}

// getAdminToken returns the current valid admin token
func getAdminToken() string {
	adminTokenMu.RLock()
	defer adminTokenMu.RUnlock()
	if adminToken == nil {
		return ""
	}
	return adminToken.AccessToken
}

func validateToken(r *http.Request) (*oidc.IDToken, error) {
	if oidcVerifier == nil {
		return nil, fmt.Errorf("OIDC provider not initialized")
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Try query param
		authHeader = r.URL.Query().Get("token")
		if authHeader == "" {
			return nil, fmt.Errorf("missing Authorization header")
		}
		authHeader = "Bearer " + authHeader
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		log.Printf("DEBUG: Invalid Auth Header. Len: %d, Content: %s...", len(parts), func() string {
			if len(authHeader) > 10 {
				return authHeader[:10]
			}
			return authHeader
		}())
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	return oidcVerifier.Verify(context.Background(), parts[1])
}

func validateCookie(r *http.Request) (*oidc.IDToken, error) {
	if oidcVerifier == nil {
		return nil, fmt.Errorf("OIDC provider not initialized")
	}
	cookie, err := r.Cookie("kuberde_session")
	if err != nil {
		return nil, err
	}
	return oidcVerifier.Verify(context.Background(), cookie.Value)
}

// --- Handlers ---

// handleHealthz handles liveness probe (basic server health)
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "kuberde-server",
	}); err != nil {
		log.Printf("Failed to encode healthz response: %v", err)
	}
}

// handleReadyz handles readiness probe (checks dependencies)
func handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check database connection
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"status": "not ready",
				"reason": "database connection failed",
			}); err != nil {
				log.Printf("Failed to encode readyz response: %v", err)
			}
			return
		}
	}

	// Check OIDC verifier is initialized
	if oidcVerifier == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "not ready",
			"reason": "OIDC verifier not initialized",
		}); err != nil {
			log.Printf("Failed to encode readyz response: %v", err)
		}
		return
	}

	// All checks passed
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ready",
		"service": "kuberde-server",
	}); err != nil {
		log.Printf("Failed to encode readyz response: %v", err)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateRandomState()

	// Check for return_url query param
	returnURL := r.URL.Query().Get("return_url")
	if returnURL != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "frp_return_url",
			Value:    returnURL,
			HttpOnly: true,
			Secure:   isSecureDeployment(),
			Path:     "/",
			MaxAge:   300, // 5 minutes
		})
	}

	// Ideally store state in cookie to validate in callback
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HttpOnly: true,
		Secure:   isSecureDeployment(),
		Path:     "/auth/callback",
	})
	http.Redirect(w, r, oauth2Config.AuthCodeURL(state), http.StatusFound)
}

// ensureUserExists creates user record in database if it doesn't exist
// Also creates default workspace for new users
func ensureUserExists(userID string, username string, email string, fullName string) error {
	if db == nil {
		// Database not initialized - this is a fatal error for on-demand user creation
		log.Printf("ERROR: Database not initialized, cannot create user %s", username)
		return fmt.Errorf("database not initialized")
	}

	userRepo := dbpkg.UserRepo()

	// Check if user already exists by ID
	existingUser, err := userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("failed to check user existence by ID: %w", err)
	}

	// User already exists with matching ID, skip creation
	if existingUser != nil {
		log.Printf("User %s already exists in database with matching ID", username)
		return nil
	}

	// Check if username already exists (could be old record with different ID)
	existingByUsername, err := userRepo.FindByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to check user existence by username: %w", err)
	}

	if existingByUsername != nil {
		// Username exists but with different ID - this is a data inconsistency
		log.Printf("WARNING: User %s exists with different ID (db: %s, keycloak: %s)",
			username, existingByUsername.ID, userID)

		// Check if the existing user has any workspaces
		workspaces, _ := dbpkg.WorkspaceRepo().FindByOwnerID(existingByUsername.ID, 1, 0)
		hasData := len(workspaces) > 0

		if !hasData {
			// No associated data - safe to delete and recreate with correct ID
			log.Printf("Deleting old user record %s (no associated data)", existingByUsername.ID)
			if err := userRepo.Delete(existingByUsername.ID); err != nil {
				log.Printf("ERROR: Failed to delete old user record: %v", err)
				return fmt.Errorf("user exists with different ID and cannot be updated")
			}
			// Continue to create new user below
		} else {
			// Has associated data - cannot safely recreate
			log.Printf("ERROR: User %s has associated data, cannot update ID from %s to %s",
				username, existingByUsername.ID, userID)
			// Return success but use existing ID - this prevents duplicate key error
			// The user can still log in and use their existing data
			return nil
		}
	}

	// Create new user
	newUser := &models.User{
		ID:       userID,
		Username: username,
		Email:    email,
		FullName: fullName,
	}

	if err := userRepo.Create(newUser); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("✓ Created user record: %s (%s)", username, userID)

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "create", "user", userID, "", ""); err != nil {
		log.Printf("WARNING: Failed to log audit entry: %v", err)
	}

	return nil
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Verify State
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token field in oauth2 token", http.StatusInternalServerError)
		return
	}

	// Verify and parse ID token
	idToken, err := oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Extract user claims
	var claims struct {
		Subject           string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Name              string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure user exists in database (auto-create on first login)
	if err := ensureUserExists(claims.Subject, claims.PreferredUsername, claims.Email, claims.Name); err != nil {
		log.Printf("WARNING: Failed to ensure user exists: %v", err)
		// Don't block login if database fails
	}

	// Derive cookie domain from agentDomain for wildcard cookie sharing
	// This allows cookies to work across main domain and agent subdomains
	// Examples:
	//   - Service: frp.byai.uk, Agent: *.frp.byai.uk -> Cookie Domain: .frp.byai.uk
	//   - Service: www.kuberde.com, Agent: *.kuberde.com -> Cookie Domain: .kuberde.com
	cookieDomain := getRootDomain(agentDomain)

	// Set Session Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "kuberde_session",
		Value:    rawIDToken, // In prod, encrypt this or use session store
		Path:     "/",
		Domain:   cookieDomain, // Wildcard cookie
		HttpOnly: true,
		Secure:   isSecureDeployment(),
		Expires:  oauth2Token.Expiry,
	})

	// Check for return URL
	returnCookie, err := r.Cookie("frp_return_url")
	redirectTarget := "/"
	if err == nil && returnCookie.Value != "" {
		redirectTarget = returnCookie.Value
		// Consume cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "frp_return_url",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
	}

	// Ignore return_url if it's the login page (prevent redirect loop)
	if redirectTarget == "/login" || redirectTarget == "/#/login" {
		redirectTarget = "/"
	}

	// Convert to HashRouter format if needed
	// Frontend uses HashRouter, so "/" should be "/#/"
	if redirectTarget == "/" {
		redirectTarget = "/#/"
	} else if len(redirectTarget) > 0 && redirectTarget[0] == '/' && !strings.HasPrefix(redirectTarget, "/#") {
		// Convert paths like "/workspaces" to "/#/workspaces"
		redirectTarget = "/#" + redirectTarget
	}

	// Redirect to original target or dashboard
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

func handleSubdomainProxy(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}

	// Extract Agent ID from subdomain
	// Examples:
	//   - user-alice-dev.frp.byai.uk -> user-alice-dev
	//   - user-bob-test.kuberde.com -> user-bob-test
	//   - my-service.agent.kuberde.com -> my-service
	suffix := getAgentDomainSuffix()
	if !strings.HasSuffix(host, suffix) {
		http.Error(w, fmt.Sprintf("Invalid host: must end with %s", suffix), http.StatusBadRequest)
		return
	}
	agentID := strings.TrimSuffix(host, suffix)

	// 1. Auth Check (Cookie)

	token, err := validateCookie(r)
	if err != nil {
		// Construct return URL
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		returnURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.Path)

		// Redirect to login on main domain with return_url
		loginURL := fmt.Sprintf("%s/auth/login?return_url=%s", frpURL, returnURL)

		http.Redirect(w, r, loginURL, http.StatusFound)
		return
	}

	// 2. Authorization Check (Owner or Admin)
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Sub               string `json:"sub"`
		RealmAccess       struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid claims", http.StatusForbidden)
		return
	}

	// Check if user is admin
	isAdmin := false
	for _, role := range claims.RealmAccess.Roles {
		if role == "admin" {
			isAdmin = true
			break
		}
	}

	// Admin can access any service
	if !isAdmin {
		// Non-admin users must own the service
		authorized := false

		// Check new naming convention: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash8}
		if strings.HasPrefix(agentID, "kuberde-agent-") {
			// Extract parts: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash8}
			agentIDTrimmed := strings.TrimPrefix(agentID, "kuberde-agent-")
			parts := strings.SplitN(agentIDTrimmed, "-", 4)
			if len(parts) >= 1 {
				agentUserName := parts[0]
				if agentUserName == claims.PreferredUsername {
					authorized = true
				}
			}
		}

		if !authorized {
			log.Printf("Access denied: user %s (ID: %s) attempted to access agent %s",
				claims.PreferredUsername, claims.Sub, agentID)
			http.Error(w, "Forbidden: You do not have permission to access this service", http.StatusForbidden)
			return
		}
	} else {
		// Admin is accessing a service - determine the owner and log it
		targetOwner := "unknown"

		// Extract owner from agent ID
		if strings.HasPrefix(agentID, "kuberde-agent-") {
			agentIDTrimmed := strings.TrimPrefix(agentID, "kuberde-agent-")
			parts := strings.SplitN(agentIDTrimmed, "-", 4)
			if len(parts) >= 1 {
				targetOwner = parts[0] // userName is the first part
			}
		} else if strings.HasPrefix(agentID, "kuberde-") {
			agentIDTrimmed := strings.TrimPrefix(agentID, "kuberde-")
			parts := strings.SplitN(agentIDTrimmed, "-", 3)
			if len(parts) >= 3 && db != nil {
				workspaceID := parts[1]
				workspace, err := dbpkg.WorkspaceRepo().FindByID(workspaceID)
				if err == nil && workspace != nil && workspace.Owner != nil {
					targetOwner = workspace.Owner.Username
				}
			}
		} else if strings.HasPrefix(agentID, "user-") {
			idParts := strings.SplitN(agentID, "-", 3)
			if len(idParts) >= 2 {
				targetOwner = idParts[1]
			}
		}

		// Only log if admin is accessing another user's service
		if targetOwner != claims.PreferredUsername && targetOwner != "unknown" {
			logAdminAccess(claims.Sub, claims.PreferredUsername, "http_proxy_access", "service", agentID, targetOwner)
		}
	}

	// 3. Proxy
	sessionsMu.RLock()
	sess, ok := agentSessions[agentID]
	sessionsMu.RUnlock()

	if !ok || sess.IsClosed() {
		log.Printf("DEBUG: Session not found or closed for agent %s (ok=%v). Checking scale status...", agentID, ok)
		// Check if agent is scaled down and attempt scale-up
		scaledDown, err := isAgentScaledDown(agentID)
		if err != nil {
			log.Printf("Failed to check agent scale status for %s: %v", agentID, err)
			http.Error(w, "Agent status check failed", http.StatusInternalServerError)
			return
		}

		if scaledDown {
			log.Printf("Agent %s is scaled down (HTTP request), attempting scale-up...", agentID)

			// Attempt scale-up (HTTP requests have shorter timeout than SSH)
			err := scaleUpDeployment(agentID, 30)
			if err != nil {
				log.Printf("Scale-up failed for agent %s: %v", agentID, err)
				http.Error(w, "Agent startup failed", http.StatusBadGateway)
				return
			}
		}

		// Re-check session after scale-up (or if it was already up but missing session)
		sess, ok = waitForSessionReady(agentID, 10)

		if !ok || sess == nil || sess.IsClosed() {
			http.Error(w, "Agent offline", http.StatusBadGateway)
			return
		}
	}

	// Track Active Connection
	updateActiveConnections(agentID, 1)
	defer updateActiveConnections(agentID, -1)

	// Create Reverse Proxy
	director := func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = "agent-internal" // Virtual host, agent ignores it or uses it
		// IMPORTANT: Path handling. Since we are subdomain, path "/" is forwarded as "/"
	}

	proxy := &httputil.ReverseProxy{
		Director:  director,
		Transport: &YamuxRoundTripper{Session: sess, AgentID: agentID},
	}

	proxy.ServeHTTP(w, r)
}

// YamuxRoundTripper implements http.RoundTripper using Yamux Stream
type YamuxRoundTripper struct {
	Session *yamux.Session
	AgentID string
}

func (t *YamuxRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Open a stream
	stream, err := t.Session.Open()
	if err != nil {
		return nil, err
	}
	// We must close the stream if request fails, but if it succeeds,
	// Response.Body.Close() should probably trigger stream close?
	// Actually http.Transport manages connections.
	// Since we are dialing a NEW stream for EVERY request (not keep-alive efficiently),
	// we can use a custom DialContext in http.Transport.

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return &MonitoredConn{Conn: stream, AgentID: t.AgentID}, nil
		},
		DisableKeepAlives: true, // Simpler for now
	}

	// Wait, if we create a new transport every time, it's fine but overhead.
	// And we used `stream` in DialContext closure.
	// When RoundTrip returns, the request is done.
	// But if response body is not closed, stream leaks?
	// http.Transport should handle closing the connection if DisableKeepAlives is true.

	return transport.RoundTrip(req)
}

// --- Existing Handlers (Agent, Connect, Mgmt) ---
// (Keep handleAgent, handleUserConnect, handleMgmtAgentStats, wsConn, etc.)
// Re-pasting them to ensure file completeness

func handleAgent(w http.ResponseWriter, r *http.Request) {

	token, err := validateToken(r)
	if err != nil {
		log.Printf("Agent connection rejected: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("id")
	if agentID == "" {
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Agent authenticated: ID=%s, Sub=%s", agentID, token.Subject)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[%s] WebSocket upgrade failed: %v", agentID, err)
		return
	}

	log.Printf("[%s] Agent connected", agentID)

	statsMu.Lock()
	agentStats[agentID] = &AgentStats{
		Online:       true,
		LastActivity: time.Now(),
	}
	statsMu.Unlock()

	defer func() {
		statsMu.Lock()
		if stats, ok := agentStats[agentID]; ok {
			stats.Online = false
		}
		statsMu.Unlock()
		log.Printf("[%s] Agent disconnected", agentID)
	}()

	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 30 * time.Second       // Increased from 10s for stability
	config.ConnectionWriteTimeout = 120 * time.Second // Increased from 5s for large file transfers (supports 2GB @ 10MB/s)

	conn := &wsConn{Conn: ws}
	session, err := yamux.Server(conn, config)
	if err != nil {
		log.Printf("[%s] Yamux server init failed: %v", agentID, err)
		_ = ws.Close()
		return
	}

	sessionsMu.Lock()
	if old, exists := agentSessions[agentID]; exists {
		log.Printf("[%s] Closing old session", agentID)
		_ = old.Close()
	}
	agentSessions[agentID] = session
	sessionsMu.Unlock()

	log.Printf("[%s] Session ready", agentID)
	select {}
}

func handleUserConnect(w http.ResponseWriter, r *http.Request) {

	token, err := validateToken(r)
	if err != nil {
		log.Printf("User connection rejected: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	subject := token.Subject
	log.Printf("User authenticated: Sub=%s, Target=%s", subject, r.URL.Path)

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Sub               string `json:"sub"`
		RealmAccess       struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := token.Claims(&claims); err != nil {
		log.Printf("Failed to extract claims: %v", err)
		http.Error(w, "Invalid token claims", http.StatusForbidden)
		return
	}

	agentID := strings.TrimPrefix(r.URL.Path, "/connect/")
	if agentID == "" || agentID == r.URL.Path {
		http.Error(w, "Missing agent ID in path", http.StatusBadRequest)
		return
	}

	// Check if user is admin
	isAdmin := false
	for _, role := range claims.RealmAccess.Roles {
		if role == "admin" {
			isAdmin = true
			break
		}
	}

	// Admin can access any service
	if !isAdmin {
		// Non-admin users must own the service
		authorized := false

		// Check new naming convention: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash8}
		if strings.HasPrefix(agentID, "kuberde-agent-") {
			// Extract parts: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash8}
			agentIDTrimmed := strings.TrimPrefix(agentID, "kuberde-agent-")
			parts := strings.SplitN(agentIDTrimmed, "-", 4)
			if len(parts) >= 1 {
				agentUserName := parts[0]
				if agentUserName == claims.PreferredUsername {
					authorized = true
				}
			}
		}

		if !authorized {
			log.Printf("SSH access denied: user %s (ID: %s) attempted to access agent %s",
				claims.PreferredUsername, claims.Sub, agentID)
			http.Error(w, "Forbidden: You do not have permission to access this service", http.StatusForbidden)
			return
		}
	} else {
		// Admin is accessing a service via SSH - determine the owner and log it
		targetOwner := "unknown"

		// Extract owner from agent ID
		if strings.HasPrefix(agentID, "kuberde-agent-") {
			agentIDTrimmed := strings.TrimPrefix(agentID, "kuberde-agent-")
			parts := strings.SplitN(agentIDTrimmed, "-", 4)
			if len(parts) >= 1 {
				targetOwner = parts[0] // userName is the first part
			}
		} else if strings.HasPrefix(agentID, "kuberde-") {
			agentIDTrimmed := strings.TrimPrefix(agentID, "kuberde-")
			parts := strings.SplitN(agentIDTrimmed, "-", 3)
			if len(parts) >= 3 && db != nil {
				workspaceID := parts[1]
				workspace, err := dbpkg.WorkspaceRepo().FindByID(workspaceID)
				if err == nil && workspace != nil && workspace.Owner != nil {
					targetOwner = workspace.Owner.Username
				}
			}
		} else if strings.HasPrefix(agentID, "user-") {
			idParts := strings.SplitN(agentID, "-", 3)
			if len(idParts) >= 2 {
				targetOwner = idParts[1]
			}
		}

		// Only log if admin is accessing another user's service
		if targetOwner != claims.PreferredUsername && targetOwner != "unknown" {
			logAdminAccess(claims.Sub, claims.PreferredUsername, "ssh_proxy_access", "service", agentID, targetOwner)
		}
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("User [%s] upgrade failed: %v", agentID, err)
		return
	}
	defer func() { _ = ws.Close() }()

	sessionsMu.RLock()
	sess, ok := agentSessions[agentID]
	sessionsMu.RUnlock()

	if !ok || sess.IsClosed() {
		// Check if agent is scaled down and try to scale up
		scaledDown, err := isAgentScaledDown(agentID)
		if err != nil {
			log.Printf("Failed to check agent scale status: %v", err)
			msg := "Error: Agent unavailable (status check failed)\n"
			if err := ws.WriteMessage(websocket.BinaryMessage, []byte(msg)); err != nil {
				log.Printf("Failed to write error message to ws: %v", err)
			}
			return
		}

		if scaledDown {
			log.Printf("Agent %s is scaled down, attempting scale-up...", agentID)

			// Try to scale up with 60 second timeout
			err := scaleUpDeployment(agentID, 60)
			if err != nil {
				log.Printf("Scale-up failed for agent %s: %v", agentID, err)
				msg := fmt.Sprintf("Error: Failed to start agent: %v\n", err)
				if err := ws.WriteMessage(websocket.BinaryMessage, []byte(msg)); err != nil {
					log.Printf("Failed to write error message to ws: %v", err)
				}
				return
			}
		}

		// Re-check if session is now available (wait even if it was already up)
		sess, ok = waitForSessionReady(agentID, 15)

		if !ok || sess == nil || sess.IsClosed() {
			msg := "Error: Agent [" + agentID + "] not found or offline\n"
			log.Println(msg)
			if err := ws.WriteMessage(websocket.BinaryMessage, []byte(msg)); err != nil {
				log.Printf("Failed to write error message to ws: %v", err)
			}
			return
		}
	}

	stream, err := sess.Open()
	if err != nil {
		log.Printf("Failed to open stream to agent [%s]: %v", agentID, err)
		return
	}
	defer func() { _ = stream.Close() }()

	log.Printf("Tunneling User -> Agent [%s]", agentID)
	updateAgentActivity(agentID)
	updateActiveConnections(agentID, 1)
	defer updateActiveConnections(agentID, -1)

	// DB: Create Connection Record
	connRecord := TunnelConnection{
		AgentID:   agentID,
		UserID:    subject,
		ClientIP:  r.RemoteAddr,
		StartedAt: time.Now(),
		Active:    true,
		Protocol:  "websocket",
	}
	if db != nil {
		db.Create(&connRecord)
	}

	var bytesIn int64
	var bytesOut int64

	// Defer DB Update
	defer func() {
		if db != nil {
			// Update final stats
			// Re-query/Save to avoid race conditions or partial updates
			db.Model(&connRecord).Updates(map[string]interface{}{
				"active":    false,
				"ended_at":  time.Now(),
				"bytes_in":  atomic.LoadInt64(&bytesIn),
				"bytes_out": atomic.LoadInt64(&bytesOut),
			})
		}
	}()

	userConn := &wsConn{Conn: ws}

	// Upstream: User -> Agent
	go func() {
		// Count bytes WRITTEN to the agent (Inbound to agent)
		wrappedStream := &TrafficCounter{ReadWriter: stream, BytesWrite: &bytesIn}
		updater := &ActivityUpdater{AgentID: agentID, Delegate: wrappedStream}
		n, err := io.Copy(updater, userConn)
		if err != nil {
			log.Printf("Error copying from user to agent [%s]: %v", agentID, err)
		}
		log.Printf("[%s] Upstream (User->Agent) closed. Bytes: %d, Err: %v", agentID, n, err)
	}()

	// Downstream: Agent -> User
	// Count bytes WRITTEN to the user (Outbound from agent)
	wrappedUserConn := &TrafficCounter{ReadWriter: userConn, BytesWrite: &bytesOut}
	updater := &ActivityUpdater{AgentID: agentID, Delegate: wrappedUserConn}
	n, err := io.Copy(updater, stream)
	if err != nil {
		log.Printf("Error copying from agent [%s] to user: %v", agentID, err)
	}
	log.Printf("[%s] Downstream (Agent->User) closed. Bytes: %d, Err: %v", agentID, n, err)
}

func handleGetConnections(w http.ResponseWriter, r *http.Request) {
	// Auth Check
	token, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusForbidden)
		return
	}

	agentID := r.URL.Query().Get("agentId")
	limit := 50

	var conns []TunnelConnection
	query := db.Order("started_at desc").Limit(limit)

	if agentID != "" {
		// Check ownership if it's a user agent
		if strings.HasPrefix(agentID, "user-") {
			parts := strings.SplitN(agentID, "-", 3)
			if len(parts) >= 2 && parts[1] != claims.PreferredUsername {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		query = query.Where("agent_id = ?", agentID)
	} else {
		// If no agent ID, filter by user ownership (simplified: only show agents owned by user)
		// This requires a LIKE query or joining.
		// For now, let's just show records where UserID matches the caller OR AgentID starts with "user-<username>-"
		// But UserID in TunnelConnection is the *connector*, not the *owner*.
		// So we must filter by AgentID prefix.
		prefix := fmt.Sprintf("user-%s-%%", claims.PreferredUsername)
		query = query.Where("agent_id LIKE ?", prefix)
	}

	if err := query.Find(&conns).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(conns); err != nil {
		log.Printf("Failed to encode connections: %v", err)
	}
}

func handleGetTraffic(w http.ResponseWriter, r *http.Request) {
	// Returns aggregated traffic per agent for the last 24h
	// Auth Check
	token, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusForbidden)
		return
	}

	// Simplified: Return raw list of recent connections for frontend to aggregate,
	// or implement aggregation here.
	// Let's implement basic aggregation by AgentID.

	type TrafficStat struct {
		AgentID  string `json:"agentId"`
		TotalIn  int64  `json:"totalIn"`
		TotalOut int64  `json:"totalOut"`
	}

	var stats []TrafficStat

	// Filter by owner
	prefix := fmt.Sprintf("user-%s-%%", claims.PreferredUsername)

	err = db.Model(&TunnelConnection{}).
		Select("agent_id, sum(bytes_in) as total_in, sum(bytes_out) as total_out").
		Where("agent_id LIKE ? AND started_at > ?", prefix, time.Now().Add(-24*time.Hour)).
		Group("agent_id").
		Scan(&stats).Error

	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Failed to encode stats: %v", err)
	}
}

func handleGetEvents(w http.ResponseWriter, r *http.Request) {
	// Auth Check
	_, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if k8sClientset == nil {
		http.Error(w, "K8s client not initialized", http.StatusInternalServerError)
		return
	}

	events, err := k8sClientset.CoreV1().Events(kuberdeNamespace).List(context.TODO(), metav1.ListOptions{
		Limit: 100, // Limit to recent events
	})
	if err != nil {
		log.Printf("Failed to list events: %v", err)
		http.Error(w, "Failed to list events", http.StatusInternalServerError)
		return
	}

	type SystemEvent struct {
		ID        string `json:"id"`
		Timestamp string `json:"timestamp"`
		AgentID   string `json:"agentId"`
		Type      string `json:"type"`
		Message   string `json:"message"`
	}

	var respEvents []SystemEvent
	for _, e := range events.Items {
		agentID := ""
		switch e.InvolvedObject.Kind {
		case "Deployment", "RDEAgent":
			agentID = e.InvolvedObject.Name
		case "Pod":
			// Extract agent ID from pod name if possible, or use label logic?
			// Pod name is usually agentID-hash-hash.
			// Simplified: just send object name
			agentID = e.InvolvedObject.Name
		}

		respEvents = append(respEvents, SystemEvent{
			ID:        string(e.UID),
			Timestamp: e.LastTimestamp.Format(time.RFC3339),
			AgentID:   agentID,
			Type:      e.Type, // Normal, Warning
			Message:   fmt.Sprintf("[%s] %s: %s", e.Reason, e.InvolvedObject.Kind, e.Message),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(respEvents); err != nil {
		log.Printf("Failed to encode events: %v", err)
	}
}

func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	// Auth Check
	_, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	// /api/agents/{id}/logs
	if len(pathParts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	agentID := pathParts[3]

	if k8sClientset == nil {
		http.Error(w, "K8s client not initialized", http.StatusInternalServerError)
		return
	}

	// Find Pod through Deployment (deployment name = agentID)
	// Get deployment first to find its pod selector
	deployment, err := k8sClientset.AppsV1().Deployments(kuberdeNamespace).Get(context.TODO(), agentID, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	// Use deployment's selector to find pods
	labelSelector := ""
	for k, v := range deployment.Spec.Selector.MatchLabels {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += fmt.Sprintf("%s=%s", k, v)
	}

	pods, err := k8sClientset.CoreV1().Pods(kuberdeNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		http.Error(w, "Failed to list pods", http.StatusInternalServerError)
		return
	}
	if len(pods.Items) == 0 {
		http.Error(w, "Pod not found (Agent might be offline)", http.StatusNotFound)
		return
	}

	podName := pods.Items[0].Name
	tail := int64(100)

	// Try to get logs from 'workload' container, fallback to 'kuberde-agent' if needed
	containerName := "workload"

	req := k8sClientset.CoreV1().Pods(kuberdeNamespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tail,
		Container: containerName,
	})

	stream, err := req.Stream(context.TODO())
	if err != nil {
		// Fallback to kuberde-agent?
		http.Error(w, "Failed to stream logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = stream.Close() }()

	w.Header().Set("Content-Type", "text/plain")
	if _, err := io.Copy(w, stream); err != nil {
		log.Printf("Failed to copy logs to response: %v", err)
	}
}

func handleMgmtAgentStats(w http.ResponseWriter, r *http.Request) {
	agentID := strings.TrimPrefix(r.URL.Path, "/mgmt/agents/")
	if agentID == "" {
		http.Error(w, "Missing agent ID", http.StatusBadRequest)
		return
	}

	statsMu.RLock()
	stats, ok := agentStats[agentID]
	statsMu.RUnlock()

	if !ok {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Failed to encode agent stats: %v", err)
	}
}

// Helpers
func generateRandomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "" // Return empty string on error, though unlikely
	}
	return base64.URLEncoding.EncodeToString(b)
}

func updateAgentActivity(agentID string) {
	statsMu.Lock()
	defer statsMu.Unlock()

	// Check if agent has active session
	sessionsMu.RLock()
	hasSession := agentSessions[agentID] != nil
	sessionsMu.RUnlock()

	if stats, ok := agentStats[agentID]; ok {
		stats.UpdateActivity()
		stats.HasActiveSession = hasSession
	} else {
		agentStats[agentID] = &AgentStats{
			Online:           true,
			LastActivity:     time.Now(),
			HasActiveSession: hasSession,
		}
	}
}

type MonitoredConn struct {
	net.Conn
	AgentID string
}

func (c *MonitoredConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if n > 0 {
		updateAgentActivity(c.AgentID)
	}
	return
}

func (c *MonitoredConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if n > 0 {
		updateAgentActivity(c.AgentID)
	}
	return
}

// ActivityUpdater & wsConn definitions (Must be present)
type ActivityUpdater struct {
	AgentID  string
	Delegate io.ReadWriter
}

func (a *ActivityUpdater) Read(p []byte) (int, error) {
	n, err := a.Delegate.Read(p)
	if n > 0 {
		updateAgentActivity(a.AgentID)
	}
	return n, err
}
func (a *ActivityUpdater) Write(p []byte) (int, error) {
	n, err := a.Delegate.Write(p)
	if n > 0 {
		updateAgentActivity(a.AgentID)
	}
	return n, err
}

type wsConn struct {
	*websocket.Conn
	reader io.Reader
}

func (c *wsConn) Read(p []byte) (int, error) {
	for {
		if c.reader == nil {
			messageType, r, err := c.NextReader()
			if err != nil {
				return 0, err
			}
			if messageType != websocket.BinaryMessage {
				continue
			}
			c.reader = r
		}
		n, err := c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			continue
		}
		return n, err
	}
}
func (c *wsConn) Write(p []byte) (int, error) {
	err := c.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}
func (c *wsConn) SetDeadline(t time.Time) error { return c.Conn.UnderlyingConn().SetDeadline(t) }
func (c *wsConn) SetReadDeadline(t time.Time) error {
	return c.Conn.UnderlyingConn().SetReadDeadline(t)
}
func (c *wsConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.UnderlyingConn().SetWriteDeadline(t)
}

// K8s Scale-up Helper Functions

func getAgentNamespaceAndName(agentID string) (namespace, crName string) {
	// Parse agentID format: "user-{owner}-{name}" or just "{name}"
	// Return namespace and CR name
	namespace = kuberdeNamespace

	if strings.HasPrefix(agentID, "user-") {
		parts := strings.SplitN(agentID, "-", 3)
		if len(parts) == 3 {
			crName = parts[2] // CR name is the last part
		} else {
			crName = agentID
		}
	} else {
		crName = agentID
	}
	return namespace, crName
}

func isAgentScaledDown(agentID string) (bool, error) {
	if k8sClientset == nil {
		return false, fmt.Errorf("K8s client not initialized")
	}

	namespace, _ := getAgentNamespaceAndName(agentID)

	// Get Deployment associated with this agent
	// Deployment name = agentID (set by operator)
	deployment, err := k8sClientset.AppsV1().Deployments(namespace).Get(context.TODO(), agentID, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Check if replicas are 0
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
		return true, nil
	}
	return false, nil
}

func scaleUpDeployment(agentID string, timeoutSeconds int) error {
	if k8sClientset == nil {
		return fmt.Errorf("K8s client not initialized")
	}

	namespace, _ := getAgentNamespaceAndName(agentID)

	// Acquire lock to prevent concurrent scale-up for same agent
	scaleUpMutex.Lock()
	if scaleUpInProgress[agentID] {
		scaleUpMutex.Unlock()
		// Already scaling up, wait for completion
		return waitForDeploymentReady(agentID, timeoutSeconds)
	}
	scaleUpInProgress[agentID] = true
	scaleUpMutex.Unlock()

	defer func() {
		scaleUpMutex.Lock()
		delete(scaleUpInProgress, agentID)
		scaleUpMutex.Unlock()
	}()

	// Get current Deployment
	deploymentsClient := k8sClientset.AppsV1().Deployments(namespace)
	deployment, err := deploymentsClient.Get(context.TODO(), agentID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Only update if currently scaled down
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
		log.Printf("Agent %s already scaled up (replicas=%d)", agentID, *deployment.Spec.Replicas)
		return nil
	}

	// Update replicas to 1
	one := int32(1)
	deployment.Spec.Replicas = &one

	_, err = deploymentsClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment replicas: %w", err)
	}

	log.Printf("Scaled up agent %s, waiting for pod readiness...", agentID)

	// Wait for pod to be ready
	return waitForDeploymentReady(agentID, timeoutSeconds)
}

func waitForDeploymentReady(agentID string, timeoutSeconds int) error {
	namespace, _ := getAgentNamespaceAndName(agentID)
	deploymentsClient := k8sClientset.AppsV1().Deployments(namespace)

	// Poll until pod is ready or timeout
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for deployment %s to be ready (timeout=%ds)", agentID, timeoutSeconds)
		case <-ticker.C:
			deployment, err := deploymentsClient.Get(context.TODO(), agentID, metav1.GetOptions{})
			if err != nil {
				log.Printf("Error getting deployment %s: %v", agentID, err)
				continue
			}

			// Check if ready replicas == desired replicas
			desiredReplicas := int32(1)
			if deployment.Spec.Replicas != nil {
				desiredReplicas = *deployment.Spec.Replicas
			}

			readyReplicas := deployment.Status.ReadyReplicas

			if readyReplicas >= desiredReplicas && desiredReplicas > 0 {
				log.Printf("Agent %s is ready (readyReplicas=%d)", agentID, readyReplicas)
				return nil
			}

			log.Printf("Waiting for agent %s readiness: desired=%d, ready=%d", agentID, desiredReplicas, readyReplicas)
		}
	}
}

func waitForSessionReady(agentID string, timeoutSeconds int) (*yamux.Session, bool) {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		sessionsMu.RLock()
		sess, ok := agentSessions[agentID]
		sessionsMu.RUnlock()

		if ok && sess != nil && !sess.IsClosed() {
			return sess, true
		}

		if time.Now().After(deadline) {
			return nil, false
		}

		<-ticker.C
	}
}

func handleScaleUpAgent(w http.ResponseWriter, r *http.Request) {
	// Only accept PUT requests
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract agent ID from URL path (e.g., /api/agents/{id}/scale-up)
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	agentID := pathParts[3]

	// Validate authorization
	token, err := validateToken(r)
	if err != nil {
		log.Printf("Scale-up request rejected: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check ownership if user-based agent
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusForbidden)
		return
	}

	if strings.HasPrefix(agentID, "user-") {
		parts := strings.SplitN(agentID, "-", 3)
		if len(parts) >= 2 {
			owner := parts[1]
			if owner != claims.PreferredUsername {
				log.Printf("Access denied: User %s tried to scale-up agent %s owned by %s", claims.PreferredUsername, agentID, owner)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
	}

	log.Printf("Scale-up request for agent %s by user %s", agentID, claims.PreferredUsername)

	// Attempt scale-up with 60 second timeout
	err = scaleUpDeployment(agentID, 60)
	if err != nil {
		log.Printf("Scale-up failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"error": "Scale-up timeout or failed: %v"}`, err)
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status": "scaled up", "agentID": "%s"}`, agentID)
}

func handleListAgents(w http.ResponseWriter, r *http.Request) {
	// Validate Auth
	token, err := validateToken(r)
	if err != nil {
		log.Printf("Token validation failed in handleListAgents: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusForbidden)
		return
	}

	if dynamicClient == nil {
		http.Error(w, "K8s client not initialized", http.StatusInternalServerError)
		return
	}

	// List CRDs
	list, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Failed to list agents: %v", err)
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}

	type AgentResponse struct {
		ID                string `json:"id"`
		Name              string `json:"name"`
		Owner             string `json:"owner"`
		Status            string `json:"status"` // From K8s Status
		Online            bool   `json:"online"` // From K8s Status or Real-time
		LocalTarget       string `json:"localTarget"`
		RemoteProxy       string `json:"remoteProxy"`
		TTL               string `json:"ttl"`
		ActiveConnections int    `json:"activeConnections"`
		LastActivity      string `json:"lastActivity"`
		CreatedAt         string `json:"createdAt"`
		Resources         struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"resources"`
		Image string `json:"image"`
	}

	var agents = make([]AgentResponse, 0)

	for _, item := range list.Items {
		name := item.GetName()
		// Filter by owner if user- prefix
		if strings.HasPrefix(name, "user-") {
			parts := strings.SplitN(name, "-", 3)
			if len(parts) >= 2 {
				// owner := parts[1]
				// TODO: Implement owner filtering?
				_ = parts[1] // Suppress empty branch / unused variable
			}
		}

		spec, _ := item.Object["spec"].(map[string]interface{})
		status, _ := item.Object["status"].(map[string]interface{})

		resources, _ := spec["resources"].(map[string]interface{})
		requests, _ := resources["requests"].(map[string]interface{})
		workload, _ := spec["workloadContainer"].(map[string]interface{})

		// Merge Real-time stats
		statsMu.RLock()
		rtStats := agentStats[name]
		statsMu.RUnlock()

		online := false
		if status != nil {
			if val, ok := status["online"].(bool); ok {
				online = val
			}
		}
		// Prefer real-time stats if available
		if rtStats != nil && rtStats.Online {
			online = true
		}

		phase := "Unknown"
		if status != nil {
			if p, ok := status["phase"].(string); ok {
				phase = p
			}
		}

		// Handle empty resources
		cpu := "100m"
		mem := "64Mi"
		if requests != nil {
			if v, ok := requests["cpu"].(string); ok {
				cpu = v
			}
			if v, ok := requests["memory"].(string); ok {
				mem = v
			}
		}

		// Handle image
		image := ""
		if workload != nil {
			if v, ok := workload["image"].(string); ok {
				image = v
			}
		}

		// Handle owner
		owner := ""
		if v, ok := spec["owner"].(string); ok {
			owner = v
		}

		// Handle localTarget
		localTarget := ""
		if v, ok := spec["localTarget"].(string); ok {
			localTarget = v
		}

		// Handle TTL
		ttl := "0"
		if v, ok := spec["ttl"].(string); ok {
			ttl = v
		}

		agents = append(agents, AgentResponse{
			ID:                name,
			Name:              name,
			Owner:             owner,
			Status:            phase,
			Online:            online,
			LocalTarget:       localTarget,
			RemoteProxy:       fmt.Sprintf("%s.%s", name, agentDomain),
			TTL:               ttl,
			ActiveConnections: 0, // Need to track this in agentStats if needed
			LastActivity:      item.GetCreationTimestamp().Format(time.RFC3339),
			CreatedAt:         item.GetCreationTimestamp().Format(time.RFC3339),
			Resources: struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			}{CPU: cpu, Memory: mem},
			Image: image,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(agents); err != nil {
		log.Printf("Failed to encode agents response: %v", err)
	}
}

func handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	// Validate Auth
	token, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusForbidden)
		return
	}

	var req struct {
		Name        string `json:"name"`
		LocalTarget string `json:"localTarget"`
		TTL         string `json:"ttl"`
		Memory      string `json:"memory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Construct Peer ID
	agentID := fmt.Sprintf("user-%s-%s", claims.PreferredUsername, req.Name)

	// Determine agent image
	agentImage := os.Getenv("KUBERDE_AGENT_IMAGE")
	if agentImage == "" {
		agentImage = "soloking/kuberde-agent:latest"
	}

	// Create CRD Unstructured
	agent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kuberde.io/v1beta1",
			"kind":       "RDEAgent",
			"metadata": map[string]interface{}{
				"name":      agentID,
				"namespace": kuberdeNamespace,
			},
			"spec": map[string]interface{}{
				"serverUrl":   agentServerURL,
				"authSecret":  agentAuthSecret,
				"owner":       claims.PreferredUsername,
				"localTarget": req.LocalTarget,
				"ttl":         req.TTL,
				"workloadContainer": map[string]interface{}{
					"image":           agentImage,
					"imagePullPolicy": "Always",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "100m",
							"memory": req.Memory,
						},
						"limits": map[string]interface{}{
							"cpu":    "200m",
							"memory": "256Mi",
						},
					},
				},
			},
		},
	}

	_, err = dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Create(context.TODO(), agent, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create agent %s: %v", agentID, err)
		http.Error(w, "Failed to create agent", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(agent); err != nil {
		log.Printf("Failed to encode agent response: %v", err)
	}
}

func handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	// Validate Auth
	_, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	agentID := pathParts[3]

	err = dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Delete(context.TODO(), agentID, metav1.DeleteOptions{})
	if err != nil {
		http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleStopAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate Auth
	_, err := validateToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	// /api/agents/{id}/stop
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	agentID := pathParts[3]

	// Manual Scaledown -> Replicas = 0
	// We can reuse getAgentNamespaceAndName if exposed or copy logic
	namespace, _ := getAgentNamespaceAndName(agentID)

	deploymentsClient := k8sClientset.AppsV1().Deployments(namespace)
	deployment, err := deploymentsClient.Get(context.TODO(), agentID, metav1.GetOptions{})
	if err != nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	zero := int32(0)
	deployment.Spec.Replicas = &zero

	_, err = deploymentsClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, "Failed to stop agent", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status": "stopped", "agentID": "%s"}`, agentID)
}

func handleGetGlobalStats(w http.ResponseWriter, r *http.Request) {
	if dynamicClient == nil {
		http.Error(w, "K8s client not initialized", http.StatusInternalServerError)
		return
	}

	list, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}

	total := len(list.Items)
	online := 0

	statsMu.RLock()
	activeConns := len(agentSessions) // Approximation
	statsMu.RUnlock()

	for _, item := range list.Items {
		status, _ := item.Object["status"].(map[string]interface{})
		if status != nil {
			if val, ok := status["online"].(bool); ok && val {
				online++
			}
		}
	}

	resp := map[string]interface{}{
		"totalAgents":       total,
		"onlineAgents":      online,
		"offlineAgents":     total - online,
		"activeConnections": activeConns,
		"traffic24h":        "0 GB", // Placeholder
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode global stats response: %v", err)
	}
}

// ===== User Management & RBAC =====

// hasRole checks if the user has a specific role from the request
func hasRole(r *http.Request, role string) bool {
	// Verify token
	token, err := validateToken(r)
	if err != nil {
		// Try cookie
		token, err = validateCookie(r)
		if err != nil {
			return false
		}
	}

	// Extract roles from token claims
	var claims struct {
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}

	if err := token.Claims(&claims); err != nil {
		return false
	}

	// Check if user has the role
	for _, r := range claims.RealmAccess.Roles {
		if r == role {
			return true
		}
	}

	return false
}

// requireRole is middleware that checks user has the required role
func requireRole(requiredRole string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Verify token
			token, err := validateToken(r)
			if err != nil {
				// Try cookie
				token, err = validateCookie(r)
				if err != nil {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}

			// Extract roles from token claims
			var claims struct {
				RealmAccess struct {
					Roles []string `json:"roles"`
				} `json:"realm_access"`
			}

			if err := token.Claims(&claims); err != nil {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// Check if user has required role
			hasRole := false
			for _, role := range claims.RealmAccess.Roles {
				if role == requiredRole {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			// Store token in context for handler use
			ctx := context.WithValue(r.Context(), idTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}

// handleLogout clears session cookies and returns Keycloak logout URL
func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the ID token from cookie for Keycloak logout
	var idTokenHint string
	if cookie, err := r.Cookie("kuberde_session"); err == nil {
		idTokenHint = cookie.Value
	}

	// Clear session cookie - must match the same domain as login
	http.SetCookie(w, &http.Cookie{
		Name:     "kuberde_session",
		Value:    "",
		Path:     "/",
		Domain:   getRootDomain(agentDomain), // Must match the domain set during login
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureDeployment(),
		SameSite: http.SameSiteNoneMode,
	})

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/auth/callback", // Match the path set during login
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureDeployment(),
	})

	// Build Keycloak logout URL to invalidate SSO session
	// Redirect to static login page (not /auth/login which triggers OAuth immediately)
	keycloakLogoutURL := fmt.Sprintf("%s/protocol/openid-connect/logout?post_logout_redirect_uri=%s&client_id=%s",
		keycloakRealmURL,
		url.QueryEscape(frpURL+"/#/login"),
		"kuberde-cli",
	)
	if idTokenHint != "" {
		keycloakLogoutURL += "&id_token_hint=" + url.QueryEscape(idTokenHint)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message":     "Logged out successfully",
		"redirect_to": keycloakLogoutURL,
	}); err != nil {
		log.Printf("Failed to encode logout response: %v", err)
	}
}

// handleRefreshToken refreshes the user's token
func handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate current token/cookie
	token, err := validateToken(r)
	if err != nil {
		token, err = validateCookie(r)
		if err != nil {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}
	}

	// Extract user info
	var claims struct {
		Sub               string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid claims", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Token refreshed",
		"user": map[string]string{
			"id":       claims.Sub,
			"username": claims.PreferredUsername,
			"email":    claims.Email,
		},
	}); err != nil {
		log.Printf("Failed to encode token refresh response: %v", err)
	}
}

// handleGetCurrentUser returns current authenticated user info
func handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate token
	token, err := validateToken(r)
	if err != nil {
		token, err = validateCookie(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Extract user info with roles
	var claims struct {
		Sub               string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		RealmAccess       struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}

	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusInternalServerError)
		return
	}

	// Ensure roles is always an array (never nil)
	roles := claims.RealmAccess.Roles
	if roles == nil {
		roles = []string{}
	}

	// Fetch SSH keys for the user from database
	var sshKeys []models.SSHKey
	if db != nil {
		userRepo := dbpkg.UserRepo()
		user, err := userRepo.FindByID(claims.Sub)
		if err == nil && user != nil && user.SSHKeys != nil {
			// Parse SSH keys from JSONB
			if err := json.Unmarshal(*user.SSHKeys, &sshKeys); err != nil {
				log.Printf("Failed to parse SSH keys for user %s: %v", claims.Sub, err)
				sshKeys = []models.SSHKey{}
			}
		}
	}

	// Ensure ssh_keys is always an array (never nil)
	if sshKeys == nil {
		sshKeys = []models.SSHKey{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       claims.Sub,
		"username": claims.PreferredUsername,
		"email":    claims.Email,
		"name":     claims.Name,
		"roles":    roles,
		"enabled":  true,
		"ssh_keys": sshKeys,
	}); err != nil {
		log.Printf("Failed to encode current user response: %v", err)
	}
}

// handleGetSystemConfig returns public system configuration (no auth required)
func handleGetSystemConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get Keycloak URL from environment or use default
	keycloakURL := os.Getenv("KEYCLOAK_PUBLIC_URL")
	if keycloakURL == "" {
		keycloakURL = os.Getenv("KEYCLOAK_URL")
		if keycloakURL == "" {
			keycloakURL = "http://keycloak:8080"
		}
	}

	// Return public configuration that frontend needs
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"public_url":   frpURL,
		"agent_domain": agentDomain,
		"keycloak_url": keycloakURL,
		"realm_name":   "kuberde",
	}); err != nil {
		log.Printf("Failed to encode system config response: %v", err)
	}
}

// handleUsers routes user list and creation requests
func handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		requireRole("admin")(handleListUsers)(w, r)
	case http.MethodPost:
		requireRole("admin")(handleCreateUser)(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListUsers lists all users (admin only)
func handleListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Parse pagination params
	first := 0
	max := 100

	if f := r.URL.Query().Get("first"); f != "" {
		if val, err := strconv.Atoi(f); err == nil {
			first = val
		}
	}
	if m := r.URL.Query().Get("max"); m != "" {
		if val, err := strconv.Atoi(m); err == nil {
			max = val
		}
	}

	users, err := keycloakClient.GetUsers(
		ctx,
		getAdminToken(),
		"kuberde",
		gocloak.GetUsersParams{
			First: gocloak.IntP(first),
			Max:   gocloak.IntP(max),
		},
	)
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}

	// Transform to response format
	var result []map[string]interface{}
	for _, user := range users {
		if user.ID == nil || user.Username == nil {
			continue
		}

		userMap := map[string]interface{}{
			"id":       *user.ID,
			"username": *user.Username,
			"enabled":  false,
			"created":  int64(0),
		}

		if user.Email != nil {
			userMap["email"] = *user.Email
		}
		if user.Enabled != nil {
			userMap["enabled"] = *user.Enabled
		}
		if user.CreatedTimestamp != nil {
			userMap["created"] = *user.CreatedTimestamp
		}

		// Get user roles
		roles, err := keycloakClient.GetRealmRolesByUserID(
			ctx,
			getAdminToken(),
			"kuberde",
			*user.ID,
		)
		if err == nil {
			var roleNames []string
			for _, role := range roles {
				if role.Name != nil {
					roleNames = append(roleNames, *role.Name)
				}
			}
			userMap["roles"] = roleNames
		} else {
			userMap["roles"] = []string{}
		}

		result = append(result, userMap)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleCreateUser creates a new user
func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string   `json:"username"`
		Email    string   `json:"email"`
		Password string   `json:"password"`
		Roles    []string `json:"roles"`
		Enabled  bool     `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "username, email, and password are required", http.StatusBadRequest)
		return
	}

	// Default to developer role
	if len(req.Roles) == 0 {
		req.Roles = []string{"developer"}
	}

	ctx := context.Background()

	// Create user
	user := gocloak.User{
		Username:      gocloak.StringP(req.Username),
		Email:         gocloak.StringP(req.Email),
		Enabled:       gocloak.BoolP(req.Enabled || true),
		EmailVerified: gocloak.BoolP(true),
		Credentials: &[]gocloak.CredentialRepresentation{
			{
				Type:      gocloak.StringP("password"),
				Value:     gocloak.StringP(req.Password),
				Temporary: gocloak.BoolP(false),
			},
		},
	}

	userID, err := keycloakClient.CreateUser(
		ctx,
		getAdminToken(),
		"kuberde",
		user,
	)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Assign roles
	for _, roleName := range req.Roles {
		role, err := keycloakClient.GetRealmRole(
			ctx,
			getAdminToken(),
			"kuberde",
			roleName,
		)
		if err != nil {
			log.Printf("Failed to get role %s: %v", roleName, err)
			continue
		}

		err = keycloakClient.AddRealmRoleToUser(
			ctx,
			getAdminToken(),
			"kuberde",
			userID,
			[]gocloak.Role{*role},
		)
		if err != nil {
			log.Printf("Failed to assign role %s to user: %v", roleName, err)
		}
		if err != nil {
			log.Printf("Failed to assign role %s to user: %v", roleName, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id":      userID,
		"message": "User created successfully",
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleUserDetail routes individual user operations
func handleUserDetail(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path: /api/users/{id} or /api/users/{id}/quota or /api/users/{id}/ssh-keys
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	userID := parts[3]

	// Check if this is a quota request
	if len(parts) >= 5 && parts[4] == "quota" {
		switch r.Method {
		case http.MethodGet:
			handleGetUserQuota(w, r, userID)
		case http.MethodPut:
			requireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
				handleUpdateUserQuota(w, r, userID)
			})(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Check if this is an SSH keys request
	if len(parts) >= 5 && parts[4] == "ssh-keys" {
		// Allow users to manage their own SSH keys
		authenticatedUserID, err := extractUserIDFromRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is managing their own SSH keys or is an admin
		if authenticatedUserID != userID {
			requireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
				handleUserSSHKeys(w, r, userID, parts)
			})(w, r)
			return
		}

		handleUserSSHKeys(w, r, userID, parts)
		return
	}

	// Regular user detail requests
	switch r.Method {
	case http.MethodGet:
		// Allow users to view their own profile, or admin to view any profile
		authenticatedUserID, err := extractUserIDFromRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is viewing their own profile or is an admin
		if authenticatedUserID == userID {
			// User is viewing their own profile
			handleGetUser(w, r, userID)
		} else {
			// Require admin role for viewing other users' profiles
			requireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
				handleGetUser(w, r, userID)
			})(w, r)
		}
	case http.MethodPut:
		requireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
			handleUpdateUser(w, r, userID)
		})(w, r)
	case http.MethodDelete:
		requireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
			handleDeleteUser(w, r, userID)
		})(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetUser fetches a user by ID
func handleGetUser(w http.ResponseWriter, r *http.Request, userID string) {
	ctx := context.Background()

	user, err := keycloakClient.GetUserByID(
		ctx,
		getAdminToken(),
		"kuberde",
		userID,
	)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get user roles
	roles, _ := keycloakClient.GetRealmRolesByUserID(
		ctx,
		getAdminToken(),
		"kuberde",
		userID,
	)

	var roleNames []string
	for _, role := range roles {
		if role.Name != nil {
			roleNames = append(roleNames, *role.Name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       *user.ID,
		"username": *user.Username,
		"email":    user.Email,
		"enabled":  *user.Enabled,
		"created":  user.CreatedTimestamp,
		"roles":    roleNames,
	}); err != nil {
		log.Printf("Failed to encode user details: %v", err)
	}
}

// handleUpdateUser updates a user's attributes
func handleUpdateUser(w http.ResponseWriter, r *http.Request, userID string) {
	var req struct {
		Email   *string  `json:"email"`
		Enabled *bool    `json:"enabled"`
		Roles   []string `json:"roles"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Get existing user
	user, err := keycloakClient.GetUserByID(
		ctx,
		getAdminToken(),
		"kuberde",
		userID,
	)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Update fields
	if req.Email != nil {
		user.Email = gocloak.StringP(*req.Email)
	}
	if req.Enabled != nil {
		user.Enabled = gocloak.BoolP(*req.Enabled)
	}

	// Update user
	err = keycloakClient.UpdateUser(
		ctx,
		getAdminToken(),
		"kuberde",
		*user,
	)
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	// Update roles if provided
	if req.Roles != nil {
		// Remove existing roles
		existingRoles, _ := keycloakClient.GetRealmRolesByUserID(
			ctx,
			getAdminToken(),
			"kuberde",
			userID,
		)

		if len(existingRoles) > 0 {
			// Convert []*gocloak.Role to []gocloak.Role
			rolesToRemove := make([]gocloak.Role, len(existingRoles))
			for i, role := range existingRoles {
				if role != nil {
					rolesToRemove[i] = *role
				}
			}
			if err := keycloakClient.DeleteRealmRoleFromUser(
				ctx,
				getAdminToken(),
				"kuberde",
				userID,
				rolesToRemove,
			); err != nil {
				log.Printf("Failed to remove existing roles from user %s: %v", userID, err)
			}
		}

		// Add new roles
		for _, roleName := range req.Roles {
			role, err := keycloakClient.GetRealmRole(
				ctx,
				getAdminToken(),
				"kuberde",
				roleName,
			)
			if err != nil {
				continue
			}

			if err := keycloakClient.AddRealmRoleToUser(
				ctx,
				getAdminToken(),
				"kuberde",
				userID,
				[]gocloak.Role{*role},
			); err != nil {
				log.Printf("Failed to add role %s to user %s: %v", *role.Name, userID, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "User updated successfully",
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleDeleteUser deletes a user
func handleDeleteUser(w http.ResponseWriter, r *http.Request, userID string) {
	ctx := context.Background()

	// Get current user to prevent self-deletion
	token, err := validateToken(r)
	if err != nil {
		token, err = validateCookie(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := token.Claims(&claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusInternalServerError)
		return
	}

	// Prevent self-deletion
	if claims.Sub == userID {
		http.Error(w, "You cannot delete your own account", http.StatusForbidden)
		return
	}

	// Cascade delete: First get all workspaces owned by the user
	if db != nil {
		workspaceRepo := dbpkg.WorkspaceRepo()
		workspaces, err := workspaceRepo.FindByOwnerID(userID, 1000, 0) // Get all workspaces
		if err != nil {
			log.Printf("WARNING: Failed to fetch workspaces for user %s: %v", userID, err)
		}

		// Delete all workspaces (which will cascade delete services via database foreign keys)
		for _, workspace := range workspaces {
			// Get all services in this workspace to delete their RDEAgent CRs
			serviceRepo := dbpkg.ServiceRepo()
			services, err := serviceRepo.FindByWorkspaceID(workspace.ID, 1000, 0) // Get all services
			if err != nil {
				log.Printf("WARNING: Failed to fetch services for workspace %s: %v", workspace.ID, err)
			}

			// Delete RDEAgent CRs for each service
			for _, service := range services {
				if service.AgentID != "" && dynamicClient != nil {
					err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Delete(
						ctx,
						service.AgentID,
						metav1.DeleteOptions{},
					)
					if err != nil {
						log.Printf("WARNING: Failed to delete RDEAgent CR %s: %v", service.AgentID, err)
					} else {
						log.Printf("✓ Deleted RDEAgent CR: %s", service.AgentID)
					}
				}
			}

			// Delete the workspace (database will cascade delete services)
			if err := workspaceRepo.Delete(workspace.ID); err != nil {
				log.Printf("WARNING: Failed to delete workspace %s: %v", workspace.ID, err)
			} else {
				log.Printf("✓ Deleted workspace: %s", workspace.ID)
			}
		}

		// Delete user from database
		userRepo := dbpkg.UserRepo()
		if err := userRepo.Delete(userID); err != nil {
			log.Printf("WARNING: Failed to delete user from database: %v", err)
		}

		// Delete user quota
		if userQuotaRepo != nil {
			if err := userQuotaRepo.Delete(userID); err != nil {
				log.Printf("WARNING: Failed to delete user quota: %v", err)
			}
		}

		// Log audit entry
		auditRepo := dbpkg.AuditLogRepo()
		if err := auditRepo.LogAction(claims.Sub, "delete", "user", userID, "", ""); err != nil {
			log.Printf("WARNING: Failed to log audit entry: %v", err)
		}
	}

	// Delete user from Keycloak
	err = keycloakClient.DeleteUser(
		ctx,
		getAdminToken(),
		"kuberde",
		userID,
	)
	if err != nil {
		http.Error(w, "Failed to delete user from Keycloak: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✓ User %s deleted successfully (by %s)", userID, claims.Sub)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "User and all associated data deleted successfully",
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// --- Resource Quota Management Handlers ---

// GET /api/admin/resource-config - Get system resource configuration (admin only)
func handleGetResourceConfig(w http.ResponseWriter, r *http.Request) {
	if resourceConfigRepo == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	config, err := resourceConfigRepo.GetConfig()
	if err != nil {
		http.Error(w, "Failed to get resource config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(config)
}

// PUT /api/admin/resource-config - Update system resource configuration (admin only)
func handleUpdateResourceConfig(w http.ResponseWriter, r *http.Request) {
	if resourceConfigRepo == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var config models.ResourceConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := resourceConfigRepo.UpdateConfig(&config); err != nil {
		http.Error(w, "Failed to update resource config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(config)
}

// GET /api/users/{id}/quota - Get user's quota
func handleGetUserQuota(w http.ResponseWriter, r *http.Request, userID string) {
	if userQuotaRepo == nil || resourceConfigRepo == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	quota, err := userQuotaRepo.GetByUserID(userID)
	if err != nil {
		// If not found, create default quota
		config, err := resourceConfigRepo.GetConfig()
		if err != nil {
			http.Error(w, "Failed to get resource config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		quota = createDefaultQuota(userID, config)
		if err := userQuotaRepo.Create(quota); err != nil {
			log.Printf("WARNING: Failed to create default quota for user %s: %v", userID, err)
			// Return the default quota anyway, even if we couldn't save it
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(quota)
}

// PUT /api/users/{id}/quota - Update user's quota (admin only)
func handleUpdateUserQuota(w http.ResponseWriter, r *http.Request, userID string) {
	if userQuotaRepo == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	// Use a request struct that accepts objects for JSONB fields
	var req struct {
		CPUCores     int                           `json:"cpu_cores"`
		MemoryGi     int                           `json:"memory_gi"`
		StorageQuota []models.UserStorageQuotaItem `json:"storage_quota,omitempty"`
		GPUQuota     []models.UserGPUQuotaItem     `json:"gpu_quota,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Prepare UserQuota object
	quota := models.UserQuota{
		UserID:   userID,
		CPUCores: req.CPUCores,
		MemoryGi: req.MemoryGi,
	}

	// Convert storage_quota to JSON RawMessage
	if req.StorageQuota != nil {
		bytes, err := json.Marshal(req.StorageQuota)
		if err != nil {
			http.Error(w, "Failed to serialize storage_quota: "+err.Error(), http.StatusBadRequest)
			return
		}
		raw := json.RawMessage(bytes)
		quota.StorageQuota = &raw
	} else {
		raw := json.RawMessage([]byte("[]"))
		quota.StorageQuota = &raw
	}

	// Convert gpu_quota to JSON RawMessage
	if req.GPUQuota != nil {
		bytes, err := json.Marshal(req.GPUQuota)
		if err != nil {
			http.Error(w, "Failed to serialize gpu_quota: "+err.Error(), http.StatusBadRequest)
			return
		}
		raw := json.RawMessage(bytes)
		quota.GPUQuota = &raw
	} else {
		raw := json.RawMessage([]byte("[]"))
		quota.GPUQuota = &raw
	}

	// Check if quota exists
	_, err := userQuotaRepo.GetByUserID(userID)
	if err != nil {
		// Create new quota
		if err := userQuotaRepo.Create(&quota); err != nil {
			http.Error(w, "Failed to create user quota: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Update existing quota
		if err := userQuotaRepo.Update(&quota); err != nil {
			http.Error(w, "Failed to update user quota: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(quota)
}

// handleUserSSHKeys handles SSH key management for a user
func handleUserSSHKeys(w http.ResponseWriter, r *http.Request, userID string, parts []string) {
	if db == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	userRepo := dbpkg.UserRepo()

	switch r.Method {
	case http.MethodGet:
		// GET /api/users/{id}/ssh-keys - Get all SSH keys
		user, err := userRepo.FindByID(userID)
		if err != nil {
			http.Error(w, "Failed to fetch user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Parse SSH keys from JSONB
		var sshKeys []models.SSHKey
		if user.SSHKeys != nil {
			if err := json.Unmarshal(*user.SSHKeys, &sshKeys); err != nil {
				log.Printf("ERROR: Failed to parse SSH keys: %v", err)
				sshKeys = []models.SSHKey{}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sshKeys)

	case http.MethodPost:
		// POST /api/users/{id}/ssh-keys - Add new SSH key
		var req struct {
			Name      string `json:"name"`
			PublicKey string `json:"public_key"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.PublicKey == "" {
			http.Error(w, "Name and public_key are required", http.StatusBadRequest)
			return
		}

		// Get user
		user, err := userRepo.FindByID(userID)
		if err != nil {
			http.Error(w, "Failed to fetch user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Parse existing SSH keys
		var sshKeys []models.SSHKey
		if user.SSHKeys != nil {
			if err := json.Unmarshal(*user.SSHKeys, &sshKeys); err != nil {
				log.Printf("ERROR: Failed to parse SSH keys: %v", err)
				sshKeys = []models.SSHKey{}
			}
		}

		// Calculate fingerprint (simple SHA256 hash of the public key)
		h := sha256.New()
		h.Write([]byte(req.PublicKey))
		fingerprint := fmt.Sprintf("SHA256:%x", h.Sum(nil))[:20]

		// Create new SSH key
		newKey := models.SSHKey{
			ID:          uuid.New().String(),
			Name:        req.Name,
			PublicKey:   req.PublicKey,
			Fingerprint: fingerprint,
			AddedAt:     time.Now(),
		}

		// Append to existing keys
		sshKeys = append(sshKeys, newKey)

		// Marshal back to JSON
		sshKeysJSON, err := json.Marshal(sshKeys)
		if err != nil {
			http.Error(w, "Failed to serialize SSH keys: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Update user
		rawMessage := json.RawMessage(sshKeysJSON)
		user.SSHKeys = &rawMessage
		if err := userRepo.Update(user); err != nil {
			http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(newKey)

	case http.MethodDelete:
		// DELETE /api/users/{id}/ssh-keys/{keyId} - Delete SSH key
		if len(parts) < 6 {
			http.Error(w, "Invalid path, key ID required", http.StatusBadRequest)
			return
		}
		keyID := parts[5]

		// Get user
		user, err := userRepo.FindByID(userID)
		if err != nil {
			http.Error(w, "Failed to fetch user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Parse existing SSH keys
		var sshKeys []models.SSHKey
		if user.SSHKeys != nil {
			if err := json.Unmarshal(*user.SSHKeys, &sshKeys); err != nil {
				log.Printf("ERROR: Failed to parse SSH keys: %v", err)
				http.Error(w, "Failed to parse SSH keys", http.StatusInternalServerError)
				return
			}
		}

		// Filter out the key to delete
		found := false
		var updatedKeys []models.SSHKey
		for _, key := range sshKeys {
			if key.ID != keyID {
				updatedKeys = append(updatedKeys, key)
			} else {
				found = true
			}
		}

		if !found {
			http.Error(w, "SSH key not found", http.StatusNotFound)
			return
		}

		// Marshal back to JSON
		sshKeysJSON, err := json.Marshal(updatedKeys)
		if err != nil {
			http.Error(w, "Failed to serialize SSH keys: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Update user
		rawMessage := json.RawMessage(sshKeysJSON)
		user.SSHKeys = &rawMessage
		if err := userRepo.Update(user); err != nil {
			http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Helper: Create default quota for a user
func createDefaultQuota(userID string, config *models.ResourceConfig) *models.UserQuota {
	// Construct StorageQuota array
	var storageQuotaItems []models.UserStorageQuotaItem
	var storageClasses []map[string]interface{}
	if err := json.Unmarshal([]byte(config.StorageClasses), &storageClasses); err == nil {
		for _, sc := range storageClasses {
			// Use limit_gi as the quota for this class
			if limitGi, ok := sc["limit_gi"].(float64); ok {
				name := "standard"
				if n, ok := sc["name"].(string); ok {
					name = n
				}
				storageQuotaItems = append(storageQuotaItems, models.UserStorageQuotaItem{
					Name:    name,
					LimitGi: int(limitGi),
				})
			}
		}
	}
	// Marshal storage quota
	storageQuotaBytes, _ := json.Marshal(storageQuotaItems)
	storageQuotaJSON := json.RawMessage(storageQuotaBytes)

	// Construct GPUQuota array
	var gpuQuotaItems []models.UserGPUQuotaItem
	var gpuTypes []map[string]interface{}
	if err := json.Unmarshal([]byte(config.GPUTypes), &gpuTypes); err == nil {
		for _, gpu := range gpuTypes {
			// Use limit as the quota for this type
			if limit, ok := gpu["limit"].(float64); ok {
				name := "nvidia"
				if n, ok := gpu["name"].(string); ok {
					name = n
				}
				modelName := "nvidia"
				if mn, ok := gpu["model_name"].(string); ok {
					modelName = mn
				} else if n, ok := gpu["name"].(string); ok {
					modelName = n
				}

				gpuQuotaItems = append(gpuQuotaItems, models.UserGPUQuotaItem{
					Name:      name,
					ModelName: modelName,
					Limit:     int(limit),
				})
			}
		}
	}
	// Marshal gpu quota
	gpuQuotaBytes, _ := json.Marshal(gpuQuotaItems)
	gpuQuotaJSON := json.RawMessage(gpuQuotaBytes)

	return &models.UserQuota{
		UserID:       userID,
		CPUCores:     config.DefaultCPUCores,
		MemoryGi:     config.DefaultMemoryGi,
		StorageQuota: &storageQuotaJSON,
		GPUQuota:     &gpuQuotaJSON,
	}
}

// Helper: Parse storage size string (e.g., "10Gi") to integer Gi
func parseStorageSize(sizeStr string) int {
	sizeStr = strings.TrimSpace(sizeStr)
	if strings.HasSuffix(sizeStr, "Gi") {
		numStr := strings.TrimSuffix(sizeStr, "Gi")
		if val, err := strconv.Atoi(numStr); err == nil {
			return val
		}
	}
	// Default to 10Gi if parse fails
	return 10
}

// Helper: Check if StorageClass is supported in resource config
func isStorageClassSupported(storageClass, storageClassesJSON string) bool {
	if storageClassesJSON == "" || storageClassesJSON == "[]" {
		return true // If no classes defined, allow any
	}

	var classes []map[string]interface{}
	if err := json.Unmarshal([]byte(storageClassesJSON), &classes); err != nil {
		return true // On error, allow
	}

	for _, class := range classes {
		if name, ok := class["name"].(string); ok && name == storageClass {
			return true
		}
	}
	return false
}

// --- Workspace & Service API Handlers (Phase 3) ---

// Helper: Extract user ID from JWT claims in request context
// Tries Authorization header first, then falls back to session cookie
func extractUserIDFromRequest(r *http.Request) (string, error) {
	// Try Authorization header first
	token, err := validateToken(r)
	if err != nil {
		// Fall back to session cookie (for web browser requests)
		token, err = validateCookie(r)
		if err != nil {
			return "", fmt.Errorf("authentication failed: no valid token or session cookie")
		}
	}
	var claims struct {
		Subject string `json:"sub"`
	}
	if err := token.Claims(&claims); err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// Helper: Check if user has admin role
func userHasAdminRole(r *http.Request) bool {
	token, err := validateToken(r)
	if err != nil {
		token, err = validateCookie(r)
		if err != nil {
			return false
		}
	}

	var claims struct {
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}

	if err := token.Claims(&claims); err != nil {
		return false
	}

	for _, role := range claims.RealmAccess.Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}

// logAdminAccess logs when an admin user accesses resources owned by other users
func logAdminAccess(adminUserID, adminUsername, action, resource, resourceID, targetOwner string) {
	if db == nil {
		log.Printf("⚠️  ADMIN ACCESS (DB unavailable): admin=%s action=%s resource=%s/%s target_owner=%s",
			adminUsername, action, resource, resourceID, targetOwner)
		return
	}

	// Create detailed audit log entry
	details := fmt.Sprintf("Admin user '%s' (ID: %s) performed '%s' on %s '%s' owned by user '%s'",
		adminUsername, adminUserID, action, resource, resourceID, targetOwner)

	auditRepo := dbpkg.AuditLogRepo()
	err := auditRepo.LogAction(
		adminUserID,
		"admin_"+action, // Prefix with "admin_" to distinguish from regular actions
		resource,
		resourceID,
		"", // old_data
		details,
	)

	if err != nil {
		log.Printf("ERROR: Failed to log admin access: %v", err)
	}

	// Also log to console for real-time monitoring
	log.Printf("⚠️  ADMIN ACCESS: admin=%s action=%s resource=%s/%s target_owner=%s",
		adminUsername, action, resource, resourceID, targetOwner)
}

// logAdminServiceAccess logs admin access to a service (helper for API endpoints)
func logAdminServiceAccess(r *http.Request, adminUserID, adminUsername, action string, service *models.Service) {
	if service == nil || db == nil {
		return
	}

	// Get workspace to find the owner
	workspace, err := dbpkg.WorkspaceRepo().FindByID(service.WorkspaceID)
	if err != nil || workspace == nil {
		log.Printf("WARNING: Could not get workspace for admin audit log")
		return
	}

	// Get owner information
	var targetOwner string
	if workspace.Owner != nil {
		targetOwner = workspace.Owner.Username
	} else if workspace.OwnerID != "" {
		owner, err := dbpkg.UserRepo().FindByID(workspace.OwnerID)
		if err == nil && owner != nil {
			targetOwner = owner.Username
		}
	}

	// Only log if admin is accessing another user's service
	if targetOwner != "" && targetOwner != adminUsername {
		logAdminAccess(adminUserID, adminUsername, action, "service", service.ID, targetOwner)
	}
}

// Helper: Check if user owns a workspace
func userOwnsWorkspace(userID, workspaceID string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database not initialized")
	}
	workspace, err := dbpkg.WorkspaceRepo().FindByID(workspaceID)
	if err != nil {
		return false, err
	}
	if workspace == nil {
		return false, fmt.Errorf("workspace not found")
	}
	return workspace.OwnerID == userID, nil
}

// GET /api/workspaces - List user's workspaces
func handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Get workspaces owned by user
	workspaces, err := dbpkg.WorkspaceRepo().FindByOwnerID(userID, 100, 0)
	if err != nil {
		http.Error(w, "Failed to list workspaces: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if workspaces == nil {
		workspaces = []models.Workspace{}
	}

	// Update service status from CR status (real-time) for all workspaces
	user, _ := dbpkg.UserRepo().FindByID(userID)
	for i := range workspaces {
		if workspaces[i].Services == nil {
			continue
		}

		for j := range workspaces[i].Services {
			service := &workspaces[i].Services[j]
			agentID := service.AgentID

			// If AgentID is empty but service has a template, try to derive it
			if agentID == "" && service.TemplateID.Valid && user != nil {
				// Try new naming convention first
				derivedAgentID := generateAgentName(service.CreatedByID, user.Username, service.WorkspaceID, workspaces[i].Name, service.Name)
				if dynamicClient != nil {
					_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), derivedAgentID, metav1.GetOptions{})
					if err == nil {
						agentID = derivedAgentID
					} else {
						// Try old naming convention as fallback
						oldAgentID := fmt.Sprintf("kuberde-%s-%s-%s", service.CreatedByID, service.WorkspaceID, service.Name)
						_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), oldAgentID, metav1.GetOptions{})
						if err == nil {
							agentID = oldAgentID
						}
					}
				}
			}

			if agentID == "" {
				service.Status = "unknown"
				continue
			}

			// Try to get CR status from Kubernetes
			if dynamicClient != nil {
				cr, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), agentID, metav1.GetOptions{})
				if err == nil {
					// Extract status from CR
					if status, found, err := unstructured.NestedMap(cr.Object, "status"); found && err == nil {
						if phase, ok := status["phase"].(string); ok {
							switch phase {
							case "Running":
								service.Status = "running"
							case "Disconnected", "ScaledDown", "Pending":
								service.Status = "stopped"
							case "Error":
								service.Status = "error"
							default:
								service.Status = "unknown"
							}
							continue
						}
					}
				}
			}

			// Fallback to agentStats if CR not available
			statsMu.RLock()
			stats, exists := agentStats[agentID]
			statsMu.RUnlock()

			if exists && stats.Online {
				service.Status = "running"
			} else {
				service.Status = "stopped"
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"workspaces": workspaces,
	})
}

// POST /api/workspaces - Create a new workspace
func handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Safety check: Ensure user exists in database before creating workspace
	// This handles cases where ensureUserExists failed during login
	if db != nil {
		userRepo := dbpkg.UserRepo()
		existingUser, err := userRepo.FindByID(userID)
		if err != nil {
			log.Printf("ERROR: Failed to check user existence for %s: %v", userID, err)
			http.Error(w, "Failed to verify user account", http.StatusInternalServerError)
			return
		}
		if existingUser == nil {
			// User doesn't exist in database - try to create from token claims
			token, err := validateToken(r)
			if err != nil {
				token, err = validateCookie(r)
			}
			if err == nil {
				var claims struct {
					Subject           string `json:"sub"`
					PreferredUsername string `json:"preferred_username"`
					Email             string `json:"email"`
					Name              string `json:"name"`
				}
				if err := token.Claims(&claims); err == nil {
					// Attempt to create user record
					if err := ensureUserExists(claims.Subject, claims.PreferredUsername, claims.Email, claims.Name); err != nil {
						log.Printf("ERROR: Failed to create user record for %s: %v", claims.PreferredUsername, err)
						http.Error(w, "Failed to create user account. Please contact administrator.", http.StatusInternalServerError)
						return
					}
					log.Printf("✓ Successfully created user record for %s on-demand", claims.PreferredUsername)
				} else {
					log.Printf("ERROR: Failed to extract claims from token: %v", err)
					http.Error(w, "Invalid authentication token", http.StatusUnauthorized)
					return
				}
			} else {
				log.Printf("ERROR: User %s not found in database and cannot extract token claims", userID)
				http.Error(w, "User account not found. Please log out and log in again.", http.StatusUnauthorized)
				return
			}
		}
	}

	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		StorageSize  string `json:"storage_size"`
		StorageClass string `json:"storage_class"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Workspace name is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.StorageSize == "" {
		req.StorageSize = "10Gi"
	}
	if req.StorageClass == "" {
		req.StorageClass = "standard"
	}

	// Validate storage size and class against user quota
	if resourceConfigRepo != nil && userQuotaRepo != nil {
		// Parse storage size
		sizeGi := parseStorageSize(req.StorageSize)

		// Get or create user quota
		quota, err := userQuotaRepo.GetByUserID(userID)
		if err != nil {
			// Create default quota if missing
			config, _ := resourceConfigRepo.GetConfig()
			if config != nil {
				quota = createDefaultQuota(userID, config)
				if err := userQuotaRepo.Create(quota); err != nil {
					log.Printf("Failed to create default user quota: %v", err)
				}
			}
		}

		if quota != nil {
			// Check if StorageClass is supported
			config, _ := resourceConfigRepo.GetConfig()
			if config != nil && !isStorageClassSupported(req.StorageClass, config.StorageClasses) {
				http.Error(w, "StorageClass not supported", http.StatusBadRequest)
				return
			}

			// Check storage quota
			if quota.StorageQuota != nil {
				var items []models.UserStorageQuotaItem
				if err := json.Unmarshal(*quota.StorageQuota, &items); err == nil {
					found := false
					for _, item := range items {
						if item.Name == req.StorageClass {
							found = true
							if sizeGi > item.LimitGi {
								http.Error(w, fmt.Sprintf("Storage size %dGi exceeds quota %dGi for class %s", sizeGi, item.LimitGi, req.StorageClass), http.StatusForbidden)
								return
							}
							break
						}
					}
					if !found {
						// No quota defined for this class
						http.Error(w, fmt.Sprintf("No quota for storage class %s", req.StorageClass), http.StatusForbidden)
						return
					}
				}
			}
		}
	}

	// Get user information for PVC naming
	user, err := dbpkg.UserRepo().FindByID(userID)
	if err != nil || user == nil {
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}

	// Create workspace
	workspace := &models.Workspace{
		Name:         req.Name,
		Description:  req.Description,
		OwnerID:      userID,
		StorageSize:  req.StorageSize,
		StorageClass: req.StorageClass,
	}

	if err := dbpkg.WorkspaceRepo().Create(workspace); err != nil {
		http.Error(w, "Failed to create workspace: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate PVC name using userName and workspaceName
	pvcName := generateWorkspacePVCName(userID, user.Username, workspace.ID, workspace.Name)
	workspace.PVCName = pvcName

	// Create PVC in Kubernetes if client is available
	if k8sClientset != nil {
		go func() {
			if err := createWorkspacePVC(context.Background(), user.Username, workspace); err != nil {
				log.Printf("WARNING: Failed to create PVC %s: %v", pvcName, err)
				// Don't fail the API call, PVC creation is async
			} else {
				log.Printf("✓ Created PVC %s for workspace %s", pvcName, workspace.ID)
				// Update workspace record with PVC name
				if err := dbpkg.WorkspaceRepo().Update(workspace); err != nil {
					log.Printf("WARNING: Failed to update workspace with PVC name: %v", err)
				}
			}
		}()
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "create", "workspace", workspace.ID, "", ""); err != nil {
		log.Printf("Failed to log audit action: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(workspace); err != nil {
		log.Printf("Failed to encode workspace response: %v", err)
	}
}

// GET /api/workspaces/:id - Get a single workspace
func handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract workspace ID from path
	workspaceID := strings.TrimPrefix(r.URL.Path, "/api/workspaces/")
	if workspaceID == "" {
		http.Error(w, "Workspace ID is required", http.StatusBadRequest)
		return
	}

	// Check ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, workspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	workspace, err := dbpkg.WorkspaceRepo().FindByID(workspaceID)
	if err != nil || workspace == nil {
		http.Error(w, "Workspace not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(workspace)
}

// DELETE /api/workspaces/:id - Delete a workspace
func handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract workspace ID from path
	workspaceID := strings.TrimPrefix(r.URL.Path, "/api/workspaces/")
	if workspaceID == "" {
		http.Error(w, "Workspace ID is required", http.StatusBadRequest)
		return
	}

	// Check ownership (admins can delete any workspace)
	isAdmin := userHasAdminRole(r)
	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, workspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Get workspace to find PVC name
	workspace, err := dbpkg.WorkspaceRepo().FindByID(workspaceID)
	if err != nil || workspace == nil {
		http.Error(w, "Workspace not found", http.StatusNotFound)
		return
	}

	// Delete all agents (RDEAgent CRs) associated with services in this workspace
	services, err := dbpkg.ServiceRepo().FindByWorkspaceID(workspaceID, 1000, 0)
	if err == nil && dynamicClient != nil {
		for _, service := range services {
			if service.AgentID != "" {
				log.Printf("Deleting agent CR %s for service %s", service.AgentID, service.ID)
				err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Delete(context.TODO(), service.AgentID, metav1.DeleteOptions{})
				if err != nil {
					log.Printf("WARNING: Failed to delete agent CR %s: %v", service.AgentID, err)
				}
			}
		}
	}

	// Delete PVC if it exists
	if workspace.PVCName != "" && k8sClientset != nil {
		pvcClient := k8sClientset.CoreV1().PersistentVolumeClaims(kuberdeNamespace)
		if err := pvcClient.Delete(context.Background(), workspace.PVCName, metav1.DeleteOptions{}); err != nil {
			log.Printf("WARNING: Failed to delete PVC %s: %v", workspace.PVCName, err)
			// Continue with workspace deletion even if PVC deletion fails
		} else {
			log.Printf("✓ Deleted PVC %s", workspace.PVCName)
		}
	}

	// Delete workspace from database
	if err := dbpkg.WorkspaceRepo().Delete(workspaceID); err != nil {
		http.Error(w, "Failed to delete workspace: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "delete", "workspace", workspaceID, "", ""); err != nil {
		log.Printf("Failed to log audit action: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Workspace deleted successfully",
	})
}

// GET /api/workspaces/:id/services - List services in a workspace
// inferAgentTypeFromService attempts to infer agent type from service characteristics
func inferAgentTypeFromService(svc *models.Service) string {
	serviceName := strings.ToLower(svc.Name)

	// Check by common service names
	if strings.Contains(serviceName, "ssh") {
		return "ssh"
	}
	if strings.Contains(serviceName, "jupyter") {
		return "jupyter"
	}
	if strings.Contains(serviceName, "coder") || strings.Contains(serviceName, "vscode") || strings.Contains(serviceName, "code-server") {
		return "coder"
	}
	if strings.Contains(serviceName, "file") || strings.Contains(serviceName, "filebrowser") {
		return "file"
	}

	// Check by port
	switch svc.ExternalPort {
	case 22:
		return "ssh"
	case 8888:
		return "jupyter"
	case 8080, 8443:
		if strings.Contains(serviceName, "code") {
			return "coder"
		}
	}

	return ""
}

func handleListWorkspaceServices(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract workspace ID from path: /api/workspaces/{id}/services
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		// Try to extract from path if using path-based routing
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/workspaces/"), "/")
		if len(parts) > 0 {
			workspaceID = parts[0]
		}
	}

	if workspaceID == "" {
		http.Error(w, "Workspace ID is required", http.StatusBadRequest)
		return
	}

	// Check if user owns the workspace (admins can access any)
	isAdmin := userHasAdminRole(r)
	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, workspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	// Get services
	services, err := dbpkg.ServiceRepo().FindByWorkspaceID(workspaceID, 100, 0)
	if err != nil {
		http.Error(w, "Failed to list services: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if services == nil {
		services = []models.Service{}
	}

	// Get user and workspace information for auto-repair naming
	var user *models.User
	var workspace *models.Workspace
	if len(services) > 0 {
		user, _ = dbpkg.UserRepo().FindByID(userID)
		workspace, _ = dbpkg.WorkspaceRepo().FindByID(workspaceID)
	}

	// Update service status from CR status (real-time)
	for i := range services {
		agentID := services[i].AgentID

		// If AgentID is empty but service has a template, try to derive and update it
		if agentID == "" && services[i].TemplateID.Valid && services[i].WorkspaceID != "" && user != nil && workspace != nil {
			// Try new naming convention first: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash}
			derivedAgentID := generateAgentName(services[i].CreatedByID, user.Username, services[i].WorkspaceID, workspace.Name, services[i].Name)

			// Check if CR exists with this name
			if dynamicClient != nil {
				_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), derivedAgentID, metav1.GetOptions{})
				if err == nil {
					// CR exists! Update the service record
					services[i].AgentID = derivedAgentID
					if updateErr := dbpkg.ServiceRepo().Update(&services[i]); updateErr != nil {
						log.Printf("WARNING: Failed to update AgentID for service %s: %v", services[i].ID, updateErr)
					} else {
						log.Printf("✓ Updated service %s with derived AgentID: %s", services[i].ID, derivedAgentID)
						agentID = derivedAgentID
					}
				} else {
					// Try old naming convention as fallback: kuberde-{userID}-{workspaceID}-{serviceName}
					oldAgentID := fmt.Sprintf("kuberde-%s-%s-%s", services[i].CreatedByID, services[i].WorkspaceID, services[i].Name)
					_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), oldAgentID, metav1.GetOptions{})
					if err == nil {
						services[i].AgentID = oldAgentID
						if updateErr := dbpkg.ServiceRepo().Update(&services[i]); updateErr != nil {
							log.Printf("WARNING: Failed to update AgentID for service %s: %v", services[i].ID, updateErr)
						} else {
							log.Printf("✓ Updated service %s with old-style AgentID: %s", services[i].ID, oldAgentID)
							agentID = oldAgentID
						}
					}
				}
			}
		}

		if agentID == "" {
			services[i].Status = "unknown"
			continue
		}

		// Try to get CR status from Kubernetes
		if dynamicClient != nil {
			cr, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), agentID, metav1.GetOptions{})
			if err == nil {
				// Extract status from CR
				if status, found, err := unstructured.NestedMap(cr.Object, "status"); found && err == nil {
					if phase, ok := status["phase"].(string); ok {
						switch phase {
						case "Running":
							services[i].Status = "running"
						case "Disconnected", "ScaledDown":
							services[i].Status = "stopped"
						case "Starting", "Pending":
							services[i].Status = "starting"
						case "Error":
							services[i].Status = "error"
						default:
							services[i].Status = "unknown"
						}
						continue
					}
				}
			}
		}

		// Fallback to agentStats if CR not available
		statsMu.RLock()
		stats, exists := agentStats[agentID]
		statsMu.RUnlock()

		if exists && stats.Online {
			services[i].Status = "running"
		} else {
			services[i].Status = "stopped"
		}
	}

	// Convert services to include remote_proxy field and ensure agent_type is populated
	serviceResponses := make([]map[string]interface{}, len(services))
	for i := range services {
		// Load template if available to get agent_type
		if services[i].TemplateID.Valid && !services[i].AgentType.Valid {
			template, err := dbpkg.AgentTemplateRepo().GetByID(context.TODO(), services[i].TemplateID.String)
			if err == nil && template != nil {
				services[i].AgentType = sql.NullString{String: template.AgentType, Valid: true}
				services[i].Template = template
			}
		}

		// For services without template, try to infer agent_type from service name or port
		if !services[i].AgentType.Valid {
			inferredType := inferAgentTypeFromService(&services[i])
			if inferredType != "" {
				services[i].AgentType = sql.NullString{String: inferredType, Valid: true}
			}
		}

		// Marshal service to map
		svcBytes, _ := json.Marshal(&services[i])
		var svcMap map[string]interface{}
		_ = json.Unmarshal(svcBytes, &svcMap)

		// Add remote_proxy field
		if services[i].AgentID != "" {
			svcMap["remote_proxy"] = fmt.Sprintf("%s.%s", services[i].AgentID, agentDomain)
		}

		// Ensure agent_type is always present in response (even if null)
		if _, exists := svcMap["agent_type"]; !exists && services[i].AgentType.Valid {
			svcMap["agent_type"] = services[i].AgentType.String
		}

		serviceResponses[i] = svcMap
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"services": serviceResponses,
	})
}

// POST /api/workspaces/:id/services - Create a service in a workspace

// ensureUserExistsInDB verifies user exists and creates if needed from token
func ensureUserExistsInDB(userID string, r *http.Request) error {
	if db == nil {
		return nil
	}

	userRepo := dbpkg.UserRepo()
	existingUser, err := userRepo.FindByID(userID)
	if err != nil {
		log.Printf("ERROR: Failed to check user existence for %s: %v", userID, err)
		return fmt.Errorf("failed to verify user account")
	}

	if existingUser != nil {
		return nil
	}

	// User doesn't exist - try to create from token
	token, err := validateToken(r)
	if err != nil {
		token, err = validateCookie(r)
	}
	if err != nil {
		log.Printf("ERROR: User %s not found in database and cannot extract token claims", userID)
		return fmt.Errorf("user account not found. Please log out and log in again")
	}

	var claims struct {
		Subject           string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Name              string `json:"name"`
	}

	if err := token.Claims(&claims); err != nil {
		log.Printf("ERROR: Failed to extract claims from token: %v", err)
		return fmt.Errorf("invalid authentication token")
	}

	if err := ensureUserExists(claims.Subject, claims.PreferredUsername, claims.Email, claims.Name); err != nil {
		log.Printf("ERROR: Failed to create user record for %s: %v", claims.PreferredUsername, err)
		return fmt.Errorf("failed to create user account. Please contact administrator")
	}

	log.Printf("✓ Successfully created user record for %s on-demand", claims.PreferredUsername)
	return nil
}

// extractWorkspaceID gets workspace ID from query or path
func extractWorkspaceID(r *http.Request) (string, error) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/workspaces/"), "/")
		if len(parts) > 0 {
			workspaceID = parts[0]
		}
	}
	if workspaceID == "" {
		return "", fmt.Errorf("workspace ID is required")
	}
	return workspaceID, nil
}

// loadAgentTemplate loads template by ID
func loadAgentTemplate(templateID string) (*models.AgentTemplate, error) {
	if templateID == "" {
		return nil, nil
	}

	template, err := dbpkg.AgentTemplateRepo().GetByID(context.Background(), templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	if template == nil {
		return nil, fmt.Errorf("template not found")
	}

	return template, nil
}

// applyTemplateDefaults applies template defaults to service request
func applyTemplateDefaults(template *models.AgentTemplate, localTarget *string, externalPort *int) {
	if template == nil {
		return
	}
	if *localTarget == "" {
		*localTarget = template.DefaultLocalTarget
	}
	if *externalPort <= 0 {
		*externalPort = template.DefaultExternalPort
	}
}

// autoLookupGPUConfig automatically fills in GPU resource name and node selector from model
func autoLookupGPUConfig(gpuModel string, gpuResourceName *string, gpuNodeSelector *map[string]interface{}) {
	if gpuModel == "" || resourceConfigRepo == nil {
		return
	}
	if *gpuResourceName != "" && len(*gpuNodeSelector) > 0 {
		return
	}

	config, err := resourceConfigRepo.GetConfig()
	if err != nil || config == nil || config.GPUTypes == "" {
		return
	}

	var gpuTypes []map[string]interface{}
	if err := json.Unmarshal([]byte(config.GPUTypes), &gpuTypes); err != nil {
		return
	}

	for _, gpuType := range gpuTypes {
		modelName, ok := gpuType["model_name"].(string)
		if !ok || modelName != gpuModel {
			continue
		}

		if *gpuResourceName == "" {
			if resourceName, ok := gpuType["resource_name"].(string); ok {
				*gpuResourceName = resourceName
			}
		}

		if len(*gpuNodeSelector) == 0 {
			labelKey, _ := gpuType["node_label_key"].(string)
			labelValue, _ := gpuType["node_label_value"].(string)
			if labelKey != "" && labelValue != "" {
				*gpuNodeSelector = map[string]interface{}{
					labelKey: labelValue,
				}
			}
		}
		break
	}
}

func handleCreateWorkspaceService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if err := ensureUserExistsInDB(userID, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	workspaceID, err := extractWorkspaceID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	owns, err := userOwnsWorkspace(userID, workspaceID)
	if err != nil {
		http.Error(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if !owns {
		http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
		return
	}

	var req struct {
		Name         string                 `json:"name"`
		LocalTarget  string                 `json:"local_target"`
		ExternalPort int                    `json:"external_port"`
		AgentID      string                 `json:"agent_id"`
		TemplateID   string                 `json:"template_id"`
		StartupArgs  string                 `json:"startup_args"`
		EnvVars      map[string]interface{} `json:"env_vars"`
		// Resource configuration
		CPUCores        string                 `json:"cpu_cores"`         // e.g., "4"
		MemoryGiB       string                 `json:"memory_gib"`        // e.g., "16"
		GPUCount        int64                  `json:"gpu_count"`         // Number of GPUs
		GPUModel        string                 `json:"gpu_model"`         // e.g., "NVIDIA A100"
		GPUResourceName string                 `json:"gpu_resource_name"` // e.g., "nvidia.com/gpu"
		GPUNodeSelector map[string]interface{} `json:"gpu_node_selector"` // e.g., {"nvidia.com/model": "A100"}
		TTL             string                 `json:"ttl"`               // Idle timeout (e.g., "24h", "0")
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	template, err := loadAgentTemplate(req.TemplateID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	applyTemplateDefaults(template, &req.LocalTarget, &req.ExternalPort)

	if req.LocalTarget == "" || req.ExternalPort <= 0 {
		http.Error(w, "local_target and external_port are required (either in request or from template)", http.StatusBadRequest)
		return
	}

	var envVarsJSON *json.RawMessage
	if req.EnvVars != nil {
		envVarsBytes, err := json.Marshal(req.EnvVars)
		if err != nil {
			http.Error(w, "Invalid env_vars format: "+err.Error(), http.StatusBadRequest)
			return
		}
		raw := json.RawMessage(envVarsBytes)
		envVarsJSON = &raw
	}

	autoLookupGPUConfig(req.GPUModel, &req.GPUResourceName, &req.GPUNodeSelector)

	// Convert GPU node selector to JSON if provided
	var gpuNodeSelectorJSON *json.RawMessage
	if len(req.GPUNodeSelector) > 0 {
		gpuNodeSelectorBytes, err := json.Marshal(req.GPUNodeSelector)
		if err != nil {
			http.Error(w, "Failed to encode GPU node selector: "+err.Error(), http.StatusBadRequest)
			return
		}
		gpuNodeSelectorJSON = (*json.RawMessage)(&gpuNodeSelectorBytes)
	}

	// Create service
	// Default TTL to 24h if not provided
	ttl := req.TTL
	if ttl == "" {
		ttl = "24h"
	}

	service := &models.Service{
		Name:            req.Name,
		LocalTarget:     req.LocalTarget,
		ExternalPort:    req.ExternalPort,
		AgentID:         req.AgentID,
		WorkspaceID:     workspaceID,
		CreatedByID:     userID,
		Status:          "pending",
		StartupArgs:     sql.NullString{String: req.StartupArgs, Valid: req.StartupArgs != ""},
		EnvVars:         envVarsJSON,
		CPUCores:        sql.NullString{String: req.CPUCores, Valid: req.CPUCores != ""},
		MemoryGiB:       sql.NullString{String: req.MemoryGiB, Valid: req.MemoryGiB != ""},
		GPUCount:        sql.NullInt64{Int64: req.GPUCount, Valid: req.GPUCount > 0},
		GPUModel:        sql.NullString{String: req.GPUModel, Valid: req.GPUModel != ""},
		GPUResourceName: sql.NullString{String: req.GPUResourceName, Valid: req.GPUResourceName != ""},
		GPUNodeSelector: gpuNodeSelectorJSON,
		TTL:             sql.NullString{String: ttl, Valid: ttl != ""},
	}

	// Copy template info if provided
	if template != nil {
		service.TemplateID = sql.NullString{String: template.ID, Valid: true}
		service.AgentType = sql.NullString{String: template.AgentType, Valid: true}
	}

	if err := dbpkg.ServiceRepo().Create(service); err != nil {
		http.Error(w, "Failed to create service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If template is provided, create RDEAgent CR
	if template != nil {
		// Get user and workspace information for naming
		user, err := dbpkg.UserRepo().FindByID(userID)
		if err != nil || user == nil {
			service.Status = "failed"
			if err := dbpkg.ServiceRepo().Update(service); err != nil {
				log.Printf("Failed to update service status to failed: %v", err)
			}
			http.Error(w, "Failed to get user information", http.StatusInternalServerError)
			return
		}

		// Check SSH public keys for SSH service
		if template.AgentType == "ssh" {
			// Check if user has SSH public keys
			var sshKeys []models.SSHKey
			if user.SSHKeys != nil {
				if err := json.Unmarshal(*user.SSHKeys, &sshKeys); err != nil {
					log.Printf("WARNING: Failed to parse user SSH keys: %v", err)
				}
			}

			if len(sshKeys) == 0 {
				// Delete the service we just created
				if err := dbpkg.ServiceRepo().Delete(service.ID); err != nil {
					log.Printf("Failed to delete service: %v", err)
				}
				http.Error(w, "SSH service requires at least one SSH public key. Please add an SSH public key to your profile first.", http.StatusBadRequest)
				return
			}
		}

		workspace, err := dbpkg.WorkspaceRepo().FindByID(workspaceID)
		if err != nil || workspace == nil {
			service.Status = "failed"
			if err := dbpkg.ServiceRepo().Update(service); err != nil {
				log.Printf("Failed to update service status to failed: %v", err)
			}
			http.Error(w, "Failed to get workspace information", http.StatusInternalServerError)
			return
		}

		agentID, err := createRDEAgentFromTemplate(context.Background(), service, template, user, userID, workspaceID, workspace)
		if err != nil {
			// Update service status to failed
			service.Status = "failed"
			if err := dbpkg.ServiceRepo().Update(service); err != nil {
				log.Printf("Failed to update service status to failed: %v", err)
			}
			http.Error(w, "Failed to create agent: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Update service with the generated agent ID
		if agentID != "" {
			service.AgentID = agentID
			if err := dbpkg.ServiceRepo().Update(service); err != nil {
				log.Printf("WARNING: Failed to update service AgentID: %v", err)
			} else {
				log.Printf("✓ Updated service %s with AgentID: %s", service.ID, agentID)
			}
		}
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "create", "service", service.ID, "", ""); err != nil {
		log.Printf("Failed to log audit entry: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(service); err != nil {
		log.Printf("Failed to encode service: %v", err)
	}
}

// PUT /api/services/:id - Update a service
func handleUpdateService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract service ID from path: /api/services/{id}
	serviceID := r.URL.Query().Get("id")
	if serviceID == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
		if len(parts) > 0 {
			serviceID = parts[0]
		}
	}

	if serviceID == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}

	// Get service
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check workspace ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	var adminUsername string
	if isAdmin {
		// Extract admin username for audit logging
		token, err := validateToken(r)
		if err != nil {
			token, _ = validateCookie(r)
		}
		if token != nil {
			var claims struct {
				PreferredUsername string `json:"preferred_username"`
			}
			if err := token.Claims(&claims); err != nil {
				log.Printf("Failed to get claims for admin audit logging: %v", err)
			}
			adminUsername = claims.PreferredUsername
		}
	}

	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	var req struct {
		Name            string                 `json:"name"`
		LocalTarget     string                 `json:"local_target"`
		ExternalPort    int                    `json:"external_port"`
		AgentID         string                 `json:"agent_id"`
		Status          string                 `json:"status"`
		IsPinned        *bool                  `json:"is_pinned"`
		StartupArgs     string                 `json:"startup_args"`
		EnvVars         map[string]interface{} `json:"env_vars"`
		CPUCores        string                 `json:"cpu_cores"`
		MemoryGiB       string                 `json:"memory_gib"`
		GPUCount        int64                  `json:"gpu_count"`
		GPUModel        string                 `json:"gpu_model"`
		GPUResourceName string                 `json:"gpu_resource_name"`
		GPUNodeSelector map[string]interface{} `json:"gpu_node_selector"`
		TTL             string                 `json:"ttl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update fields
	if req.Name != "" {
		service.Name = req.Name
	}
	if req.LocalTarget != "" {
		service.LocalTarget = req.LocalTarget
	}
	if req.ExternalPort > 0 {
		service.ExternalPort = req.ExternalPort
	}
	if req.AgentID != "" {
		service.AgentID = req.AgentID
	}
	if req.Status != "" {
		service.Status = req.Status
	}
	if req.IsPinned != nil {
		service.IsPinned = *req.IsPinned
	}

	// Update configuration fields
	if req.StartupArgs != "" {
		service.StartupArgs = sql.NullString{String: req.StartupArgs, Valid: true}
	}

	// EnvVars
	if req.EnvVars != nil {
		bytes, _ := json.Marshal(req.EnvVars)
		raw := json.RawMessage(bytes)
		service.EnvVars = &raw
	}

	// Resources
	if req.CPUCores != "" {
		service.CPUCores = sql.NullString{String: req.CPUCores, Valid: true}
	}
	if req.MemoryGiB != "" {
		service.MemoryGiB = sql.NullString{String: req.MemoryGiB, Valid: true}
	}
	if req.TTL != "" {
		service.TTL = sql.NullString{String: req.TTL, Valid: true}
	}

	// GPU
	// We update GPU fields if they seem to be part of the intention
	// We rely on GPUCount >= 0
	if req.GPUCount >= 0 {
		service.GPUCount = sql.NullInt64{Int64: req.GPUCount, Valid: req.GPUCount > 0}

		if req.GPUCount > 0 {
			// Auto-lookup GPU configuration if only model is provided
			if req.GPUModel != "" && (req.GPUResourceName == "" || len(req.GPUNodeSelector) == 0) {
				if resourceConfigRepo != nil {
					config, err := resourceConfigRepo.GetConfig()
					if err == nil && config != nil && config.GPUTypes != "" {
						// Parse GPU types
						var gpuTypes []map[string]interface{}
						gpuTypesBytes := []byte(config.GPUTypes)
						_ = json.Unmarshal(gpuTypesBytes, &gpuTypes)

						// Find matching GPU type by model_name
						for _, gpuType := range gpuTypes {
							if modelName, ok := gpuType["model_name"].(string); ok && modelName == req.GPUModel {
								// Set resource_name if not provided
								if req.GPUResourceName == "" {
									if resourceName, ok := gpuType["resource_name"].(string); ok {
										req.GPUResourceName = resourceName
									}
								}
								// Set node_selector if not provided
								if len(req.GPUNodeSelector) == 0 {
									labelKey, _ := gpuType["node_label_key"].(string)
									labelValue, _ := gpuType["node_label_value"].(string)
									if labelKey != "" && labelValue != "" {
										req.GPUNodeSelector = map[string]interface{}{
											labelKey: labelValue,
										}
									}
								}
								break
							}
						}
					}
				}
			}

			if req.GPUModel != "" {
				service.GPUModel = sql.NullString{String: req.GPUModel, Valid: true}
			}
			if req.GPUResourceName != "" {
				service.GPUResourceName = sql.NullString{String: req.GPUResourceName, Valid: true}
			}
			if req.GPUNodeSelector != nil {
				bytes, _ := json.Marshal(req.GPUNodeSelector)
				raw := json.RawMessage(bytes)
				service.GPUNodeSelector = &raw
			}
		} else {
			// Clearing GPU
			service.GPUModel = sql.NullString{Valid: false}
			service.GPUResourceName = sql.NullString{Valid: false}
			service.GPUNodeSelector = nil
		}
	}

	if err := dbpkg.ServiceRepo().Update(service); err != nil {
		http.Error(w, "Failed to update service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update K8S CR
	if service.AgentID != "" && dynamicClient != nil {
		if err := updateRDEAgentSpec(context.Background(), service); err != nil {
			log.Printf("WARNING: Failed to update RDEAgent CR %s: %v", service.AgentID, err)
			// Return error to client so they know K8S update failed?
			// http.Error(w, "Service updated but K8S sync failed: "+err.Error(), http.StatusInternalServerError)
			// return
			// Actually, let's log it but return success for DB.
		} else {
			log.Printf("✓ Synced updates to RDEAgent CR %s", service.AgentID)
		}
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "update", "service", serviceID, "", ""); err != nil {
		log.Printf("Failed to log audit action: %v", err)
	}

	// Log admin access if admin is updating another user's service
	if isAdmin && adminUsername != "" {
		logAdminServiceAccess(r, userID, adminUsername, "update_service", service)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(service); err != nil {
		log.Printf("Failed to encode service: %v", err)
	}
}

// DELETE /api/services/:id - Delete a service
func handleDeleteService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract service ID
	serviceID := r.URL.Query().Get("id")
	if serviceID == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
		if len(parts) > 0 {
			serviceID = parts[0]
		}
	}

	if serviceID == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}

	// Get service
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check workspace ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	var adminUsername string
	if isAdmin {
		// Extract admin username for audit logging
		token, err := validateToken(r)
		if err != nil {
			token, _ = validateCookie(r)
		}
		if token != nil {
			var claims struct {
				PreferredUsername string `json:"preferred_username"`
			}
			if err := token.Claims(&claims); err != nil {
				log.Printf("Failed to get claims for admin audit logging: %v", err)
			}
			adminUsername = claims.PreferredUsername
		}
	}

	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	// Delete RDEAgent CR if exists
	if service.AgentID != "" && dynamicClient != nil {
		ctx := context.Background()
		err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Delete(ctx, service.AgentID, metav1.DeleteOptions{})
		if err != nil {
			// Log error but don't fail the deletion if CR doesn't exist
			if !strings.Contains(err.Error(), "not found") {
				log.Printf("WARNING: Failed to delete RDEAgent CR %s: %v", service.AgentID, err)
			} else {
				log.Printf("RDEAgent CR %s not found, skipping deletion", service.AgentID)
			}
		} else {
			log.Printf("✓ Deleted RDEAgent CR %s for service %s", service.AgentID, serviceID)
		}
	}

	if err := dbpkg.ServiceRepo().Delete(serviceID); err != nil {
		http.Error(w, "Failed to delete service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "delete", "service", serviceID, "", ""); err != nil {
		log.Printf("Failed to log audit action: %v", err)
	}

	// Log admin access if admin is deleting another user's service
	if isAdmin && adminUsername != "" {
		logAdminServiceAccess(r, userID, adminUsername, "delete_service", service)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Service deleted successfully",
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// PUT /api/services/:id/restart - Restart service by deleting pod
func handleRestartService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if k8sClientset == nil {
		http.Error(w, "Kubernetes client not available", http.StatusServiceUnavailable)
		return
	}

	// Extract service ID from path /api/services/:id/restart
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}
	serviceID := parts[0]

	// Get service
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check workspace ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	var adminUsername string
	if isAdmin {
		// Extract admin username for audit logging
		token, err := validateToken(r)
		if err != nil {
			token, _ = validateCookie(r)
		}
		if token != nil {
			var claims struct {
				PreferredUsername string `json:"preferred_username"`
			}
			if err := token.Claims(&claims); err != nil {
				log.Printf("Failed to get claims for admin audit logging: %v", err)
			}
			adminUsername = claims.PreferredUsername
		}
	}

	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	if service.AgentID == "" {
		http.Error(w, "Service has no agent ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Find deployment to get pod selector
	deployment, err := k8sClientset.AppsV1().Deployments(kuberdeNamespace).Get(ctx, service.AgentID, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	// Build label selector from deployment
	labelSelector := ""
	for k, v := range deployment.Spec.Selector.MatchLabels {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += fmt.Sprintf("%s=%s", k, v)
	}

	// List pods for this service
	pods, err := k8sClientset.CoreV1().Pods(kuberdeNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		http.Error(w, "Failed to list pods: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(pods.Items) == 0 {
		http.Error(w, "No pods found for this service", http.StatusNotFound)
		return
	}

	// Delete all pods for this service to trigger restart
	deletedPods := []string{}
	for _, pod := range pods.Items {
		err := k8sClientset.CoreV1().Pods(kuberdeNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("WARNING: Failed to delete pod %s: %v", pod.Name, err)
		} else {
			deletedPods = append(deletedPods, pod.Name)
			log.Printf("✓ Deleted pod %s for service %s restart", pod.Name, serviceID)
		}
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "restart", "service", serviceID, "", ""); err != nil {
		log.Printf("Failed to log audit entry: %v", err)
	}

	// Log admin access if admin is restarting another user's service
	if isAdmin && adminUsername != "" {
		logAdminServiceAccess(r, userID, adminUsername, "restart_service", service)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Service restart initiated",
		"deleted_pods": deletedPods,
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// PUT /api/services/:id/stop - Stop service by scaling deployment to 0
func handleStopService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if k8sClientset == nil {
		http.Error(w, "Kubernetes client not available", http.StatusServiceUnavailable)
		return
	}

	// Extract service ID from path /api/services/:id/stop
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}
	serviceID := parts[0]

	// Get service
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check workspace ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	var adminUsername string
	if isAdmin {
		// Extract admin username for audit logging
		token, err := validateToken(r)
		if err != nil {
			token, _ = validateCookie(r)
		}
		if token != nil {
			var claims struct {
				PreferredUsername string `json:"preferred_username"`
			}
			if err := token.Claims(&claims); err != nil {
				log.Printf("Failed to get claims for admin audit logging: %v", err)
			}
			adminUsername = claims.PreferredUsername
		}
	}

	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	if service.AgentID == "" {
		http.Error(w, "Service has no agent ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Get the deployment
	deployment, err := k8sClientset.AppsV1().Deployments(kuberdeNamespace).Get(ctx, service.AgentID, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	// Scale to 0
	replicas := int32(0)
	deployment.Spec.Replicas = &replicas

	_, err = k8sClientset.AppsV1().Deployments(kuberdeNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to scale deployment: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("✓ Scaled deployment %s to 0 replicas for service %s", service.AgentID, serviceID)

	// Update service status to stopped
	service.Status = "stopped"
	if err := dbpkg.ServiceRepo().Update(service); err != nil {
		log.Printf("WARNING: Failed to update service status: %v", err)
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "stop", "service", serviceID, "", ""); err != nil {
		log.Printf("Failed to log audit entry: %v", err)
	}

	// Log admin access if admin is stopping another user's service
	if isAdmin && adminUsername != "" {
		logAdminServiceAccess(r, userID, adminUsername, "stop_service", service)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Service stopped successfully",
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// PUT /api/services/:id/start - Start service by scaling deployment to 1
func handleStartService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if k8sClientset == nil {
		http.Error(w, "Kubernetes client not available", http.StatusServiceUnavailable)
		return
	}

	// Extract service ID from path /api/services/:id/start
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}
	serviceID := parts[0]

	// Get service
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check workspace ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	var adminUsername string
	if isAdmin {
		// Extract admin username for audit logging
		token, err := validateToken(r)
		if err != nil {
			token, _ = validateCookie(r)
		}
		if token != nil {
			var claims struct {
				PreferredUsername string `json:"preferred_username"`
			}
			if err := token.Claims(&claims); err != nil {
				log.Printf("Failed to get claims for admin audit logging: %v", err)
			}
			adminUsername = claims.PreferredUsername
		}
	}

	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	if service.AgentID == "" {
		http.Error(w, "Service has no agent ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Get the deployment
	deployment, err := k8sClientset.AppsV1().Deployments(kuberdeNamespace).Get(ctx, service.AgentID, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	// Scale to 1
	replicas := int32(1)
	deployment.Spec.Replicas = &replicas

	_, err = k8sClientset.AppsV1().Deployments(kuberdeNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to scale deployment: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("✓ Scaled deployment %s to 1 replica for service %s", service.AgentID, serviceID)

	// Update service status to running
	service.Status = "running"
	if err := dbpkg.ServiceRepo().Update(service); err != nil {
		log.Printf("WARNING: Failed to update service status: %v", err)
	}

	// Log audit entry
	auditRepo := dbpkg.AuditLogRepo()
	if err := auditRepo.LogAction(userID, "start", "service", serviceID, "", ""); err != nil {
		log.Printf("Failed to log audit action: %v", err)
	}

	// Log admin access if admin is starting another user's service
	if isAdmin && adminUsername != "" {
		logAdminServiceAccess(r, userID, adminUsername, "start_service", service)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Service started successfully",
	})
}

// GET /api/services/:id/logs - Get service pod logs
func handleGetServiceLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if k8sClientset == nil {
		http.Error(w, "Kubernetes client not available", http.StatusServiceUnavailable)
		return
	}

	// Extract service ID from path /api/services/:id/logs
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}
	serviceID := parts[0]

	// Get service
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check workspace ownership
	owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
	if err != nil {
		http.Error(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if !owns {
		http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
		return
	}

	if service.AgentID == "" {
		http.Error(w, "Service has no agent ID", http.StatusBadRequest)
		return
	}

	// Get container parameter (default: workload)
	container := r.URL.Query().Get("container")
	if container == "" {
		container = "workload"
	}

	// Validate container name
	if container != "kuberde-agent" && container != "workload" {
		http.Error(w, "Invalid container name. Must be 'kuberde-agent' or 'workload'", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Find deployment to get pod selector
	deployment, err := k8sClientset.AppsV1().Deployments(kuberdeNamespace).Get(ctx, service.AgentID, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	// Build label selector from deployment
	labelSelector := ""
	for k, v := range deployment.Spec.Selector.MatchLabels {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += fmt.Sprintf("%s=%s", k, v)
	}

	// List pods for this service
	pods, err := k8sClientset.CoreV1().Pods(kuberdeNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		http.Error(w, "Failed to list pods: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(pods.Items) == 0 {
		http.Error(w, "No pods found for this service (service may be stopped)", http.StatusNotFound)
		return
	}

	podName := pods.Items[0].Name
	tail := int64(500) // Get last 500 lines

	// Get logs from specified container
	req := k8sClientset.CoreV1().Pods(kuberdeNamespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tail,
		Container: container,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get logs from container '%s': %v", container, err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = stream.Close() }()

	// Read logs from stream
	logs, err := io.ReadAll(stream)
	if err != nil {
		http.Error(w, "Failed to read logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"container": container,
		"pod_name":  podName,
		"logs":      string(logs),
	})
}

// GET /api/services/:id - Get service by ID
func handleGetService(w http.ResponseWriter, r *http.Request, serviceID string) {
	userID, err := extractUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if serviceID == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}

	// Get service with template information
	service, err := dbpkg.ServiceRepo().FindByID(serviceID)
	if err != nil {
		http.Error(w, "Failed to get service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Load template if available
	if service.TemplateID.Valid {
		template, err := dbpkg.AgentTemplateRepo().GetByID(context.TODO(), service.TemplateID.String)
		if err == nil && template != nil {
			service.Template = template
		}
	}

	// Check workspace ownership (admins can access any workspace)
	isAdmin := userHasAdminRole(r)
	var adminUsername string
	if isAdmin {
		// Extract admin username for audit logging
		token, err := validateToken(r)
		if err != nil {
			token, _ = validateCookie(r)
		}
		if token != nil {
			var claims struct {
				PreferredUsername string `json:"preferred_username"`
			}
			if err := token.Claims(&claims); err != nil {
				log.Printf("Failed to get claims for admin audit logging: %v", err)
			}
			adminUsername = claims.PreferredUsername
		}
	}

	if !isAdmin {
		owns, err := userOwnsWorkspace(userID, service.WorkspaceID)
		if err != nil {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		if !owns {
			http.Error(w, "Forbidden: You do not own this workspace", http.StatusForbidden)
			return
		}
	}

	// Update service status from CR status (real-time)
	agentID := service.AgentID
	log.Printf("[handleGetService] Service ID: %s, AgentID: %s", service.ID, agentID)

	// If AgentID is empty but service has a template, try to derive and update it
	if agentID == "" && service.TemplateID.Valid && service.WorkspaceID != "" {
		// Get user and workspace information for naming
		user, err := dbpkg.UserRepo().FindByID(service.CreatedByID)
		workspace, wsErr := dbpkg.WorkspaceRepo().FindByID(service.WorkspaceID)

		if err == nil && user != nil && wsErr == nil && workspace != nil {
			// Try new naming convention first: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash}
			derivedAgentID := generateAgentName(service.CreatedByID, user.Username, service.WorkspaceID, workspace.Name, service.Name)
			log.Printf("[handleGetService] AgentID empty, trying new naming convention: %s", derivedAgentID)

			// Check if CR exists with this name
			if dynamicClient != nil {
				_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), derivedAgentID, metav1.GetOptions{})
				if err == nil {
					// CR exists! Update the service record
					service.AgentID = derivedAgentID
					if updateErr := dbpkg.ServiceRepo().Update(service); updateErr != nil {
						log.Printf("[handleGetService] WARNING: Failed to update AgentID: %v", updateErr)
					} else {
						log.Printf("[handleGetService] ✓ Updated service %s with derived AgentID: %s", service.ID, derivedAgentID)
						agentID = derivedAgentID
					}
				} else {
					// Try old naming convention as fallback: kuberde-{userID}-{workspaceID}-{serviceName}
					oldAgentID := fmt.Sprintf("kuberde-%s-%s-%s", service.CreatedByID, service.WorkspaceID, service.Name)
					log.Printf("[handleGetService] New convention failed, trying old convention: %s", oldAgentID)
					_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), oldAgentID, metav1.GetOptions{})
					if err == nil {
						service.AgentID = oldAgentID
						if updateErr := dbpkg.ServiceRepo().Update(service); updateErr != nil {
							log.Printf("[handleGetService] WARNING: Failed to update AgentID: %v", updateErr)
						} else {
							log.Printf("[handleGetService] ✓ Updated service %s with old-style AgentID: %s", service.ID, oldAgentID)
							agentID = oldAgentID
						}
					} else {
						log.Printf("[handleGetService] CR not found with either naming convention")
					}
				}
			}
		} else {
			log.Printf("[handleGetService] Failed to get user or workspace info for naming: user=%v, workspace=%v", user != nil, workspace != nil)
		}
	}

	if agentID != "" && dynamicClient != nil {
		// Try to get CR status from Kubernetes
		cr, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), agentID, metav1.GetOptions{})
		if err == nil {
			log.Printf("[handleGetService] CR found for AgentID: %s", agentID)
			// Extract status from CR
			if status, found, err := unstructured.NestedMap(cr.Object, "status"); found && err == nil {
				log.Printf("[handleGetService] CR status: %+v", status)
				// Get phase from CR status
				if phase, ok := status["phase"].(string); ok {
					log.Printf("[handleGetService] CR phase: %s", phase)
					switch phase {
					case "Running":
						service.Status = "running"
					case "Disconnected", "ScaledDown":
						service.Status = "stopped"
					case "Starting", "Pending":
						service.Status = "starting"
					case "Error":
						service.Status = "error"
					default:
						service.Status = "unknown"
					}
				} else {
					log.Printf("[handleGetService] Phase field not found or not string in status")
				}
			} else {
				log.Printf("[handleGetService] Status field not found in CR, found=%v, err=%v", found, err)
			}
		} else {
			log.Printf("[handleGetService] Failed to get CR for AgentID %s: %v", agentID, err)
			// Fallback to agentStats if CR not found
			statsMu.RLock()
			stats, exists := agentStats[agentID]
			statsMu.RUnlock()

			if exists && stats.Online {
				service.Status = "running"
			} else {
				service.Status = "stopped"
			}
		}
	} else {
		log.Printf("[handleGetService] AgentID empty or dynamicClient nil: agentID=%s, dynamicClient=%v", agentID, dynamicClient != nil)
		service.Status = "unknown"
	}

	// Log admin access if admin is accessing another user's service
	if isAdmin && adminUsername != "" {
		logAdminServiceAccess(r, userID, adminUsername, "view_service", service)
	}

	// Convert service to include remote_proxy field
	svcBytes, _ := json.Marshal(service)
	var svcMap map[string]interface{}
	_ = json.Unmarshal(svcBytes, &svcMap)

	// Add remote_proxy field
	if service.AgentID != "" {
		svcMap["remote_proxy"] = fmt.Sprintf("%s.%s", service.AgentID, agentDomain)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(svcMap)
}

// GET /api/agent-templates - List all agent templates
func handleGetAgentTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete && r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Handle DELETE /api/agent-templates/{id}
	if r.Method == http.MethodDelete {
		handleDeleteAgentTemplate(w, r)
		return
	}

	// Handle PUT /api/agent-templates/{id}
	if r.Method == http.MethodPut {
		handleUpdateAgentTemplate(w, r)
		return
	}

	// Handle POST /api/agent-templates
	if r.Method == http.MethodPost {
		handleCreateAgentTemplate(w, r)
		return
	}

	ctx := r.Context()
	templates, err := dbpkg.AgentTemplateRepo().GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to get templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if templates == nil {
		templates = []models.AgentTemplate{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(templates)
}

// POST /api/agent-templates - Create a new agent template
func handleCreateAgentTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                string                 `json:"name"`
		AgentType           string                 `json:"agent_type"`
		Description         string                 `json:"description"`
		DockerImage         string                 `json:"docker_image"`
		DefaultLocalTarget  string                 `json:"default_local_target"`
		DefaultExternalPort int                    `json:"default_external_port"`
		StartupArgs         string                 `json:"startup_args"`
		EnvVars             map[string]interface{} `json:"env_vars"`
		SecurityContext     map[string]interface{} `json:"security_context"`
		VolumeMounts        []interface{}          `json:"volume_mounts"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.AgentType == "" || req.DockerImage == "" {
		http.Error(w, "name, agent_type, and docker_image are required", http.StatusBadRequest)
		return
	}

	template := &models.AgentTemplate{
		ID:                  uuid.New().String(),
		Name:                req.Name,
		AgentType:           req.AgentType,
		Description:         req.Description,
		DockerImage:         req.DockerImage,
		DefaultLocalTarget:  req.DefaultLocalTarget,
		DefaultExternalPort: req.DefaultExternalPort,
		StartupArgs:         req.StartupArgs,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if req.EnvVars != nil {
		bytes, _ := json.Marshal(req.EnvVars)
		raw := json.RawMessage(bytes)
		template.EnvVars = &raw
	}
	if req.SecurityContext != nil {
		bytes, _ := json.Marshal(req.SecurityContext)
		raw := json.RawMessage(bytes)
		template.SecurityContext = &raw
	}
	if req.VolumeMounts != nil {
		bytes, _ := json.Marshal(req.VolumeMounts)
		raw := json.RawMessage(bytes)
		template.VolumeMounts = &raw
	}

	ctx := r.Context()
	if err := dbpkg.AgentTemplateRepo().Create(ctx, template); err != nil {
		http.Error(w, "Failed to create template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(template)
}

// PUT /api/agent-templates/{id} - Update an agent template
func handleUpdateAgentTemplate(w http.ResponseWriter, r *http.Request) {
	// Extract template ID from URL path
	templateID := strings.TrimPrefix(r.URL.Path, "/api/agent-templates/")
	if templateID == "" {
		http.Error(w, "Template ID is required", http.StatusBadRequest)
		return
	}

	// Protect built-in templates from modification (except for admins)
	if strings.HasPrefix(templateID, "tpl-") && !hasRole(r, "admin") {
		http.Error(w, "Cannot modify built-in templates", http.StatusForbidden)
		return
	}

	ctx := r.Context()

	// Get existing template
	existingTemplate, err := dbpkg.AgentTemplateRepo().GetByID(ctx, templateID)
	if err != nil {
		http.Error(w, "Template not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Parse request body
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update fields
	if name, ok := updates["name"].(string); ok && name != "" {
		existingTemplate.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		existingTemplate.Description = desc
	}
	if dockerImage, ok := updates["docker_image"].(string); ok && dockerImage != "" {
		existingTemplate.DockerImage = dockerImage
	}
	if localTarget, ok := updates["default_local_target"].(string); ok && localTarget != "" {
		existingTemplate.DefaultLocalTarget = localTarget
	}
	if port, ok := updates["default_external_port"].(float64); ok && port > 0 {
		existingTemplate.DefaultExternalPort = int(port)
	}
	if startupArgs, ok := updates["startup_args"].(string); ok {
		existingTemplate.StartupArgs = startupArgs
	}
	if envVars, ok := updates["env_vars"]; ok {
		bytes, err := json.Marshal(envVars)
		if err == nil {
			raw := json.RawMessage(bytes)
			existingTemplate.EnvVars = &raw
		}
	}
	if securityContext, ok := updates["security_context"]; ok {
		bytes, err := json.Marshal(securityContext)
		if err == nil {
			raw := json.RawMessage(bytes)
			existingTemplate.SecurityContext = &raw
		}
	}
	if volumeMounts, ok := updates["volume_mounts"]; ok {
		bytes, err := json.Marshal(volumeMounts)
		if err == nil {
			raw := json.RawMessage(bytes)
			existingTemplate.VolumeMounts = &raw
		}
	}

	// Update in database
	if err := dbpkg.AgentTemplateRepo().Update(ctx, existingTemplate); err != nil {
		http.Error(w, "Failed to update template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(existingTemplate)
}

// DELETE /api/agent-templates/{id} - Delete an agent template
func handleDeleteAgentTemplate(w http.ResponseWriter, r *http.Request) {
	// Extract template ID from URL path
	templateID := strings.TrimPrefix(r.URL.Path, "/api/agent-templates/")
	if templateID == "" {
		http.Error(w, "Template ID is required", http.StatusBadRequest)
		return
	}

	// Protect built-in templates from deletion
	if strings.HasPrefix(templateID, "tpl-") {
		http.Error(w, "Cannot delete built-in templates", http.StatusForbidden)
		return
	}

	ctx := r.Context()
	err := dbpkg.AgentTemplateRepo().Delete(ctx, templateID)
	if err != nil {
		http.Error(w, "Failed to delete template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Template deleted successfully",
	})
}

// GET /api/agent-templates/export - Export all templates as JSON
func handleExportAllTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	templates, err := dbpkg.AgentTemplateRepo().GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to get templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create export format (exclude ID, CreatedAt, UpdatedAt for clean import)
	type ExportTemplate struct {
		Name                string           `json:"name"`
		AgentType           string           `json:"agent_type"`
		Description         string           `json:"description"`
		DockerImage         string           `json:"docker_image"`
		DefaultLocalTarget  string           `json:"default_local_target"`
		DefaultExternalPort int              `json:"default_external_port"`
		StartupArgs         string           `json:"startup_args,omitempty"`
		EnvVars             *json.RawMessage `json:"env_vars,omitempty"`
		SecurityContext     *json.RawMessage `json:"security_context,omitempty"`
		VolumeMounts        *json.RawMessage `json:"volume_mounts,omitempty"`
	}

	exportData := make([]ExportTemplate, 0, len(templates))
	for _, t := range templates {
		exportData = append(exportData, ExportTemplate{
			Name:                t.Name,
			AgentType:           t.AgentType,
			Description:         t.Description,
			DockerImage:         t.DockerImage,
			DefaultLocalTarget:  t.DefaultLocalTarget,
			DefaultExternalPort: t.DefaultExternalPort,
			StartupArgs:         t.StartupArgs,
			EnvVars:             t.EnvVars,
			SecurityContext:     t.SecurityContext,
			VolumeMounts:        t.VolumeMounts,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"agent-templates-export.json\"")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now().Format(time.RFC3339),
		"templates":   exportData,
	})
}

// GET /api/agent-templates/{id}/export - Export single template as JSON
func handleExportSingleTemplate(w http.ResponseWriter, r *http.Request) {
	// Extract template ID from URL path
	templateID := strings.TrimPrefix(r.URL.Path, "/api/agent-templates/")
	templateID = strings.TrimSuffix(templateID, "/export")
	if templateID == "" {
		http.Error(w, "Template ID is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	template, err := dbpkg.AgentTemplateRepo().GetByID(ctx, templateID)
	if err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Create export format
	type ExportTemplate struct {
		Name                string           `json:"name"`
		AgentType           string           `json:"agent_type"`
		Description         string           `json:"description"`
		DockerImage         string           `json:"docker_image"`
		DefaultLocalTarget  string           `json:"default_local_target"`
		DefaultExternalPort int              `json:"default_external_port"`
		StartupArgs         string           `json:"startup_args,omitempty"`
		EnvVars             *json.RawMessage `json:"env_vars,omitempty"`
		SecurityContext     *json.RawMessage `json:"security_context,omitempty"`
		VolumeMounts        *json.RawMessage `json:"volume_mounts,omitempty"`
	}

	exportData := ExportTemplate{
		Name:                template.Name,
		AgentType:           template.AgentType,
		Description:         template.Description,
		DockerImage:         template.DockerImage,
		DefaultLocalTarget:  template.DefaultLocalTarget,
		DefaultExternalPort: template.DefaultExternalPort,
		StartupArgs:         template.StartupArgs,
		EnvVars:             template.EnvVars,
		SecurityContext:     template.SecurityContext,
		VolumeMounts:        template.VolumeMounts,
	}

	filename := fmt.Sprintf("agent-template-%s.json", template.AgentType)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now().Format(time.RFC3339),
		"template":    exportData,
	})
}

// POST /api/agent-templates/import - Import templates from JSON
func handleImportTemplates(w http.ResponseWriter, r *http.Request) {
	var importData struct {
		Version   string `json:"version"`
		Templates []struct {
			Name                string           `json:"name"`
			AgentType           string           `json:"agent_type"`
			Description         string           `json:"description"`
			DockerImage         string           `json:"docker_image"`
			DefaultLocalTarget  string           `json:"default_local_target"`
			DefaultExternalPort int              `json:"default_external_port"`
			StartupArgs         string           `json:"startup_args"`
			EnvVars             *json.RawMessage `json:"env_vars"`
			SecurityContext     *json.RawMessage `json:"security_context"`
			VolumeMounts        *json.RawMessage `json:"volume_mounts"`
		} `json:"templates"`
		Template *struct {
			Name                string           `json:"name"`
			AgentType           string           `json:"agent_type"`
			Description         string           `json:"description"`
			DockerImage         string           `json:"docker_image"`
			DefaultLocalTarget  string           `json:"default_local_target"`
			DefaultExternalPort int              `json:"default_external_port"`
			StartupArgs         string           `json:"startup_args"`
			EnvVars             *json.RawMessage `json:"env_vars"`
			SecurityContext     *json.RawMessage `json:"security_context"`
			VolumeMounts        *json.RawMessage `json:"volume_mounts"`
		} `json:"template"`
	}

	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	successCount := 0
	failedCount := 0
	errors := []string{}

	// Handle both single template and multiple templates
	templatesToImport := importData.Templates
	if importData.Template != nil {
		templatesToImport = append(templatesToImport, *importData.Template)
	}

	for _, tpl := range templatesToImport {
		// Validate required fields
		if tpl.Name == "" || tpl.AgentType == "" || tpl.DockerImage == "" {
			failedCount++
			errors = append(errors, fmt.Sprintf("Template '%s': missing required fields", tpl.Name))
			continue
		}

		// Check if template with same agent_type already exists
		existing, _ := dbpkg.AgentTemplateRepo().GetByAgentType(ctx, tpl.AgentType)
		if existing != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Template with agent_type '%s' already exists", tpl.AgentType))
			continue
		}

		// Create new template
		newTemplate := &models.AgentTemplate{
			ID:                  uuid.New().String(),
			Name:                tpl.Name,
			AgentType:           tpl.AgentType,
			Description:         tpl.Description,
			DockerImage:         tpl.DockerImage,
			DefaultLocalTarget:  tpl.DefaultLocalTarget,
			DefaultExternalPort: tpl.DefaultExternalPort,
			StartupArgs:         tpl.StartupArgs,
			EnvVars:             tpl.EnvVars,
			SecurityContext:     tpl.SecurityContext,
			VolumeMounts:        tpl.VolumeMounts,
		}

		// Save to database
		if err := dbpkg.AgentTemplateRepo().Create(ctx, newTemplate); err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Failed to create template '%s': %v", tpl.Name, err))
			continue
		}

		successCount++
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success_count": successCount,
		"failed_count":  failedCount,
		"errors":        errors,
		"message":       fmt.Sprintf("Imported %d templates, %d failed", successCount, failedCount),
	})
}

// createWorkspacePVC creates a PersistentVolumeClaim for a workspace
func createWorkspacePVC(ctx context.Context, userName string, workspace *models.Workspace) error {
	if k8sClientset == nil {
		return fmt.Errorf("kubernetes client not available")
	}

	// Use the PVCName already set on the workspace
	pvcName := workspace.PVCName
	if pvcName == "" {
		return fmt.Errorf("PVCName not set on workspace")
	}

	// Parse storage size (e.g., "50Gi" -> "50Gi")
	storageSize := workspace.StorageSize
	if storageSize == "" {
		storageSize = "50Gi"
	}

	// Get StorageClass from workspace
	storageClass := workspace.StorageClass
	if storageClass == "" {
		storageClass = "standard"
	}

	// Determine access mode based on storage class
	// local-path only supports ReadWriteOnce, other storage classes use ReadWriteMany
	var accessMode corev1.PersistentVolumeAccessMode
	if storageClass == "local-path" {
		accessMode = corev1.ReadWriteOnce
		log.Printf("Using ReadWriteOnce access mode for local-path storage class")
	} else {
		accessMode = corev1.ReadWriteMany
		log.Printf("Using ReadWriteMany access mode for storage class: %s", storageClass)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: kuberdeNamespace,
			Labels: map[string]string{
				"app":       "kuberde",
				"type":      "workspace",
				"user":      userName,
				"workspace": workspace.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessMode, // Dynamic based on storage class
			},
			StorageClassName: &storageClass,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mustParse(storageSize),
				},
			},
		},
	}

	pvcClient := k8sClientset.CoreV1().PersistentVolumeClaims(kuberdeNamespace)

	_, err := pvcClient.Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create PVC %s: %v", pvcName, err)
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	log.Printf("✓ Created PVC %s in namespace %s", pvcName, kuberdeNamespace)
	return nil
}

// mustParse parses a Kubernetes resource quantity string
func mustParse(quantity string) resource.Quantity {
	q, err := resource.ParseQuantity(quantity)
	if err != nil {
		log.Printf("WARNING: Failed to parse quantity %s: %v, using default 50Gi", quantity, err)
		q, _ = resource.ParseQuantity("50Gi")
	}
	return q
}

// createRDEAgentFromTemplate creates an RDEAgent CR from an agent template
// Returns the generated agent ID (CR name)
func createRDEAgentFromTemplate(ctx context.Context, service *models.Service, template *models.AgentTemplate, user *models.User, userID, workspaceID string, workspace *models.Workspace) (string, error) {
	if dynamicClient == nil {
		// Kubernetes client not available, skip CR creation
		log.Printf("WARNING: Kubernetes client not available, skipping RDEAgent CR creation for service %s", service.ID)
		return "", nil
	}

	// Generate CR name using new naming convention with hash
	// Format: kuberde-agent-{userName}-{workspaceName}-{serviceName}-{hash8}
	crName := generateAgentName(userID, user.Username, workspaceID, workspace.Name, service.Name)

	// Parse template configuration
	var templateEnvVars map[string]interface{}
	if template.EnvVars != nil {
		if err := json.Unmarshal(*template.EnvVars, &templateEnvVars); err != nil {
			log.Printf("WARNING: Failed to parse template env_vars: %v", err)
			templateEnvVars = make(map[string]interface{})
		}
	} else {
		templateEnvVars = make(map[string]interface{})
	}

	// Merge with user customization
	if service.EnvVars != nil {
		var userEnvVars map[string]interface{}
		if err := json.Unmarshal(*service.EnvVars, &userEnvVars); err == nil {
			for k, v := range userEnvVars {
				templateEnvVars[k] = v // User customization overrides template
			}
		}
	}

	// Build environment variables list
	// variable envList removed as it is unused

	// Parse startup args
	var args []string
	if template.StartupArgs != "" {
		args = strings.Fields(template.StartupArgs)
	}

	// If user provided startup_args, use those instead
	if service.StartupArgs.Valid && service.StartupArgs.String != "" {
		args = strings.Fields(service.StartupArgs.String)
	}

	// Extract port from local target (e.g., "127.0.0.1:22" -> 22)
	containerPort := 8080 // Default
	if parts := strings.Split(service.LocalTarget, ":"); len(parts) == 2 {
		if port, err := strconv.Atoi(parts[1]); err == nil {
			containerPort = port
		}
	}

	// Build environment variables list for workloadContainer spec
	workloadEnv := []map[string]interface{}{}
	for k, v := range templateEnvVars {
		workloadEnv = append(workloadEnv, map[string]interface{}{
			"name":  k,
			"value": fmt.Sprintf("%v", v),
		})
	}

	// Build resource requests and limits from service configuration
	resources := map[string]interface{}{}
	if service.CPUCores.Valid || service.MemoryGiB.Valid || service.GPUCount.Valid {
		requests := make(map[string]interface{})
		limits := make(map[string]interface{})

		// CPU configuration
		// Request: always 100m, Limit: UI value
		requests["cpu"] = "100m"
		if service.CPUCores.Valid && service.CPUCores.String != "" {
			limits["cpu"] = service.CPUCores.String
		} else {
			// Default limit
			limits["cpu"] = "500m"
		}

		// Memory configuration
		// Request: always 128Mi, Limit: UI value in Gi
		requests["memory"] = "128Mi"
		if service.MemoryGiB.Valid && service.MemoryGiB.String != "" {
			limits["memory"] = service.MemoryGiB.String + "Gi"
		} else {
			// Default limit
			limits["memory"] = "512Mi"
		}

		// GPU configuration
		// Exception: both request and limit use UI value
		if service.GPUCount.Valid && service.GPUCount.Int64 > 0 {
			gpuCount := fmt.Sprintf("%d", service.GPUCount.Int64)
			// Use dynamic GPU resource name from service configuration
			gpuResourceName := "nvidia.com/gpu" // default fallback
			if service.GPUResourceName.Valid && service.GPUResourceName.String != "" {
				gpuResourceName = service.GPUResourceName.String
			}
			requests[gpuResourceName] = gpuCount
			limits[gpuResourceName] = gpuCount
		}

		resources["requests"] = requests
		resources["limits"] = limits
	} else {
		// Default resources if nothing specified
		resources = map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "100m",
				"memory": "128Mi",
			},
			"limits": map[string]interface{}{
				"cpu":    "500m",
				"memory": "512Mi",
			},
		}
	}

	// Parse security context from template
	var securityContext map[string]interface{}
	if template.SecurityContext != nil {
		if err := json.Unmarshal(*template.SecurityContext, &securityContext); err != nil {
			log.Printf("WARNING: Failed to parse template security_context: %v", err)
		}
	}

	// Parse volume mounts from template
	var volumeMounts []map[string]interface{}
	if template.VolumeMounts != nil {
		if err := json.Unmarshal(*template.VolumeMounts, &volumeMounts); err != nil {
			log.Printf("WARNING: Failed to parse template volume_mounts: %v", err)
			// Fallback to default
			volumeMounts = []map[string]interface{}{
				{
					"name":      "workspace",
					"mountPath": "/root",
					"readOnly":  false,
				},
			}
		}
	} else {
		// Default volume mount if template doesn't specify
		volumeMounts = []map[string]interface{}{
			{
				"name":      "workspace",
				"mountPath": "/root",
				"readOnly":  false,
			},
		}
	}

	// Extract SSH public keys if this is an SSH service
	var sshPublicKeys []string
	if template.AgentType == "ssh" && user.SSHKeys != nil {
		var sshKeys []models.SSHKey
		if err := json.Unmarshal(*user.SSHKeys, &sshKeys); err == nil {
			for _, key := range sshKeys {
				if key.PublicKey != "" {
					sshPublicKeys = append(sshPublicKeys, key.PublicKey)
				}
			}
		}
	}

	// Build workloadContainer spec
	workloadContainer := map[string]interface{}{
		"image":           template.DockerImage,
		"imagePullPolicy": "IfNotPresent",
		"args":            args,
		"env":             workloadEnv,
		"ports": []map[string]interface{}{
			{
				"containerPort": containerPort,
				"name":          "service",
				"protocol":      "TCP",
			},
		},
		"volumeMounts": volumeMounts, // Use volumeMounts from template
		"resources":    resources,
	}

	// Add securityContext if present in template
	if len(securityContext) > 0 {
		workloadContainer["securityContext"] = securityContext
		log.Printf("Added securityContext to CR %s: %v", crName, securityContext)
	}

	// Determine TTL value
	ttl := "24h" // Default TTL
	if service.TTL.Valid && service.TTL.String != "" {
		ttl = service.TTL.String
	}

	// Build spec map
	spec := map[string]interface{}{
		"owner":             userID,
		"serverUrl":         agentServerURL,
		"authSecret":        agentAuthSecret,
		"localTarget":       fmt.Sprintf("localhost:%d", containerPort),
		"ttl":               ttl, // Use TTL from service, default to 24h
		"workloadContainer": workloadContainer,
		// Declare volumes to share workspace PVC across services
		"volumes": []map[string]interface{}{
			{
				"name": "workspace",
				"persistentVolumeClaim": map[string]interface{}{
					"claimName": workspace.PVCName,
				},
			},
		},
		// Use existing workspace PVC instead of creating new one
		"pvcName": workspace.PVCName,
	}

	// Add SSH public keys if present
	if len(sshPublicKeys) > 0 {
		spec["sshPublicKeys"] = sshPublicKeys
		log.Printf("Added %d SSH public keys to CR %s", len(sshPublicKeys), crName)
	}

	// Add GPU node selector if present
	if service.GPUNodeSelector != nil {
		var nodeSelector map[string]interface{}
		if err := json.Unmarshal(*service.GPUNodeSelector, &nodeSelector); err == nil && len(nodeSelector) > 0 {
			spec["nodeSelector"] = nodeSelector
			log.Printf("Added GPU nodeSelector to CR %s: %v", crName, nodeSelector)
		}
	}

	// Build RDEAgent CR following Operator's expected format
	agent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kuberde.io/v1beta1",
			"kind":       "RDEAgent",
			"metadata": map[string]interface{}{
				"name":      crName,
				"namespace": kuberdeNamespace,
				"labels": map[string]interface{}{
					"app":       "kuberde",
					"type":      "agent",
					"user":      userID,
					"workspace": workspaceID,
					"service":   service.Name,
				},
			},
			"spec": spec,
		},
	}

	// Create the RDEAgent CR
	_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Create(ctx, agent, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create RDEAgent CR %s: %v", crName, err)
		return "", fmt.Errorf("failed to create RDEAgent CR: %w", err)
	}

	log.Printf("✓ Created RDEAgent CR %s from template %s", crName, template.AgentType)
	return crName, nil
}

func updateRDEAgentSpec(ctx context.Context, service *models.Service) error {
	if dynamicClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	if service.AgentID == "" {
		return fmt.Errorf("service has no AgentID")
	}

	// Get existing CR
	agentCR, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(ctx, service.AgentID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get RDEAgent CR: %w", err)
	}

	// Update spec fields
	spec, ok := agentCR.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid CR spec format")
	}

	// Update TTL
	if service.TTL.Valid {
		spec["ttl"] = service.TTL.String
	}

	// Update workloadContainer
	workloadContainer, ok := spec["workloadContainer"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid workloadContainer format")
	}

	// Update Args
	if service.StartupArgs.Valid {
		workloadContainer["args"] = strings.Fields(service.StartupArgs.String)
	}

	// Fetch template if available to merge env vars
	var templateEnvVars map[string]interface{}
	if service.TemplateID.Valid {
		template, err := dbpkg.AgentTemplateRepo().GetByID(ctx, service.TemplateID.String)
		if err == nil && template != nil && template.EnvVars != nil {
			if err := json.Unmarshal(*template.EnvVars, &templateEnvVars); err != nil {
				log.Printf("WARNING: Failed to parse template env_vars: %v", err)
			}
		}
	}
	if templateEnvVars == nil {
		templateEnvVars = make(map[string]interface{})
	}

	// Merge service env vars (overrides template)
	if service.EnvVars != nil {
		var userEnvVars map[string]interface{}
		if err := json.Unmarshal(*service.EnvVars, &userEnvVars); err == nil {
			for k, v := range userEnvVars {
				templateEnvVars[k] = v
			}
		}
	}

	// Rebuild env list
	var envList []map[string]interface{}
	for k, v := range templateEnvVars {
		envList = append(envList, map[string]interface{}{
			"name":  k,
			"value": fmt.Sprintf("%v", v),
		})
	}
	workloadContainer["env"] = envList

	// Update Resources
	resources := map[string]interface{}{}
	if service.CPUCores.Valid || service.MemoryGiB.Valid || service.GPUCount.Valid {
		requests := make(map[string]interface{})
		limits := make(map[string]interface{})

		// CPU
		requests["cpu"] = "100m"
		if service.CPUCores.Valid && service.CPUCores.String != "" {
			limits["cpu"] = service.CPUCores.String
		} else {
			limits["cpu"] = "500m"
		}

		// Memory
		requests["memory"] = "128Mi"
		if service.MemoryGiB.Valid && service.MemoryGiB.String != "" {
			limits["memory"] = service.MemoryGiB.String + "Gi"
		} else {
			limits["memory"] = "512Mi"
		}

		// GPU
		if service.GPUCount.Valid && service.GPUCount.Int64 > 0 {
			gpuCount := fmt.Sprintf("%d", service.GPUCount.Int64)
			gpuResourceName := "nvidia.com/gpu"
			if service.GPUResourceName.Valid && service.GPUResourceName.String != "" {
				gpuResourceName = service.GPUResourceName.String
			}
			requests[gpuResourceName] = gpuCount
			limits[gpuResourceName] = gpuCount
		}

		resources["requests"] = requests
		resources["limits"] = limits
		workloadContainer["resources"] = resources
	}

	spec["workloadContainer"] = workloadContainer

	// Update GPU Node Selector
	if service.GPUNodeSelector != nil {
		var nodeSelector map[string]interface{}
		if err := json.Unmarshal(*service.GPUNodeSelector, &nodeSelector); err == nil && len(nodeSelector) > 0 {
			spec["nodeSelector"] = nodeSelector
		} else {
			delete(spec, "nodeSelector")
		}
	} else if service.GPUCount.Valid && service.GPUCount.Int64 == 0 {
		// If GPUNodeSelector is nil and GPU is disabled (GPUCount=0), remove nodeSelector
		delete(spec, "nodeSelector")
	}

	// Apply updates
	agentCR.Object["spec"] = spec
	_, err = dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Update(ctx, agentCR, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update K8S resource: %w", err)
	}

	log.Printf("✓ Updated RDEAgent CR %s", service.AgentID)
	return nil
}

// handleDownloadCLI serves kuberde-cli binaries for different platforms
// GET /download/cli/{platform}
// Supported platforms: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64, windows-arm64
func handleDownloadCLI(w http.ResponseWriter, r *http.Request) {
	// Extract platform from URL path
	platform := strings.TrimPrefix(r.URL.Path, "/download/cli/")
	if platform == "" {
		http.Error(w, "Platform not specified", http.StatusBadRequest)
		return
	}

	// Map platform to binary file name
	var fileName string
	switch platform {
	case "linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64":
		fileName = fmt.Sprintf("kuberde-cli-%s", platform)
	case "windows-amd64", "windows-arm64":
		fileName = fmt.Sprintf("kuberde-cli-%s.exe", platform)
	default:
		http.Error(w, "Unsupported platform", http.StatusBadRequest)
		return
	}

	// Read binary file
	filePath := filepath.Join("cli-binaries", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to read CLI binary %s: %v", filePath, err)
		http.Error(w, "Binary not found", http.StatusNotFound)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write binary data
	_, err = w.Write(data)
	if err != nil {
		log.Printf("ERROR: Failed to write CLI binary response: %v", err)
	}

	log.Printf("✓ Served CLI binary %s for platform %s", fileName, platform)
}

// GET /api/admin/workspaces - List all workspaces (admin only)
func handleAdminListWorkspaces(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params
	limit := 10
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	workspaces, err := dbpkg.WorkspaceRepo().GetAll(limit, offset)
	if err != nil {
		http.Error(w, "Failed to list workspaces: "+err.Error(), http.StatusInternalServerError)
		return
	}

	total, err := dbpkg.WorkspaceRepo().Count()
	if err != nil {
		log.Printf("Failed to count workspaces: %v", err)
	}

	// Update service status from CR status (real-time) for all workspaces
	for i := range workspaces {
		if workspaces[i].Services == nil {
			continue
		}

		var user *models.User
		if workspaces[i].Owner != nil {
			user = workspaces[i].Owner
		} else if workspaces[i].OwnerID != "" {
			user, _ = dbpkg.UserRepo().FindByID(workspaces[i].OwnerID)
		}

		for j := range workspaces[i].Services {
			service := &workspaces[i].Services[j]
			agentID := service.AgentID

			// If AgentID is empty but service has a template, try to derive it
			if agentID == "" && service.TemplateID.Valid && user != nil {
				// Try new naming convention first
				derivedAgentID := generateAgentName(service.CreatedByID, user.Username, service.WorkspaceID, workspaces[i].Name, service.Name)
				if dynamicClient != nil {
					_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), derivedAgentID, metav1.GetOptions{})
					if err == nil {
						agentID = derivedAgentID
					} else {
						// Try old naming convention as fallback
						oldAgentID := fmt.Sprintf("kuberde-%s-%s-%s", service.CreatedByID, service.WorkspaceID, service.Name)
						_, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), oldAgentID, metav1.GetOptions{})
						if err == nil {
							agentID = oldAgentID
						}
					}
				}
			}

			if agentID == "" {
				service.Status = "unknown"
				continue
			}

			// Try to get CR status from Kubernetes
			if dynamicClient != nil {
				cr, err := dynamicClient.Resource(frpAgentGVR).Namespace(kuberdeNamespace).Get(context.TODO(), agentID, metav1.GetOptions{})
				if err == nil {
					// Extract status from CR
					if status, found, err := unstructured.NestedMap(cr.Object, "status"); found && err == nil {
						if phase, ok := status["phase"].(string); ok {
							switch phase {
							case "Running":
								service.Status = "running"
							case "Disconnected", "ScaledDown", "Pending":
								service.Status = "stopped"
							case "Error":
								service.Status = "error"
							default:
								service.Status = "unknown"
							}
							continue
						}
					}
				}
			}

			// Fallback to agentStats if CR not available
			statsMu.RLock()
			stats, exists := agentStats[agentID]
			statsMu.RUnlock()

			if exists && stats.Online {
				service.Status = "running"
			} else {
				service.Status = "stopped"
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"workspaces": workspaces,
		"total":      total,
	})
}

// GET /api/admin/stats - Get global statistics (admin only)
func handleGetAdminStats(w http.ResponseWriter, r *http.Request) {
	userCount, err := dbpkg.UserRepo().Count()
	if err != nil {
		http.Error(w, "Failed to count users: "+err.Error(), http.StatusInternalServerError)
		return
	}

	workspaceCount, err := dbpkg.WorkspaceRepo().Count()
	if err != nil {
		http.Error(w, "Failed to count workspaces: "+err.Error(), http.StatusInternalServerError)
		return
	}

	serviceCount, err := dbpkg.ServiceRepo().Count()
	if err != nil {
		http.Error(w, "Failed to count services: "+err.Error(), http.StatusInternalServerError)
		return
	}

	activeServiceCount, err := dbpkg.ServiceRepo().CountActive()
	if err != nil {
		http.Error(w, "Failed to count active services: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate resource totals
	workspaces, err := dbpkg.WorkspaceRepo().GetAll(10000, 0) // Get all workspaces
	if err != nil {
		log.Printf("Failed to get all workspaces for stats: %v", err)
	}

	services, err := dbpkg.ServiceRepo().GetAll()
	if err != nil {
		log.Printf("Failed to get all services for stats: %v", err)
	}

	// Calculate PVC totals
	totalPVCCount := int64(0)
	totalPVCSizeGi := float64(0)
	for _, ws := range workspaces {
		if ws.PVCName != "" {
			totalPVCCount++
			// Parse storage size (e.g., "50Gi" -> 50)
			sizeStr := ws.StorageSize
			if len(sizeStr) > 2 && (sizeStr[len(sizeStr)-2:] == "Gi") {
				if size, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-2], 64); err == nil {
					totalPVCSizeGi += size
				}
			}
		}
	}

	// Calculate resource totals from services
	totalCPUCores := float64(0)
	totalMemoryGiB := float64(0)
	totalGPUCount := int64(0)
	for _, svc := range services {
		if svc.CPUCores.Valid {
			if cpu, err := strconv.ParseFloat(svc.CPUCores.String, 64); err == nil {
				totalCPUCores += cpu
			}
		}
		if svc.MemoryGiB.Valid {
			if mem, err := strconv.ParseFloat(svc.MemoryGiB.String, 64); err == nil {
				totalMemoryGiB += mem
			}
		}
		if svc.GPUCount.Valid {
			totalGPUCount += svc.GPUCount.Int64
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total_users":       userCount,
		"total_workspaces":  workspaceCount,
		"total_services":    serviceCount,
		"active_services":   activeServiceCount,
		"total_pvc_count":   totalPVCCount,
		"total_pvc_size_gi": totalPVCSizeGi,
		"total_cpu_cores":   totalCPUCores,
		"total_memory_gib":  totalMemoryGiB,
		"total_gpu_count":   totalGPUCount,
	})
}

// GET /api/admin/audit-logs - List audit logs (admin only)
func handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	// Parse filter params
	filter := dbpkg.AuditLogFilter{
		UserID:   r.URL.Query().Get("user_id"),
		Action:   r.URL.Query().Get("action"),
		Resource: r.URL.Query().Get("resource"),
	}

	// Parse date range
	if start := r.URL.Query().Get("start_date"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			filter.StartDate = &t
		}
	}
	if end := r.URL.Query().Get("end_date"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			filter.EndDate = &t
		}
	}

	auditRepo := dbpkg.AuditLogRepo()
	logs, total, err := auditRepo.Search(filter, limit, offset)
	if err != nil {
		http.Error(w, "Failed to search audit logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logs,
		"total": total,
	})
}

// getDatabaseConfig retrieves database configuration from environment with defaults
func getDatabaseConfig() (host, port, user, password, database string) {
	host = os.Getenv("DB_HOST")
	port = os.Getenv("DB_PORT")
	user = os.Getenv("DB_USER")
	password = os.Getenv("DB_PASSWORD")
	database = os.Getenv("DB_NAME")

	if host == "" {
		host = "postgresql"
	}
	if port == "" {
		port = "5432"
	}
	if user == "" {
		user = "postgres"
	}
	if database == "" {
		database = "kuberde"
	}
	return
}

// initDatabase initializes PostgreSQL database connection with retry logic
func initDatabase() error {
	pgHost, pgPort, pgUser, pgPassword, pgDatabase := getDatabaseConfig()
	pgDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		pgHost, pgPort, pgUser, pgPassword, pgDatabase)

	maxDBRetries := 3
	var dbErr error
	for i := 0; i < maxDBRetries; i++ {
		dbErr = dbpkg.InitDB(pgDSN)
		if dbErr == nil {
			log.Println("✓ PostgreSQL database initialized")
			db = dbpkg.DB
			resourceConfigRepo = repositories.NewResourceConfigRepository(db)
			userQuotaRepo = repositories.NewUserQuotaRepository(db)
			log.Println("✓ Repository instances initialized")
			return nil
		}

		if i < maxDBRetries-1 {
			log.Printf("WARNING: Failed to initialize PostgreSQL database (attempt %d/%d): %v. Retrying in 5s...", i+1, maxDBRetries, dbErr)
			time.Sleep(5 * time.Second)
		}
	}
	return fmt.Errorf("failed to initialize PostgreSQL database after %d retries: %v", maxDBRetries, dbErr)
}

// initKubernetes initializes Kubernetes clients
func initKubernetes() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("WARNING: Failed to get in-cluster config: %v. Scale-up will be disabled.", err)
		return
	}

	k8sClientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("WARNING: Failed to create K8s client: %v. Scale-up will be disabled.", err)
		return
	}
	log.Println("Kubernetes client initialized for auto-scaling")

	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		log.Printf("WARNING: Failed to create Dynamic client: %v. API operations will fail.", err)
	}
}

// routeHTTPRequest routes HTTP requests to appropriate handlers
func routeHTTPRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)

	host := r.Host
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}

	if strings.HasSuffix(host, getAgentDomainSuffix()) {
		handleSubdomainProxy(w, r)
		return
	}

	routeMainDomain(w, r)
}

// routeMainDomain handles routing for main domain requests
func routeMainDomain(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/healthz" || r.URL.Path == "/livez":
		handleHealthz(w, r)
	case r.URL.Path == "/readyz":
		handleReadyz(w, r)
	case r.URL.Path == "/auth/login":
		handleLogin(w, r)
	case r.URL.Path == "/auth/callback":
		handleCallback(w, r)
	case r.URL.Path == "/auth/logout":
		handleLogout(w, r)
	case r.URL.Path == "/auth/refresh":
		handleRefreshToken(w, r)
	case r.URL.Path == "/api/me":
		handleGetCurrentUser(w, r)
	case r.URL.Path == "/api/system-config":
		handleGetSystemConfig(w, r)
	case r.URL.Path == "/api/users":
		handleUsers(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/users/"):
		handleUserDetail(w, r)
	case strings.HasPrefix(r.URL.Path, "/ws"):
		handleAgent(w, r)
	case strings.HasPrefix(r.URL.Path, "/connect/"):
		handleUserConnect(w, r)
	case strings.HasPrefix(r.URL.Path, "/mgmt/"):
		handleMgmtAgentStats(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/agents/") && strings.Contains(r.URL.Path, "/scale-up"):
		handleScaleUpAgent(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/agents/") && strings.Contains(r.URL.Path, "/stop"):
		handleStopAgent(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/agents/"):
		handleDeleteAgent(w, r)
	case r.URL.Path == "/api/agents":
		if r.Method == http.MethodPost {
			handleCreateAgent(w, r)
		} else {
			handleListAgents(w, r)
		}
	case r.URL.Path == "/api/stats":
		handleGetGlobalStats(w, r)
	case r.URL.Path == "/api/connections":
		handleGetConnections(w, r)
	case r.URL.Path == "/api/traffic":
		handleGetTraffic(w, r)
	case r.URL.Path == "/api/events":
		handleGetEvents(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/agents/") && strings.HasSuffix(r.URL.Path, "/logs"):
		handleGetLogs(w, r)
	case r.URL.Path == "/api/workspaces":
		if r.Method == http.MethodPost {
			handleCreateWorkspace(w, r)
		} else {
			handleListWorkspaces(w, r)
		}
	case strings.HasPrefix(r.URL.Path, "/api/workspaces/") && strings.HasSuffix(r.URL.Path, "/services"):
		if r.Method == http.MethodPost {
			handleCreateWorkspaceService(w, r)
		} else {
			handleListWorkspaceServices(w, r)
		}
	case strings.HasPrefix(r.URL.Path, "/api/workspaces/"):
		routeWorkspaceRequest(w, r)
	case r.URL.Path == "/api/services" || strings.HasPrefix(r.URL.Path, "/api/services/"):
		routeServiceRequest(w, r)
	case r.URL.Path == "/api/admin/resource-config":
		routeAdminResourceConfig(w, r)
	case r.URL.Path == "/api/agent-templates/export":
		handleExportAllTemplates(w, r)
	case r.URL.Path == "/api/agent-templates/import":
		handleImportTemplates(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/agent-templates/") && strings.HasSuffix(r.URL.Path, "/export"):
		handleExportSingleTemplate(w, r)
	case r.URL.Path == "/api/agent-templates" || strings.HasPrefix(r.URL.Path, "/api/agent-templates/"):
		handleGetAgentTemplates(w, r)
	case r.URL.Path == "/api/admin/workspaces":
		requireRole("admin")(handleAdminListWorkspaces)(w, r)
	case r.URL.Path == "/api/admin/stats":
		requireRole("admin")(handleGetAdminStats)(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/admin/audit-logs"):
		requireRole("admin")(handleListAuditLogs)(w, r)
	case strings.HasPrefix(r.URL.Path, "/download/cli/"):
		handleDownloadCLI(w, r)
	default:
		_, _ = fmt.Fprintf(w, "<h1>FRP Server</h1><p>Logged in? Check cookie.</p><a href='/auth/login'>Login</a>")
	}
}

// routeWorkspaceRequest handles workspace-specific routing
func routeWorkspaceRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleGetWorkspace(w, r)
	case http.MethodDelete:
		handleDeleteWorkspace(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeServiceRequest handles service-specific routing
func routeServiceRequest(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/restart") && r.Method == http.MethodPut {
		handleRestartService(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/stop") && r.Method == http.MethodPut {
		handleStopService(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/start") && r.Method == http.MethodPut {
		handleStartService(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/logs") && r.Method == http.MethodGet {
		handleGetServiceLogs(w, r)
		return
	}

	serviceID := r.URL.Query().Get("id")
	if serviceID == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			serviceID = parts[0]
		}
	}

	switch r.Method {
	case http.MethodGet:
		if serviceID != "" {
			handleGetService(w, r, serviceID)
		} else {
			http.Error(w, "Service ID required", http.StatusBadRequest)
		}
	case http.MethodPut:
		handleUpdateService(w, r)
	case http.MethodDelete:
		handleDeleteService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeAdminResourceConfig handles admin resource configuration routing
func routeAdminResourceConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		requireRole("admin")(handleGetResourceConfig)(w, r)
	case http.MethodPut:
		requireRole("admin")(handleUpdateResourceConfig)(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	initAuth()

	if err := initKeycloakAdmin(); err != nil {
		log.Printf("WARNING: Failed to initialize Keycloak admin client: %v. User management will be unavailable.", err)
	}

	go runActivityMonitor()

	if err := initDatabase(); err != nil {
		log.Fatalf("FATAL: %v. Pod will restart.", err)
	}

	initKubernetes()

	http.HandleFunc("/", routeHTTPRequest)

	// Configure HTTP server with increased timeouts for large file transfers
	server := &http.Server{
		Addr:              ":8080",
		ReadTimeout:       10 * time.Minute, // Allow large file uploads (2GB @ 10MB/s = ~3.5min + margin)
		WriteTimeout:      10 * time.Minute, // Allow large file downloads
		IdleTimeout:       15 * time.Minute, // Keep connections alive for long transfers
		ReadHeaderTimeout: 30 * time.Second, // Prevent slowloris attacks
		MaxHeaderBytes:    1 << 20,          // 1 MB max header size
	}

	log.Println("Server started on :8080 (timeouts: read/write=10m, idle=15m)")
	log.Fatal(server.ListenAndServe())
}
