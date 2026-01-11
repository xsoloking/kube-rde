# Multi-Tenant Architecture Design Document

**Date:** 2025-12-09
**Status:** Design Complete
**Scope:** Support multi-tenant isolation with per-tenant namespaces and shared Server instance

---

## Executive Summary

Extend FRP to support multi-tenancy with:
- **Agent ID Format**: `agent-{tenant}-{user}-{agent-name}` (e.g., `agent-acme-alice-ssh-server`)
- **Authentication**: Keycloak realm with `tenant_id` claim in JWT tokens
- **Namespace Strategy**: Namespace-per-tenant (`frp-acme`, `frp-startup`, etc.)
- **Operator**: Single instance watching all tenant namespaces
- **Server**: Single shared instance (short-term), per-tenant (long-term evolution)
- **Database Isolation**: Application-level (short-term), row-level security (long-term)

---

## Architecture Overview

### Current Single-Tenant Model
```
User (alice) → Keycloak (frp realm) → Server → Agent (user-alice-ssh)
                                        ↓
                                   Kubernetes (kuberde namespace)
```

### New Multi-Tenant Model
```
User (alice@acme) → Keycloak (frp realm) → Server → Agent (agent-acme-alice-ssh)
                    [tenant_id: acme]         ↓
                                          Kubernetes (frp-acme namespace)

User (bob@startup) → Keycloak (frp realm) → Server → Agent (agent-startup-bob-ssh)
                     [tenant_id: startup]      ↓
                                          Kubernetes (frp-startup namespace)
```

---

## Component 1: Keycloak Multi-Tenant Configuration

### 1.1 User Attributes in Keycloak

**File:** `deploy/keycloak/realm-export.json`

Add `tenant_id` custom attribute to user configuration:

```json
{
  "realm": "frp",
  "users": [
    {
      "username": "alice",
      "enabled": true,
      "emailVerified": true,
      "firstName": "Alice",
      "lastName": "Admin",
      "email": "alice@acme.com",
      "attributes": {
        "tenant_id": ["acme"]
      },
      "credentials": [
        {
          "type": "password",
          "value": "password123",
          "temporary": false
        }
      ]
    },
    {
      "username": "bob",
      "enabled": true,
      "emailVerified": true,
      "firstName": "Bob",
      "lastName": "Developer",
      "email": "bob@startup.com",
      "attributes": {
        "tenant_id": ["startup"]
      }
    }
  ]
}
```

### 1.2 Protocol Mapper for Tenant ID

**Configuration:** Create a protocol mapper to include `tenant_id` in access tokens

```json
{
  "name": "tenant-id-mapper",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "config": {
    "user.attribute": "tenant_id",
    "claim.name": "tenant_id",
    "jsonType.label": "String",
    "id.token.claim": "true",
    "access.token.claim": "true",
    "userinfo.token.claim": "true"
  }
}
```

**kcadm.sh Setup Command:**
```bash
kcadm.sh create protocol-mappers/models \
  -r frp \
  -s name=tenant-id-mapper \
  -s protocol=openid-connect \
  -s protocolMapper=oidc-usermodel-attribute-mapper \
  -s 'config."user.attribute"=tenant_id' \
  -s 'config."claim.name"=tenant_id' \
  -s 'config."jsonType.label"=String' \
  -s 'config."id.token.claim"=true' \
  -s 'config."access.token.claim"=true' \
  -s 'config."userinfo.token.claim"=true'
```

### 1.3 Resulting JWT Token

```json
{
  "exp": 1735914400,
  "iat": 1735910800,
  "jti": "a1b2c3d4-e5f6-7890-ghij-klmnopqrstu",
  "iss": "http://keycloak:8081/realms/frp",
  "aud": "account",
  "sub": "alice-uuid-1234",
  "typ": "Bearer",
  "azp": "kuberde-cli",
  "session_state": "session-uuid",
  "acr": "1",
  "allowed-origins": ["http://localhost:8080"],
  "realm_access": {
    "roles": ["default-roles-frp", "offline_access", "uma_authorization"]
  },
  "resource_access": {
    "account": {
      "roles": ["manage-account", "manage-account-links", "view-profile"]
    }
  },
  "name": "Alice Admin",
  "preferred_username": "alice",
  "given_name": "Alice",
  "family_name": "Admin",
  "email": "alice@acme.com",
  "email_verified": true,
  "tenant_id": "acme"
}
```

