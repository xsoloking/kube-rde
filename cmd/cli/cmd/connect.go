package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"tailscale.com/types/key"

	"kuberde/pkg/wgtunnel"
)

var connectCmd = &cobra.Command{
	Use:   "connect <websocket-url>",
	Short: "Connect to an agent (DERP relay with WebSocket fallback)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runConnect(args[0])
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

// runConnect attempts DERP relay first, then falls back to WebSocket relay.
// arg may be:
//   - a full WebSocket URL: "wss://frp.byai.uk/connect/user-alice-dev"
//   - a bare SSH hostname:  "kuberde-user-alice-dev"  (from SSH ProxyCommand %n)
//   - a bare agent ID:      "user-alice-dev"
func runConnect(arg string) {
	token, err := loadToken()
	if err != nil || token == nil || token.AccessToken == "" {
		fmt.Fprintln(os.Stderr, "❌ No valid token found.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please authenticate using:")
		fmt.Fprintln(os.Stderr, "  $ kuberde-cli login")
		os.Exit(1)
	}
	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now()) {
		fmt.Fprintln(os.Stderr, "⚠️  Stored token has expired. Run 'kuberde-cli login' to refresh.")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✓ Using stored authentication token.")

	wsURL, err := resolveConnectURL(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	// Try DERP relay; fall back to WebSocket relay on any error.
	if err := connectViaDERP(wsURL, token.AccessToken); err != nil {
		fmt.Fprintf(os.Stderr, "DERP relay unavailable (%v), using WebSocket relay.\n", err)
		connectViaWebSocket(wsURL, token.AccessToken)
	}
}

// resolveConnectURL turns any of the accepted arg forms into a full wss:// URL.
func resolveConnectURL(arg string) (string, error) {
	// Already a full URL.
	if strings.HasPrefix(arg, "ws://") || strings.HasPrefix(arg, "wss://") {
		return arg, nil
	}

	// The arg is the SSH hostname from ProxyCommand %n, which equals the agent ID
	// (e.g. "kuberde-agent-admin-abc-ssh-8a37f662"). Agent IDs already start with
	// "kuberde-", so no prefix stripping is needed.
	agentID := arg

	// Look up the server URL saved by `kuberde-cli config-ssh`.
	serverURL, err := LoadServerURL()
	if err != nil {
		return "", fmt.Errorf("no server URL configured; run 'kuberde-cli config-ssh --server <url>' first or provide a full wss:// URL")
	}
	serverURL = strings.TrimSuffix(serverURL, "/ws")

	// Convert https → wss, http → ws.
	switch {
	case strings.HasPrefix(serverURL, "https://"):
		serverURL = "wss://" + strings.TrimPrefix(serverURL, "https://")
	case strings.HasPrefix(serverURL, "http://"):
		serverURL = "ws://" + strings.TrimPrefix(serverURL, "http://")
	}

	return serverURL + "/connect/" + agentID, nil
}

// ─── agentCoordInfo ───────────────────────────────────────────────────────────

// agentCoordInfo is the JSON payload returned by GET /api/agent-coordination/{id}.
type agentCoordInfo struct {
	AgentID   string   `json:"agent_id"`
	PublicKey string   `json:"public_key"`
	Endpoints []string `json:"endpoints"`
	DERPUrl   string   `json:"derp_url"`
}

// ─── DERP relay path ──────────────────────────────────────────────────────────

// connectViaDERP establishes a DERP-relayed encrypted tunnel to the agent,
// then bridges stdin/stdout through it (SSH ProxyCommand mode).
//
// The DERP relay uses WireGuard node keys for end-to-end encryption —
// the server can see DERP metadata but cannot read the SSH payload.
func connectViaDERP(wsURL, bearerToken string) error {
	serverHTTPURL, agentID, err := parseConnectURL(wsURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	// 1. Fetch agent's coordination info (public key + DERP URL).
	coord, err := getAgentCoordination(serverHTTPURL, agentID, bearerToken)
	if err != nil {
		return fmt.Errorf("coordination fetch: %w", err)
	}
	if coord.PublicKey == "" {
		return fmt.Errorf("agent has no WireGuard key registered yet")
	}
	derpURL := coord.DERPUrl
	if derpURL == "" {
		return fmt.Errorf("agent did not provide a DERP URL")
	}

	// 2. Load / generate CLI's own WireGuard private key.
	cliPrivKey := loadOrGenerateCLIKey()

	// 3. Parse the agent's public key.
	agentPubKey, err := wgtunnel.ParsePublicKey(coord.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid agent public key %q: %w", coord.PublicKey, err)
	}

	// 4. Register CLI's public key with server so it notifies the agent.
	// If the server reports the agent is offline, abort DERP and let the caller
	// fall back to the WebSocket relay (which handles scale-up etc.).
	if err := registerCLIKey(serverHTTPURL, agentID, cliPrivKey.Public().String(), bearerToken); err != nil {
		return fmt.Errorf("agent offline: %w", err)
	}

	// 5. Dial the agent via DERP relay.
	fmt.Fprintf(os.Stderr, "Connecting via DERP relay to %s...\n", agentPubKey.ShortString())
	dialer, err := wgtunnel.Dial(cliPrivKey, derpURL, agentPubKey)
	if err != nil {
		return fmt.Errorf("DERP dial: %w", err)
	}
	defer dialer.Close()

	// 6. Bridge stdin/stdout through the DERP relay (SSH ProxyCommand role).
	dialer.BridgeStdio(os.Stdin, os.Stdout)
	return nil
}

// parseConnectURL extracts the HTTPS base URL and agent ID from a WebSocket
// connect URL like "wss://frp.byai.uk/connect/user-alice-dev".
func parseConnectURL(wsURL string) (serverBaseURL, agentID string, err error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", "", err
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	default:
		return "", "", fmt.Errorf("unexpected scheme %q", u.Scheme)
	}
	// Path is typically "/connect/{agentID}"
	agentID = strings.TrimPrefix(u.Path, "/connect/")
	if agentID == "" || agentID == u.Path {
		return "", "", fmt.Errorf("could not extract agent ID from path %q", u.Path)
	}
	u.Path = ""
	u.RawQuery = ""
	return u.String(), agentID, nil
}

// getAgentCoordination fetches WireGuard coordination info for an agent.
func getAgentCoordination(serverHTTPURL, agentID, bearerToken string) (*agentCoordInfo, error) {
	req, _ := http.NewRequest(http.MethodGet,
		serverHTTPURL+"/api/agent-coordination/"+agentID, nil)
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var info agentCoordInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// loadOrGenerateCLIKey loads an existing WireGuard private key for this CLI
// user, or generates a new one and persists it.
func loadOrGenerateCLIKey() key.NodePrivate {
	home, _ := os.UserHomeDir()
	keyPath := filepath.Join(home, ".kuberde", "wg-key")

	data, err := os.ReadFile(keyPath)
	if err == nil {
		var k key.NodePrivate
		if err := k.UnmarshalText(data); err == nil {
			return k
		}
	}
	k := key.NewNode()
	text, _ := k.MarshalText()
	_ = os.MkdirAll(filepath.Dir(keyPath), 0700)
	_ = os.WriteFile(keyPath, text, 0600)
	return k
}

// registerCLIKey posts the CLI user's WireGuard public key to the server so it
// can forward it to the agent via the Yamux control channel.
// Returns an error if the server cannot reach the agent (agent offline).
func registerCLIKey(serverHTTPURL, agentID, pubKeyStr, bearerToken string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"user_public_key": pubKeyStr,
	})
	req, _ := http.NewRequest(http.MethodPost,
		serverHTTPURL+"/api/agent-coordination/"+agentID+"/peer",
		strings.NewReader(string(payload)))
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("register key request failed: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d (agent may be offline)", resp.StatusCode)
	}
	return nil
}

// ─── WebSocket relay fallback ─────────────────────────────────────────────────

// connectViaWebSocket uses the existing WebSocket+Yamux relay path.
func connectViaWebSocket(wsURL, accessToken string) {
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	header := http.Header{}
	header.Add("Authorization", "Bearer "+accessToken)

	ws, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			log.Fatalf("Dial failed: %v (Status: %s, Body: %s)", err, resp.Status, string(body))
		}
		log.Fatalf("Dial failed: %v", err)
	}
	defer func() { _ = ws.Close() }()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	done := make(chan struct{})

	// WS → Stdout
	go func() {
		defer close(done)
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if _, err := os.Stdout.Write(message); err != nil {
				return
			}
		}
	}()

	// Stdin → WS
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				_ = ws.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-c:
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}
}

// ─── Token helpers ────────────────────────────────────────────────────────────

func loadToken() (*oauth2.Token, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(home, ".kuberde", "token.json"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}
