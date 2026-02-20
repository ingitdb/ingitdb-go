package auth

import "testing"

func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_ID", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_SECRET", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CALLBACK_URL", "")
	t.Setenv("INGITDB_AUTH_COOKIE_DOMAIN", "")
	t.Setenv("INGITDB_AUTH_COOKIE_NAME", "")
	t.Setenv("INGITDB_AUTH_COOKIE_SECURE", "")
	t.Setenv("INGITDB_AUTH_API_BASE_URL", "")

	cfg := LoadConfigFromEnv()
	if cfg.CookieName != defaultCookieName {
		t.Fatalf("expected default cookie name %q, got %q", defaultCookieName, cfg.CookieName)
	}
	if !cfg.CookieSecure {
		t.Fatal("expected cookie secure default true")
	}
	if len(cfg.Scopes) != 2 || cfg.Scopes[0] != "public_repo" || cfg.Scopes[1] != "read:user" {
		t.Fatalf("unexpected scopes: %#v", cfg.Scopes)
	}
}

func TestLoadConfigFromEnv_ParsesCookieSecure(t *testing.T) {
	t.Setenv("INGITDB_AUTH_COOKIE_SECURE", "false")

	cfg := LoadConfigFromEnv()
	if cfg.CookieSecure {
		t.Fatal("expected cookie secure false")
	}
}

func TestValidateForHTTPMode(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	if err := cfg.ValidateForHTTPMode(); err == nil {
		t.Fatal("expected validation error")
	}
	cfg.GitHubClientID = "id"
	cfg.GitHubClientSecret = "secret"
	cfg.CallbackURL = "https://api.ingitdb.com/auth/github/callback"
	cfg.CookieDomain = ".ingitdb.com"
	cfg.AuthAPIBaseURL = "https://api.ingitdb.com"
	if err := cfg.ValidateForHTTPMode(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