---

## Component 2: Agent ID Format & Extraction

### 2.1 New Agent ID Structure

```
agent-{tenant}-{user}-{agent-name}

Parts:
  [0] = "agent" (prefix for identification)
  [1] = {tenant}    (tenant slug, e.g., "acme", "startup")
  [2] = {user}      (username from JWT, e.g., "alice", "bob")
  [3+] = {agent-name} (workload name, can contain hyphens)

Examples:
  agent-acme-alice-ssh-server      (Alice's SSH server at Acme)
  agent-acme-alice-files-server    (Alice's files server at Acme)
  agent-startup-bob-dev-env        (Bob's dev environment at Startup)
  agent-enterprise-charlie-prod    (Charlie's production workload)
```

### 2.2 Agent ID Parsing in Code

**File:** `cmd/server/main.go`

```go
// AgentIdentity represents parsed Agent ID
type AgentIdentity struct {
  Prefix    string // "agent"
  Tenant    string // "acme"
  User      string // "alice"
  AgentName string // "ssh-server" (remaining parts joined)
}

// ParseAgentID parses the new multi-tenant format
func ParseAgentID(agentID string) (*AgentIdentity, error) {
  parts := strings.Split(agentID, "-")

  if len(parts) < 4 {
    return nil, fmt.Errorf("invalid agent ID format: %s", agentID)
  }

  if parts[0] != "agent" {
    return nil, fmt.Errorf("agent ID must start with 'agent': %s", agentID)
  }

  return &AgentIdentity{
    Prefix:    parts[0],
    Tenant:    parts[1],
    User:      parts[2],
    AgentName: strings.Join(parts[3:], "-"),
  }, nil
}

// Example usage:
// ParseAgentID("agent-acme-alice-ssh-server")
// → {Prefix: "agent", Tenant: "acme", User: "alice", AgentName: "ssh-server"}
```

---

## Component 3: Token Validation with Tenant Claims

### 3.1 Enhanced Validation Logic

**File:** `cmd/server/main.go`

```go
// TokenClaims represents JWT claims
type TokenClaims struct {
  Sub               string `json:"sub"`
  PreferredUsername string `json:"preferred_username"`
  TenantID          string `json:"tenant_id"`
  Email             string `json:"email"`
  EmailVerified     bool   `json:"email_verified"`
  jwt.StandardClaims
}

// ValidateTokenAndAgent validates token and checks agent access
func ValidateTokenAndAgent(tokenString string, agentID string) error {
  // 1. Parse JWT token
  token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
    // Fetch JWKS and return verification key
    // (existing JWKS validation logic)
    return getJWKSKey(token.Header["kid"])
  })

  if err != nil || !token.Valid {
    return fmt.Errorf("invalid token: %v", err)
  }

  claims, ok := token.Claims.(*TokenClaims)
  if !ok {
    return fmt.Errorf("invalid claims type")
  }

  // 2. Validate tenant_id claim exists
  if claims.TenantID == "" {
    return fmt.Errorf("missing tenant_id in token")
  }

  // 3. Parse agent ID
  agentIdentity, err := ParseAgentID(agentID)
  if err != nil {
    return fmt.Errorf("invalid agent ID: %v", err)
  }

  // 4. Check tenant match
  if claims.TenantID != agentIdentity.Tenant {
    return fmt.Errorf("tenant mismatch: token tenant %s, agent tenant %s",
                     claims.TenantID, agentIdentity.Tenant)
  }

  // 5. Check user match
  if claims.PreferredUsername != agentIdentity.User {
    return fmt.Errorf("user mismatch: token user %s, agent user %s",
                     claims.PreferredUsername, agentIdentity.User)
  }

  // 6. Token is valid for this agent
  return nil
}
```

### 3.2 Usage in Request Handlers

```go
// handleMgmtAgentStats validates tenant + user + agent ownership
func handleMgmtAgentStats(w http.ResponseWriter, r *http.Request) {
  agentID := r.PathValue("id") // e.g., "agent-acme-alice-ssh"

  // Extract token from header or cookie
  token, err := extractToken(r)
  if err != nil {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
  }

  // Validate token AND agent access
  if err := ValidateTokenAndAgent(token, agentID); err != nil {
    log.Printf("Authorization denied: %v", err)
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
  }

  // Fetch and return agent stats
  stats := getAgentStats(agentID)
  json.NewEncoder(w).Encode(stats)
}
```

