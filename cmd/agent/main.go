package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"golang.org/x/oauth2/clientcredentials"
	"tailscale.com/types/key"

	"kuberde/pkg/wgtunnel"
)

var (
	serverURL        = "ws://127.0.0.1:8080/ws"
	localTarget      = "127.0.0.1:22"
	agentID          = "default-agent"
	authClientID     = ""
	authClientSecret = ""
	authTokenURL     = ""

	// WireGuard / DERP state
	agentNodePriv key.NodePrivate
	agentDERPURL  string // populated after key registration
)

func init() {
	if url := os.Getenv("SERVER_URL"); url != "" {
		serverURL = url
	}
	if target := os.Getenv("LOCAL_TARGET"); target != "" {
		localTarget = target
	}
	if id := os.Getenv("AGENT_ID"); id != "" {
		agentID = id
	}
	authClientID = os.Getenv("AUTH_CLIENT_ID")
	authClientSecret = os.Getenv("AUTH_CLIENT_SECRET")
	authTokenURL = os.Getenv("AUTH_TOKEN_URL")
}

// ─── WireGuard key management ─────────────────────────────────────────────────

func wireguardKeyPath() string {
	base := os.Getenv("KUBERDE_DATA_DIR")
	if base == "" {
		base = "/var/lib/kuberde-agent"
	}
	return filepath.Join(base, "wg-key")
}

func loadOrGenerateKey(path string) key.NodePrivate {
	data, err := os.ReadFile(path)
	if err == nil {
		var k key.NodePrivate
		if err := k.UnmarshalText(data); err == nil {
			log.Printf("Loaded existing WireGuard key from %s", path)
			return k
		}
	}
	k := key.NewNode()
	text, _ := k.MarshalText()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err == nil {
		_ = os.WriteFile(path, text, 0600)
	}
	log.Printf("Generated new WireGuard key (saved to %s)", path)
	return k
}

