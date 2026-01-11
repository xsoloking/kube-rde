package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	issuerURL    string
	clientID     string
	clientSecret string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to the FRP system via OIDC",
	Run:   runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&issuerURL, "issuer", "http://localhost:8080/realms/kuberde", "OIDC Issuer URL")
	loginCmd.Flags().StringVar(&clientID, "client-id", "kuberde-cli", "OIDC Client ID")
	loginCmd.Flags().StringVar(&clientSecret, "client-secret", "", "OIDC Client Secret (optional)")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// 1. Setup Provider
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		log.Fatalf("Failed to query provider: %v", err)
	}

	// 2. Start Local Server to get a redirect URL
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to start local server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	callbackPath := "/callback"
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath)

	// 3. Configure OAuth2
	oauth2Config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	state := "random-state-string" // In production, use crypto random
	authURL := oauth2Config.AuthCodeURL(state)

	// 4. Open Browser
	fmt.Printf("Opening browser for login: %s\n", authURL)
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser, please copy the link above.\n")
	}

	// 5. Handle Callback
	codeChan := make(chan string)
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code found", http.StatusBadRequest)
			return
		}
		codeChan <- code
		_, _ = fmt.Fprintf(w, "Login successful! You can close this window.")
	})

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for code or timeout
	var code string
	select {
	case code = <-codeChan:
	case <-time.After(5 * time.Minute):
		log.Fatal("Timeout waiting for login callback")
	}

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Failed to gracefuly shutdown server: %v", err)
	}

	// 6. Exchange Code for Token
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.Fatalf("Failed to exchange token: %v", err)
	}

	// 7. Save Token
	if err := saveToken(oauth2Token); err != nil {
		log.Fatalf("Failed to save token: %v", err)
	}

	fmt.Println("✓ Successfully logged in!")
	fmt.Println("  Token saved to ~/.kuberde/token.json")
	fmt.Println()

	// Display token expiry information
	if !oauth2Token.Expiry.IsZero() {
		expiryTime := oauth2Token.Expiry
		now := time.Now()
		validDays := expiryTime.Sub(now).Hours() / 24

		fmt.Printf("📌 Token Details:\n")
		fmt.Printf("   Expires: %s\n", expiryTime.Format("2006-01-02 15:04:05"))
		if validDays > 0 {
			fmt.Printf("   Valid for: ~%.0f days\n", validDays)
		}
		fmt.Println()
		fmt.Println("💡 Note:")
		fmt.Println("   - Token will be automatically refreshed when using the web UI")
		fmt.Println("   - For CLI usage, run 'kuberde-cli login' again when token expires")
		fmt.Println("   - If you have logged in via the web UI, CLI login will complete automatically")
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func saveToken(token *oauth2.Token) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".kuberde")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(configDir, "token.json"))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return json.NewEncoder(file).Encode(token)
}