---

## Component 4: Kubernetes Multi-Namespace Setup

### 4.1 Namespace-per-Tenant Strategy

**Namespaces created:**
```
frp-system     (system components: Operator, Server)
frp-acme       (Acme tenant agents)
frp-startup    (Startup tenant agents)
frp-enterprise (Enterprise tenant agents)
```

**File:** `deploy/k8s/00-namespace.yaml` (updated for multi-tenant)

```yaml
---
# System namespace for shared components
apiVersion: v1
kind: Namespace
metadata:
  name: frp-system
  labels:
    environment: production

---
# Tenant namespace: Acme
apiVersion: v1
kind: Namespace
metadata:
  name: frp-acme
  labels:
    tenant-id: acme
    environment: production

---
# Tenant namespace: Startup
apiVersion: v1
kind: Namespace
metadata:
  name: frp-startup
  labels:
    tenant-id: startup
    environment: production
```

### 4.2 RBAC for Operator (Multi-Namespace Access)

**File:** `deploy/k8s/rbac-operator.yaml`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kuberde-operator
rules:
# Watch RDEAgent CRDs in ALL namespaces
- apiGroups: ["frp.byai.io"]
  resources: ["rdeagents"]
  verbs: ["get", "list", "watch"]

# Create/update Deployments in tenant namespaces
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["create", "update", "patch", "get", "list", "watch"]
  namespaces: ["frp-acme", "frp-startup", "frp-enterprise"]

# Manage Services in tenant namespaces
- apiGroups: [""]
  resources: ["services"]
  verbs: ["create", "update", "patch", "get", "list", "watch"]
  namespaces: ["frp-acme", "frp-startup", "frp-enterprise"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kuberde-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kuberde-operator
subjects:
- kind: ServiceAccount
  name: kuberde-operator
  namespace: frp-system
```

### 4.3 Updated RDEAgent CRD with Tenant Label

**File:** `deploy/k8s/05-example-agent-acme.yaml`

```yaml
apiVersion: frp.byai.io/v1beta1
kind: RDEAgent
metadata:
  name: alice-ssh-server
  namespace: frp-acme                    # Tenant namespace
  labels:
    tenant-id: acme                      # Tenant label
    user: alice                          # User label
spec:
  serverUrl: "ws://frp-server.frp-system.svc:80/ws"
  authSecret: "test-agents-auth"

  # Tenant + user + agent name
  # Agent ID will be: agent-acme-alice-ssh-server
  localTarget: "localhost:2222"

  workloadContainer:
    image: "linuxserver/openssh-server:latest"
    ports:
      - containerPort: 2222
        name: ssh
    env:
      - name: PASSWORD_ACCESS
        value: "true"
      - name: USER_NAME
        value: "agent-user"
      - name: USER_PASSWORD
        value: "sshpassword123"

  ttl: "2m"
  sshPublicKeys:
    - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIHj8pk5seTiiZUvyy5q48gB9DvjrO1dTuLWrkcvUXOS alice@acme.com"
```

---

## Component 5: Operator Changes for Multi-Tenant

### 5.1 Namespace-Aware Reconciliation

**File:** `cmd/operator/main.go`

```go
func main() {
  // Watch all namespaces (not just kuberde)
  factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, time.Second*60)
  informer := factory.ForResource(frpAgentGVR).Namespace("").Informer()  // empty = all namespaces

  controller := &Controller{
    k8sClient: k8sClient,
    dynClient: dynClient,
    informer:  informer,
  }

  informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc:    controller.onAdd,
    UpdateFunc: func(old, new interface{}) { controller.onAdd(new) },
    DeleteFunc: controller.onDelete,
  })

  // Start watching
  stop := make(chan struct{})
  go factory.Start(stop)
  if !cache.WaitForCacheSync(stop, informer.HasSynced) {
    log.Fatalf("Failed to sync cache")
  }
}
```

### 5.2 Generate Multi-Tenant Agent ID

**File:** `cmd/operator/main.go`

```go
func (c *Controller) reconcileDeployment(cr *unstructured.Unstructured) error {
  crName := cr.GetName()
  namespace := cr.GetNamespace()

  // Extract tenant from namespace label
  tenant, found, _ := unstructured.NestedString(cr.Object, "metadata", "labels", "tenant-id")
  if !found {
    // Alternative: extract from namespace name (frp-acme → acme)
    tenant = strings.TrimPrefix(namespace, "frp-")
  }

  // Extract user from label
  user, found, _ := unstructured.NestedString(cr.Object, "metadata", "labels", "user")
  if !found {
    return fmt.Errorf("missing 'user' label in CRD")
  }

  // Generate new format Agent ID
  agentID := fmt.Sprintf("agent-%s-%s-%s", tenant, user, crName)

  log.Printf("Reconciling multi-tenant agent: %s (namespace: %s)", agentID, namespace)

  // Continue with deployment creation...
  deployment := buildDeployment(cr, agentID)

  _, err := c.k8sClient.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
  if err != nil && !errors.IsAlreadyExists(err) {
    return err
  }

  return nil
}
```

---

## Component 6: Server Multi-Tenant Agent Session Management

### 6.1 Tenant-Scoped Agent Sessions

**File:** `cmd/server/main.go`

```go
type AgentSession struct {
  ID        string           // agent-acme-alice-ssh
  Tenant    string           // acme
  User      string           // alice
  Session   yamux.Session    // Yamux multiplexed connection
  CreatedAt time.Time
}

