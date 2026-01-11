package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var connectCmd = &cobra.Command{
	Use:   "connect <websocket-url>",
	Short: "Connect to the FRP server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		runConnect(url)
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

func runConnect(url string) {
	// 配置 Dialer 跳过 TLS 验证
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	header := http.Header{}

	// 尝试读取 Token
	token, err := loadToken()
	if err == nil && token != nil && token.AccessToken != "" && !token.Expiry.IsZero() {
		// Check if token is expired
		if token.Expiry.Before(time.Now()) {
			fmt.Fprintln(os.Stderr, "⚠️  Stored token has expired.")
			fmt.Fprintln(os.Stderr, "Please run 'kuberde-cli login' to refresh your authentication.")
			fmt.Fprintln(os.Stderr, "If you have logged in via the web UI, the login will complete automatically.")
			os.Exit(1)
		}
		header.Add("Authorization", "Bearer "+token.AccessToken)
		fmt.Fprintln(os.Stderr, "✓ Using stored authentication token.")
		if !token.Expiry.IsZero() {
			remaining := time.Until(token.Expiry)
			fmt.Fprintf(os.Stderr, "  (expires in %v)\n", remaining.Round(time.Hour))
		}
	} else {
		fmt.Fprintln(os.Stderr, "❌ No valid token found.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please authenticate using:")
		fmt.Fprintln(os.Stderr, "  $ kuberde-cli login")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Note: If you have already logged in via the web UI, the login will")
		fmt.Fprintln(os.Stderr, "complete automatically without requiring a password.")
		os.Exit(1)
	}

	// 连接 Server
	ws, resp, err := dialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			log.Fatalf("Dial failed: %v (Status: %s, Body: %s)", err, resp.Status, string(body))
		}
		log.Fatalf("Dial failed: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// 处理中断信号，优雅关闭
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	done := make(chan struct{})

	// 读取 WS -> Stdout
	go func() {
		defer close(done)
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				// log.Printf("read: %v", err) // SSH ProxyCommand 中最好不要输出非数据日志
				return
			}
			if _, err := os.Stdout.Write(message); err != nil {
				return
			}
		}
	}()

	// 读取 Stdin -> WS
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				if err != io.EOF {
					// log.Printf("stdin read error: %v", err)
					_ = err // Suppress empty branch warning
				}
				// Stdin 关闭，发送 Close 消息给 Server
				if err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
					// log.Printf("close write error: %v", err)
					_ = err // Suppress empty branch warning
				}
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				// log.Printf("write: %v", err)
				return
			}
		}
	}()

	select {
	case <-done:
	case <-c:
		// 收到信号，发送 Close
		if err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			// log.Printf("close write error: %v", err)
			_ = err // Suppress empty branch warning
		}
	}
}

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
