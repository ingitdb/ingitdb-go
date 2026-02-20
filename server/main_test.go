package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandler_DefaultHost_ServesStatic(t *testing.T) {
	t.Parallel()
	handler := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "www.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// ServeFile redirects for "/" to "/index.html" with a 301 or sends 200.
	// Either way the handler should not return 404.
	if w.Code == http.StatusNotFound {
		t.Fatalf("expected non-404 for default host, got %d", w.Code)
	}
}

func TestNewHandler_APIHost_RoutesAPI(t *testing.T) {
	t.Parallel()
	handler := newHandler()
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

func TestNewHandler_MCPHost_RoutesMCP(t *testing.T) {
	t.Parallel()
	handler := newHandler()
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

func TestNewHandler_APIHostWithPort(t *testing.T) {
	t.Parallel()
	handler := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	req.Host = "api.ingitdb.com:443"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// The port should be stripped and routed to API (returning 400 for missing db param).
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from API handler (host with port), got %d: %s", w.Code, w.Body.String())
	}
}