type TenantAgentSessions struct {
  mu       sync.RWMutex
  sessions map[string]*AgentSession  // agentID → session
}

// Global agent sessions per tenant
var agentSessionsByTenant map[string]*TenantAgentSessions

// Store agent session with tenant context
func (t *TenantAgentSessions) Store(identity *AgentIdentity, session yamux.Session) {
  t.mu.Lock()
  defer t.mu.Unlock()

  t.sessions[identity.AgentID] = &AgentSession{
    ID:        identity.AgentID,
    Tenant:    identity.Tenant,
    User:      identity.User,
    Session:   session,
    CreatedAt: time.Now(),
  }

  log.Printf("[%s] Agent %s connected", identity.Tenant, identity.AgentID)
}

// Get agent session with tenant validation
func (t *TenantAgentSessions) Get(agentID string, expectedTenant string) (*AgentSession, error) {
  t.mu.RLock()
  defer t.mu.RUnlock()

  session, exists := t.sessions[agentID]
  if !exists {
    return nil, fmt.Errorf("agent not connected: %s", agentID)
  }

  // Validate tenant match
  if session.Tenant != expectedTenant {
    return nil, fmt.Errorf("tenant mismatch for agent %s", agentID)
  }

  return session, nil
}
```

### 6.2 Tenant-Filtered Agent Stats API

```go
// GET /mgmt/agents/{id}
func handleMgmtAgentStats(w http.ResponseWriter, r *http.Request) {
  agentID := r.PathValue("id")

  // Extract and validate token
  token, err := extractToken(r)
  if err != nil {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
  }

  // Validate token AND tenant/user/agent ownership
  if err := ValidateTokenAndAgent(token, agentID); err != nil {
    log.Printf("Access denied to %s: %v", agentID, err)
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
  }

  // Parse agent ID to get tenant
  identity, _ := ParseAgentID(agentID)

  // Get agent session with tenant validation
  agentSessions := agentSessionsByTenant[identity.Tenant]
  session, err := agentSessions.Get(agentID, identity.Tenant)
  if err != nil {
    http.Error(w, "Agent not found", http.StatusNotFound)
    return
  }

  // Return stats
  stats := AgentStats{
    Online:           true,
    LastActivity:     time.Now(),
    HasActiveSession: true,
  }

  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(stats)
}
```

---

## Component 7: Application-Level Tenant Isolation (Phase 1)

### 7.1 Tenant Context Helper

**File:** `cmd/server/tenant.go`

```go
// TenantContext represents the current request's tenant context
type TenantContext struct {
  TenantID string
  UserID   string
  Username string
  Email    string
}

// ExtractTenantContext extracts tenant info from JWT token
func ExtractTenantContext(tokenString string) (*TenantContext, error) {
  token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
    return getJWKSKey(token.Header["kid"])
  })

  if err != nil || !token.Valid {
    return nil, fmt.Errorf("invalid token: %v", err)
  }

  claims, ok := token.Claims.(*TokenClaims)
  if !ok {
    return nil, fmt.Errorf("invalid claims")
  }

  return &TenantContext{
    TenantID: claims.TenantID,
    UserID:   claims.Sub,
    Username: claims.PreferredUsername,
    Email:    claims.Email,
  }, nil
}

