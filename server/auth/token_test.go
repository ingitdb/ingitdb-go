package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveTokenFromRequest_PrefersBearerHeader(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "cookie-token"})

	got := ResolveTokenFromRequest(req, "ingitdb_github_token")
	if got != "header-token" {
		t.Fatalf("expected header token, got %q", got)
	}
}

func TestResolveTokenFromRequest_FallsBackToCookie(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "cookie-token"})

	got := ResolveTokenFromRequest(req, "ingitdb_github_token")
	if got != "cookie-token" {
		t.Fatalf("expected cookie token, got %q", got)
	}
}
