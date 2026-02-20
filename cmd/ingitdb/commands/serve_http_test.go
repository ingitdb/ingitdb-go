package commands

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHTTPHandler_DefaultHost_Returns404(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "www.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Static files are served by Firebase hosting, so default host should return 404.
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for default host, got %d", w.Code)
	}
}

func TestNewHTTPHandler_APIHost_RoutesAPI(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler()
	// An invalid request to a valid API path without required params should
	// return 400 (Bad Request), not 404, confirming routing works.
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	req.Host = "api.ingitdb.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from API handler, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewHTTPHandler_MCPHost_RoutesMCP(t *testing.T) {
	t.Parallel()
	handler := newHTTPHandler()
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
	handler := newHTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	req.Host = "api.ingitdb.com:443"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// The port should be stripped and routed to API (returning 400 for missing db param).
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from API handler (host with port), got %d: %s", w.Code, w.Body.String())
	}
}