// Middleware to attach tenant context to request
func tenantMiddleware(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    token, _ := extractToken(r)
    tenantCtx, err := ExtractTenantContext(token)

    if err != nil {
      http.Error(w, "Unauthorized", http.StatusUnauthorized)
      return
    }

    // Store in request context
    ctx := context.WithValue(r.Context(), "tenant", tenantCtx)
    next.ServeHTTP(w, r.WithContext(ctx))
  })
}

// Usage in handlers:
func someHandler(w http.ResponseWriter, r *http.Request) {
  tenantCtx := r.Context().Value("tenant").(*TenantContext)

  // Only access data belonging to tenantCtx.TenantID
  // Example: SELECT * FROM agents WHERE tenant_id = ?
  //         Parameter: tenantCtx.TenantID
}
```

### 7.2 Query-Level Tenant Filtering

```go
// GetAgentsByTenant returns only agents for specified tenant
func (db *Database) GetAgentsByTenant(ctx context.Context, tenantID string) ([]Agent, error) {
  tenantCtx := ctx.Value("tenant").(*TenantContext)

  // Always verify tenant match
  if tenantID != tenantCtx.TenantID {
    return nil, fmt.Errorf("forbidden: cannot access tenant %s", tenantID)
  }

  query := `
    SELECT id, tenant_id, user_id, name, status, created_at
    FROM agents
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `

  rows, err := db.Query(ctx, query, tenantID)
  // ... process rows ...
}
```

---

## Component 8: Future Evolution to Row-Level Security (Phase 2)

### 8.1 PostgreSQL Schema with Tenant Column

```sql
-- Add tenant_id column to agents table
ALTER TABLE agents ADD COLUMN tenant_id VARCHAR(255) NOT NULL DEFAULT 'default';
ALTER TABLE agents ADD INDEX idx_tenant_id (tenant_id);

-- Add composite unique constraint
ALTER TABLE agents ADD UNIQUE (tenant_id, user_id, name);

-- Create view for current tenant (future RLS)
CREATE VIEW agents_for_current_tenant AS
  SELECT * FROM agents
  WHERE tenant_id = current_setting('app.current_tenant')::text;
```

### 8.2 Enable Row-Level Security (PostgreSQL 9.5+)

```sql
-- Enable RLS on agents table
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;

-- Create tenant isolation policy
CREATE POLICY tenant_isolation ON agents
  USING (tenant_id = current_setting('app.current_tenant')::text)
  WITH CHECK (tenant_id = current_setting('app.current_tenant')::text);

-- Apply to all roles
ALTER TABLE agents FORCE ROW LEVEL SECURITY;
```

---

## Data Flow Examples

### Example 1: Multi-Tenant SSH Access

```
1. User (alice@acme) logs in → Keycloak → JWT with tenant_id: "acme"

2. User accesses agent "agent-acme-alice-ssh-server"
   → Server validates: tenant matches, user matches, agent name matches

3. Server finds agent session in agentSessionsByTenant["acme"]

4. Server opens Yamux stream to agent

5. Agent receives stream, connects to localhost:2222

6. User ↔ SSH Server (data forwarding via Yamux)

7. User (bob@startup) CANNOT access agent-acme-alice-ssh-server:
   → Validation fails: tenant "startup" ≠ agent tenant "acme" → 403 Forbidden
```

### Example 2: Operator Reconciliation

```
1. Admin creates RDEAgent in namespace frp-acme:
   metadata:
     name: alice-ssh-server
     namespace: frp-acme
     labels:
       tenant-id: acme
       user: alice

2. Operator watches all namespaces, receives CREATE event

3. Operator extracts:
   - Tenant: "acme" (from label or namespace name)
   - User: "alice" (from label)
   - AgentName: "alice-ssh-server" (from metadata.name)

4. Operator generates Agent ID: "agent-acme-alice-ssh-server"

5. Operator creates Deployment in frp-acme namespace
   - Deployment name: "alice-ssh-server"
   - Labels: tenant-id=acme, user=alice
   - Agent gets AGENT_ID=agent-acme-alice-ssh-server env var

6. Agent Pod connects to Server with ID "agent-acme-alice-ssh-server"