// registerWireGuardKey POSTs this agent's WireGuard public key to the Server's
// coordination API and stores the returned DERP URL for incoming connections.
func registerWireGuardKey(serverHTTPURL, bearerToken string) {
	pubKey := agentNodePriv.Public().String()
	payload, _ := json.Marshal(map[string]interface{}{
		"public_key": pubKey,
		"endpoints":  []string{},
	})

	url := serverHTTPURL + "/api/agent-coordination/" + agentID
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("WireGuard registration: failed to build request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("WireGuard registration: HTTP request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("WireGuard registration: server returned %d", resp.StatusCode)
		return
	}

	var result struct {
		DERPUrl string `json:"derp_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.DERPUrl != "" {
		agentDERPURL = result.DERPUrl
		log.Printf("WireGuard public key registered. DERP relay: %s", agentDERPURL)
	} else {
		log.Printf("WireGuard public key registered (key prefix: %s)", pubKey[:16])
	}
}

// handleControlMessage decodes a JSON control message from the Server sent over
// a dedicated Yamux stream (identified by leading 0x01 byte).
// For "add_wireguard_peer", it spawns a DERP relay session to the connecting CLI.
func handleControlMessage(s net.Conn) {
	defer s.Close()
	var msg struct {
		Type      string `json:"type"`
		PublicKey string `json:"public_key"`
		DERPUrl   string `json:"derp_url"`
	}
	if err := json.NewDecoder(s).Decode(&msg); err != nil {
		log.Printf("Control message decode error: %v", err)
		return
	}
	switch msg.Type {
	case "add_wireguard_peer":
		derpURL := msg.DERPUrl
		if derpURL == "" {
			derpURL = agentDERPURL
		}
		if derpURL == "" {
			log.Printf("DERP: no DERP URL available, cannot relay to peer")
			return
		}
		peerKey, err := wgtunnel.ParsePublicKey(msg.PublicKey)
		if err != nil {
			log.Printf("DERP: invalid peer public key %q: %v", msg.PublicKey, err)
			return
		}
		log.Printf("DERP: starting relay session for CLI peer %s", peerKey.ShortString())
		go func() {
			listener := wgtunnel.NewAgentListener(agentNodePriv, derpURL, localTarget)
			if err := listener.Accept(peerKey); err != nil {
				log.Printf("DERP relay session ended: %v", err)
			}
		}()
	default:
		log.Printf("Unknown control message type: %q", msg.Type)
	}
}

// ─── WebSocket / Yamux adapter ───────────────────────────────────────────────

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

func (c *wsConn) SetDeadline(t time.Time) error {
	return c.Conn.UnderlyingConn().SetDeadline(t)
}

func (c *wsConn) SetReadDeadline(t time.Time) error {
	return c.Conn.UnderlyingConn().SetReadDeadline(t)
}

func (c *wsConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.UnderlyingConn().SetWriteDeadline(t)
}

// ─── Entry point ─────────────────────────────────────────────────────────────

func main() {
	// Load or generate persistent WireGuard node key.
	agentNodePriv = loadOrGenerateKey(wireguardKeyPath())
	log.Printf("WireGuard node key ready (public: %s)", agentNodePriv.Public().ShortString())

	connectURL := fmt.Sprintf("%s?id=%s", serverURL, agentID)
	log.Printf("Agent [%s] connecting to %s", agentID, connectURL)

	header := http.Header{}
	var bearerToken string

	// Obtain OAuth2 client-credentials token.
	if authClientID != "" && authTokenURL != "" {
		log.Printf("Authenticating as Client ID: %s", authClientID)
		ccConfig := clientcredentials.Config{
			ClientID:     authClientID,
			ClientSecret: authClientSecret,
			TokenURL:     authTokenURL,
		}
		token, err := ccConfig.Token(context.Background())
		if err != nil {
			log.Fatalf("Failed to retrieve access token: %v", err)
		}
		bearerToken = token.AccessToken
		header.Add("Authorization", "Bearer "+bearerToken)
		log.Println("Authentication token retrieved successfully")
	}

	// Connect to Server WebSocket.
	ws, _, err := websocket.DefaultDialer.Dial(connectURL, header)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	log.Println("Connected to Server")

	// Register WireGuard public key with the Server's coordination API.
	// This also retrieves the DERP relay URL for incoming CLI connections.
	if bearerToken != "" {
		serverHTTPURL := strings.Replace(serverURL, "ws://", "http://", 1)
		serverHTTPURL = strings.Replace(serverHTTPURL, "wss://", "https://", 1)
		serverHTTPURL = strings.TrimSuffix(serverHTTPURL, "/ws")
		go registerWireGuardKey(serverHTTPURL, bearerToken)
	}

	// Set up Yamux session (used for browser WebSocket relay path).
	yconfig := yamux.DefaultConfig()
	yconfig.EnableKeepAlive = true
	yconfig.KeepAliveInterval = 30 * time.Second
	yconfig.ConnectionWriteTimeout = 120 * time.Second

	conn := &wsConn{Conn: ws}
	session, err := yamux.Client(conn, yconfig)
	if err != nil {
		log.Fatalf("Yamux client init failed: %v", err)
	}

	log.Println("Session established. Waiting for streams...")

	for {
		stream, err := session.Accept()
		if err != nil {
			log.Printf("Session accept failed: %v", err)
			break
		}
		log.Println("Accepted new stream")
		go dispatchStream(stream)
	}
}

// dispatchStream peeks at the first byte of an incoming Yamux stream to decide
// whether it is a control message (0x01) or a regular data stream.
func dispatchStream(s net.Conn) {
	var peek [1]byte
	if _, err := io.ReadFull(s, peek[:]); err != nil {
		s.Close()
		return
	}
	if peek[0] == 0x01 {
		handleControlMessage(s)
	} else {
		handleStream(io.MultiReader(bytes.NewReader(peek[:]), s), s)
	}
}

func handleStream(r io.Reader, s net.Conn) {
	defer func() { _ = s.Close() }()

	var localConn net.Conn
	var err error

	deadline := time.Now().Add(60 * time.Second)
	for {
		localConn, err = net.Dial("tcp", localTarget)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			log.Printf("Failed to connect to local target %s after 60s: %v", localTarget, err)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	defer func() { _ = localConn.Close() }()

	log.Printf("Connected to local target %s", localTarget)

	go func() {
		if _, err := io.Copy(localConn, r); err != nil {
			log.Printf("Error copying from stream to local: %v", err)
		}
	}()
	if _, err := io.Copy(s, localConn); err != nil {
		log.Printf("Error copying from local to stream: %v", err)
	}
}
