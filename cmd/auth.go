package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ziadkadry99/auto-doc/internal/auth"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage API credentials for LLM providers",
	Long: `Store and manage API credentials for LLM providers.

Credentials are stored in ~/.autodoc/credentials.json and used
as a fallback when environment variables are not set.`,
}

var authGoogleCmd = &cobra.Command{
	Use:   "google",
	Short: "Authenticate with Google via OAuth2",
	Long: `Opens your browser for Google OAuth2 authorization.

This grants autodoc access to the Generative Language API (Gemini).
You need a Google Cloud OAuth2 Client ID and Secret, which can be
created at https://console.cloud.google.com/apis/credentials`,
	RunE: runAuthGoogle,
}

var authAnthropicCmd = &cobra.Command{
	Use:   "anthropic",
	Short: "Store Anthropic API key",
	Long: `Store your Anthropic API key for persistent use.

Get your API key at https://console.anthropic.com/settings/keys`,
	RunE: runAuthAnthropic,
}

var authOpenAICmd = &cobra.Command{
	Use:   "openai",
	Short: "Store OpenAI API key",
	Long: `Store your OpenAI API key for persistent use.

Get your API key at https://platform.openai.com/api-keys`,
	RunE: runAuthOpenAI,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which providers have stored credentials",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout [provider]",
	Short: "Remove stored credentials",
	Long: `Remove stored credentials for a provider.

If no provider is specified, removes all stored credentials.
Valid providers: google, anthropic, openai`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAuthLogout,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authGoogleCmd)
	authCmd.AddCommand(authAnthropicCmd)
	authCmd.AddCommand(authOpenAICmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}

func runAuthGoogle(cmd *cobra.Command, args []string) error {
	// Get Client ID and Secret from env or prompt.
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	reader := bufio.NewReader(os.Stdin)

	if clientID == "" {
		fmt.Print("Google OAuth2 Client ID: ")
		input, _ := reader.ReadString('\n')
		clientID = strings.TrimSpace(input)
		if clientID == "" {
			return fmt.Errorf("client ID is required")
		}
	}

	if clientSecret == "" {
		fmt.Print("Google OAuth2 Client Secret: ")
		input, _ := reader.ReadString('\n')
		clientSecret = strings.TrimSpace(input)
		if clientSecret == "" {
			return fmt.Errorf("client secret is required")
		}
	}

	token, err := auth.RunGoogleOAuth(clientID, clientSecret)
	if err != nil {
		return fmt.Errorf("OAuth flow failed: %w", err)
	}

	// Store the credentials.
	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("loading credentials: %w", err)
	}

	creds.Google = &auth.GoogleCredentials{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry.Format(time.RFC3339),
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	if err := auth.Save(creds); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Println("Google credentials stored successfully!")
	return nil
}

func runAuthAnthropic(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Anthropic API key: ")
	input, _ := reader.ReadString('\n')
	apiKey := strings.TrimSpace(input)
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Verify the key with a lightweight API call.
	fmt.Print("Verifying API key... ")
	if err := verifyAnthropicKey(apiKey); err != nil {
		fmt.Println("failed!")
		return fmt.Errorf("key verification failed: %w", err)
	}
	fmt.Println("valid!")

	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("loading credentials: %w", err)
	}

	creds.Anthropic = &auth.APIKeyCredentials{APIKey: apiKey}

	if err := auth.Save(creds); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Println("Anthropic credentials stored successfully!")
	return nil
}

func runAuthOpenAI(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("OpenAI API key: ")
	input, _ := reader.ReadString('\n')
	apiKey := strings.TrimSpace(input)
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("loading credentials: %w", err)
	}

	creds.OpenAI = &auth.APIKeyCredentials{APIKey: apiKey}

	if err := auth.Save(creds); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Println("OpenAI credentials stored successfully!")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("loading credentials: %w", err)
	}

	path, _ := auth.CredentialPath()
	fmt.Printf("Credentials file: %s\n\n", path)

	fmt.Println("Provider     Status")
	fmt.Println("--------     ------")

	// Anthropic
	if env := os.Getenv("ANTHROPIC_API_KEY"); env != "" {
		fmt.Println("anthropic    configured (env var)")
	} else if creds.Anthropic != nil && creds.Anthropic.APIKey != "" {
		fmt.Println("anthropic    configured (stored)")
	} else {
		fmt.Println("anthropic    not configured")
	}

	// OpenAI
	if env := os.Getenv("OPENAI_API_KEY"); env != "" {
		fmt.Println("openai       configured (env var)")
	} else if creds.OpenAI != nil && creds.OpenAI.APIKey != "" {
		fmt.Println("openai       configured (stored)")
	} else {
		fmt.Println("openai       not configured")
	}

	// Google
	if env := os.Getenv("GOOGLE_API_KEY"); env != "" {
		fmt.Println("google       configured (env var: API key)")
	} else if creds.Google != nil && creds.Google.RefreshToken != "" {
		fmt.Println("google       configured (stored: OAuth2)")
	} else {
		fmt.Println("google       not configured")
	}

	// Ollama (always available locally)
	fmt.Println("ollama       available (local)")

	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("loading credentials: %w", err)
	}

	if len(args) == 0 {
		// Remove all credentials.
		creds = &auth.Credentials{}
		fmt.Println("All stored credentials removed.")
	} else {
		switch args[0] {
		case "google":
			creds.Google = nil
			fmt.Println("Google credentials removed.")
		case "anthropic":
			creds.Anthropic = nil
			fmt.Println("Anthropic credentials removed.")
		case "openai":
			creds.OpenAI = nil
			fmt.Println("OpenAI credentials removed.")
		default:
			return fmt.Errorf("unknown provider %q (valid: google, anthropic, openai)", args[0])
		}
	}

	return auth.Save(creds)
}

func verifyAnthropicKey(apiKey string) error {
	// Send a minimal request to check the key is valid.
	body := strings.NewReader(`{"model":"claude-sonnet-4-20250514","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.anthropic.com/v1/messages", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid API key (401 Unauthorized)")
	}
	// Any other status (200, 429, etc.) means the key is valid.
	return nil
}
