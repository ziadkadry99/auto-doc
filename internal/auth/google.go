package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleScope = "https://www.googleapis.com/auth/generative-language"

// RunGoogleOAuth performs the OAuth2 browser flow for Google API access.
// It starts a local HTTP server, opens the browser for user consent,
// and exchanges the authorization code for tokens.
func RunGoogleOAuth(clientID, clientSecret string) (*oauth2.Token, error) {
	// Find an available port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{googleScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
	}

	// Channel to receive the auth code from the callback.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "no authorization code received"
			}
			fmt.Fprintf(w, "<html><body><h2>Authorization failed</h2><p>%s</p><p>You can close this tab.</p></body></html>", errMsg)
			errCh <- fmt.Errorf("OAuth callback error: %s", errMsg)
			return
		}
		fmt.Fprint(w, "<html><body><h2>Authorization successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}

	// Start serving in the background.
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("local server error: %w", err)
		}
	}()
	defer server.Close()

	// Open browser to consent URL.
	authURL := conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\nOpening browser for Google authorization...\n")
	fmt.Printf("If the browser doesn't open, visit this URL:\n%s\n\n", authURL)
	openBrowser(authURL)

	// Wait for the callback or timeout.
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authorization timed out after 5 minutes")
	}

	// Exchange code for tokens.
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchanging authorization code: %w", err)
	}

	return token, nil
}

// NewGoogleTokenSource creates an auto-refreshing OAuth2 token source from stored credentials.
func NewGoogleTokenSource(creds *GoogleCredentials) oauth2.TokenSource {
	conf := &oauth2.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Scopes:       []string{googleScope},
		Endpoint:     google.Endpoint,
	}

	expiry, _ := time.Parse(time.RFC3339, creds.TokenExpiry)
	token := &oauth2.Token{
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		Expiry:       expiry,
		TokenType:    "Bearer",
	}

	return conf.TokenSource(context.Background(), token)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