7. Server validates:
   - Agent ID format: ✓
   - Tenant "acme" matches namespace: ✓
   - Stores in agentSessionsByTenant["acme"]
```

---

## Authorization Matrix

| User | Token Tenant | Agent ID | Access? | Reason |
|------|--------------|----------|---------|--------|
| alice (acme) | acme | agent-acme-alice-ssh | ✅ YES | Tenant + user + agent match |
| alice (acme) | acme | agent-acme-bob-ssh | ❌ NO | User "alice" ≠ agent user "bob" |
| alice (acme) | acme | agent-startup-alice-ssh | ❌ NO | Token tenant "acme" ≠ agent tenant "startup" |
| bob (startup) | startup | agent-startup-bob-ssh | ✅ YES | Tenant + user + agent match |
| bob (startup) | startup | agent-acme-alice-ssh | ❌ NO | Token tenant "startup" ≠ agent tenant "acme" |

---

## Implementation Checklist

### Phase 1: Application-Level Isolation (Current)
- [ ] Add `tenant_id` attribute to Keycloak users
- [ ] Create protocol mapper to include `tenant_id` in JWT
- [ ] Update Operator to watch all namespaces
- [ ] Implement `ParseAgentID()` for new format
- [ ] Implement `ValidateTokenAndAgent()` with tenant validation
- [ ] Update Server agent session management with tenant context
- [ ] Create namespace-per-tenant manifests
- [ ] Update RBAC for multi-namespace Operator
- [ ] Test multi-tenant access control
- [ ] Verify tenant isolation at application layer

### Phase 2: Row-Level Security (Future)
- [ ] Add `tenant_id` column to database schema
- [ ] Migrate existing agents to default tenant
- [ ] Enable PostgreSQL RLS policies
- [ ] Add `SET app.current_tenant` before database queries
- [ ] Remove application-level tenant checks (RLS handles it)
- [ ] Stress test with many tenants

### Phase 3: Per-Tenant Server (Future)
- [ ] Design per-tenant Server deployment strategy
- [ ] Implement Server discovery mechanism
- [ ] Update Operator to route agents to tenant-specific Server
- [ ] Add cross-tenant monitoring/management console
- [ ] Implement tenant-to-Server affinity

---

## Testing Strategy

### Scenario 1: Tenant Isolation
- Create 3 tenants: acme, startup, enterprise
- Deploy agents in each tenant namespace
- Verify user from one tenant cannot access agents from another
- Verify JWT claims correctly include tenant_id

### Scenario 2: Multi-Namespace Operator
- Verify Operator successfully reconciles agents across all tenant namespaces
- Verify agent IDs are correctly generated with tenant prefix
- Verify agents connect to shared Server

### Scenario 3: Authorization Edge Cases
- User with wrong tenant tries to access agent → 403
- Spoofed token without tenant_id → 401
- Agent ID format invalid → validation error
- Tenant in URL mismatch with token tenant → 403

### Scenario 4: Performance
- Compare single-namespace vs multi-namespace Operator performance
- Verify no cross-tenant data leakage under load
- Measure query performance with `tenant_id` filtering

---

## Migration Path

### From Single-Tenant to Multi-Tenant

1. **Step 1:** Deploy new code with multi-tenant support
   - Keep existing agents in default namespace
   - Set all existing agents to `tenant_id="default"`
   - Operators support both old and new formats

2. **Step 2:** Gradually migrate tenants to dedicated namespaces
   - Create `frp-{tenant}` namespaces
   - Move agents to tenant namespaces
   - Update Keycloak with tenant assignments

3. **Step 3:** Decommission single-tenant code paths
   - Remove old Agent ID format support
   - Remove old authorization checks
   - Require all agents to use new format

---

## Summary

This multi-tenant architecture provides:

✅ **Strong Tenant Isolation**: Agent IDs encode tenant + user, validated at every access
✅ **Scalable**: Supports hundreds of tenants with namespace-per-tenant pattern
✅ **Keycloak-Native**: Leverages standard OIDC claims and protocol mappers
✅ **Kubernetes-Native**: Uses namespaces and RBAC for infrastructure isolation
✅ **Evolvable**: Phase 1 (app-level), Phase 2 (RLS), Phase 3 (per-tenant servers)
✅ **Zero Data Leakage**: Multi-layer validation prevents cross-tenant access

