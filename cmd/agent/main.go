package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"golang.org/x/oauth2/clientcredentials"
)

var (
	serverURL        = "ws://127.0.0.1:8080/ws"
	localTarget      = "127.0.0.1:22"
	agentID          = "default-agent"
	authClientID     = ""
	authClientSecret = ""
	authTokenURL     = ""
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

// wsConn 适配器 (同 Server)
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

func main() {
	// 构造带 ID 的连接 URL
	connectURL := fmt.Sprintf("%s?id=%s", serverURL, agentID)
	log.Printf("Agent [%s] connecting to %s", agentID, connectURL)

	header := http.Header{}

	// 尝试获取 Client Credentials Token
	if authClientID != "" && authTokenURL != "" {
		log.Printf("Authenticating as Client ID: %s", authClientID)
		config := clientcredentials.Config{
			ClientID:     authClientID,
			ClientSecret: authClientSecret,
			TokenURL:     authTokenURL,
		}

		token, err := config.Token(context.Background())
		if err != nil {
			log.Fatalf("Failed to retrieve access token: %v", err)
		}

		header.Add("Authorization", "Bearer "+token.AccessToken)
		log.Println("Authentication token retrieved successfully")
	}

	// 连接 Server
	ws, _, err := websocket.DefaultDialer.Dial(connectURL, header)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	log.Println("Connected to Server")

	// 配置 Yamux KeepAlive (aligned with server configuration)
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 30 * time.Second       // Increased from 10s for stability
	config.ConnectionWriteTimeout = 120 * time.Second // Increased from 5s for large file transfers

	conn := &wsConn{Conn: ws}
	session, err := yamux.Client(conn, config)
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
		go handleStream(stream)
	}
}

func handleStream(stream net.Conn) {
	defer func() { _ = stream.Close() }()

	var localConn net.Conn
	var err error

	// Retry connecting to local target for up to 60 seconds
	deadline := time.Now().Add(60 * time.Second)
	for {
		localConn, err = net.Dial("tcp", localTarget)
		if err == nil {
			break
		}

		if time.Now().After(deadline) {
			log.Printf("Failed to connect to local target %s after 60s timeout: %v", localTarget, err)
			return
		}

		time.Sleep(500 * time.Millisecond)
	}
	defer func() { _ = localConn.Close() }()

	log.Printf("Connected to local target %s", localTarget)

	go func() {
		if _, err := io.Copy(localConn, stream); err != nil {
			log.Printf("Error copying from stream to local: %v", err)
		}
	}()
	if _, err := io.Copy(stream, localConn); err != nil {
		log.Printf("Error copying from local to stream: %v", err)
	}
}
