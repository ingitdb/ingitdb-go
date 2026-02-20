package auth

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultCookieName   = "ingitdb_github_token"
	defaultCookieSecure = true
)

// Config configures OAuth and shared auth cookie behavior for HTTP API/MCP servers.
type Config struct {
	GitHubClientID     string
	GitHubClientSecret string
	CallbackURL        string
	Scopes             []string
	CookieDomain       string
	CookieName         string
	CookieSecure       bool
	AuthAPIBaseURL     string
}

// LoadConfigFromEnv loads authentication settings from environment variables.
func LoadConfigFromEnv() Config {
	cfg := Config{
		GitHubClientID:     strings.TrimSpace(os.Getenv("INGITDB_GITHUB_OAUTH_CLIENT_ID")),
		GitHubClientSecret: strings.TrimSpace(os.Getenv("INGITDB_GITHUB_OAUTH_CLIENT_SECRET")),
		CallbackURL:        strings.TrimSpace(os.Getenv("INGITDB_GITHUB_OAUTH_CALLBACK_URL")),
		CookieDomain:       strings.TrimSpace(os.Getenv("INGITDB_AUTH_COOKIE_DOMAIN")),
		CookieName:         strings.TrimSpace(os.Getenv("INGITDB_AUTH_COOKIE_NAME")),
		AuthAPIBaseURL:     strings.TrimSpace(os.Getenv("INGITDB_AUTH_API_BASE_URL")),
		Scopes: []string{
			"public_repo",
			"read:user",
		},
		CookieSecure: defaultCookieSecure,
	}
	cookieSecure := strings.TrimSpace(os.Getenv("INGITDB_AUTH_COOKIE_SECURE"))
	if cookieSecure != "" {
		secure, err := strconv.ParseBool(cookieSecure)
		if err == nil {
			cfg.CookieSecure = secure
		}
	}
	if cfg.CookieName == "" {
		cfg.CookieName = defaultCookieName
	}
	return cfg
}

// ValidateForHTTPMode validates required auth settings before HTTP server startup.
func (c Config) ValidateForHTTPMode() error {
	if c.GitHubClientID == "" {
		return fmt.Errorf("INGITDB_GITHUB_OAUTH_CLIENT_ID is required")
	}
	if c.GitHubClientSecret == "" {
		return fmt.Errorf("INGITDB_GITHUB_OAUTH_CLIENT_SECRET is required")
	}
	if c.CallbackURL == "" {
		return fmt.Errorf("INGITDB_GITHUB_OAUTH_CALLBACK_URL is required")
	}
	if c.CookieDomain == "" {
		return fmt.Errorf("INGITDB_AUTH_COOKIE_DOMAIN is required")
	}
	if c.AuthAPIBaseURL == "" {
		return fmt.Errorf("INGITDB_AUTH_API_BASE_URL is required")
	}
	return nil
}
