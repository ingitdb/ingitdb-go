package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHTTPHandler_DefaultHost_RoutesAPI(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler([]string{"api.ingitdb.com"}, []string{"mcp.ingitdb.com"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "www.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Unknown hosts are routed to API handler, which serves index.html (200).
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for default host (routed to API), got %d", w.Code)
	}
}

func TestNewHTTPHandler_APIHost_RoutesAPI(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler([]string{"api.ingitdb.com"}, []string{"mcp.ingitdb.com"})
	// API paths now require auth, so requests without token should return 401.
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	req.Host = "api.ingitdb.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from API handler, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewHTTPHandler_MCPHost_RoutesMCP(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler([]string{"api.ingitdb.com"}, []string{"mcp.ingitdb.com"})
	// A GET to /mcp should return 405 Method Not Allowed (httprouter only allows
	// POST for /mcp), confirming routing reaches the MCP handler.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Host = "mcp.ingitdb.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 from MCP handler (GET not allowed), got %d", w.Code)
	}
}

func TestNewHTTPHandler_APIHostWithPort(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler([]string{"api.ingitdb.com"}, []string{"mcp.ingitdb.com"})
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	req.Host = "api.ingitdb.com:443"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// The port should be stripped and routed to API, then auth check should return 401.
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from API handler (host with port), got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewHTTPHandler_Localhost_AllowsUnauthenticatedAPI(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler([]string{"localhost"}, []string{"mcp.ingitdb.com"})
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	req.Host = "localhost"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from API handler in localhost mode, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServeHTTP_RequiresAuthConfig(t *testing.T) {
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_ID", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_SECRET", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CALLBACK_URL", "")
	t.Setenv("INGITDB_AUTH_COOKIE_DOMAIN", "")
	t.Setenv("INGITDB_AUTH_API_BASE_URL", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := serveHTTP(ctx, "0", []string{"api.ingitdb.com"}, []string{"mcp.ingitdb.com"}, func(...any) {})
	if err == nil {
		t.Fatal("expected error for missing auth config")
	}
	if !strings.Contains(err.Error(), "invalid auth config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServeHTTP_LocalhostMode_DoesNotRequireAuthConfig(t *testing.T) {
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_ID", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_SECRET", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CALLBACK_URL", "")
	t.Setenv("INGITDB_AUTH_COOKIE_DOMAIN", "")
	t.Setenv("INGITDB_AUTH_API_BASE_URL", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := serveHTTP(ctx, "0", []string{"localhost"}, []string{"localhost"}, func(...any) {})
	if err != nil {
		t.Fatalf("expected no error in localhost mode, got: %v", err)
	}
}

func TestRequiresAuth(t *testing.T) {
	t.Parallel()
	if requiresAuth(nil) {
		t.Fatal("expected no auth requirement when api-domains is not set")
	}
	if requiresAuth([]string{"localhost"}) {
		t.Fatal("expected no auth requirement for localhost")
	}
	if !requiresAuth([]string{"api.ingitdb.com"}) {
		t.Fatal("expected auth requirement for non-localhost api domain")
	}
}
