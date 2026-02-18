package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GoogleCredentials stores OAuth2 tokens for Google API access.
type GoogleCredentials struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenExpiry  string `json:"token_expiry,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// APIKeyCredentials stores an API key for a provider.
type APIKeyCredentials struct {
	APIKey string `json:"api_key,omitempty"`
}

// Credentials holds stored credentials for all providers.
type Credentials struct {
	Google    *GoogleCredentials `json:"google,omitempty"`
	Anthropic *APIKeyCredentials `json:"anthropic,omitempty"`
	OpenAI    *APIKeyCredentials `json:"openai,omitempty"`
	MiniMax   *APIKeyCredentials `json:"minimax,omitempty"`
}

// CredentialPath returns the path to the credentials file (~/.autodoc/credentials.json).
func CredentialPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".autodoc", "credentials.json"), nil
}

// Load reads credentials from ~/.autodoc/credentials.json.
// Returns empty credentials if the file doesn't exist.
func Load() (*Credentials, error) {
	path, err := CredentialPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{}, nil
		}
		return nil, fmt.Errorf("reading credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}
	return &creds, nil
}

// Save writes credentials to ~/.autodoc/credentials.json with restricted permissions.
func Save(creds *Credentials) error {
	path, err := CredentialPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating credentials directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return nil
}

// GetAPIKey returns the API key for the given provider.
// It checks the environment variable first, then falls back to stored credentials.
func GetAPIKey(provider string) string {
	// Priority 1: Environment variable.
	switch provider {
	case "anthropic":
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return key
		}
	case "openai":
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return key
		}
	case "google":
		if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
			return key
		}
	case "minimax":
		if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
			return key
		}
	}

	// Priority 2: Stored credentials.
	creds, err := Load()
	if err != nil {
		return ""
	}

	switch provider {
	case "anthropic":
		if creds.Anthropic != nil {
			return creds.Anthropic.APIKey
		}
	case "openai":
		if creds.OpenAI != nil {
			return creds.OpenAI.APIKey
		}
	case "google":
		// For Google, API key auth is separate from OAuth.
		// This only returns the API key, not OAuth tokens.
		return ""
	case "minimax":
		if creds.MiniMax != nil {
			return creds.MiniMax.APIKey
		}
	}

	return ""
}

// HasGoogleOAuth returns true if Google OAuth credentials are stored.
func HasGoogleOAuth() bool {
	creds, err := Load()
	if err != nil {
		return false
	}
	return creds.Google != nil && creds.Google.RefreshToken != ""
}
