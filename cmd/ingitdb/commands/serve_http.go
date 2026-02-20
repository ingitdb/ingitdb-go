package commands

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/ingitdb/ingitdb-cli/server/api"
	"github.com/ingitdb/ingitdb-cli/server/mcp"
)

// newHTTPHandler returns an http.Handler that dispatches requests based on the Host header.
// apiDomains specifies which hosts route to the API handler.
// mcpDomains specifies which hosts route to the MCP handler.
func newHTTPHandler(apiDomains, mcpDomains []string) http.Handler {
	apiHandler := api.NewHandler()
	mcpHandler := mcp.NewHandler()

	apiDomainMap := make(map[string]bool)
	for _, d := range apiDomains {
		apiDomainMap[d] = true
	}

	mcpDomainMap := make(map[string]bool)
	for _, d := range mcpDomains {
		mcpDomainMap[d] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		if apiDomainMap[host] {
			apiHandler.ServeHTTP(w, r)
		} else if mcpDomainMap[host] {
			mcpHandler.ServeHTTP(w, r)
		} else {
			// Static files will be served by Firebase hosting
			http.NotFound(w, r)
		}
	})
}

// serveHTTP starts the HTTP API server on port and blocks until ctx is done.
// apiDomains specifies which hosts route to the API handler.
// mcpDomains specifies which hosts route to the MCP handler.
func serveHTTP(ctx context.Context, port string, apiDomains, mcpDomains []string, logf func(...any)) error {
	_ = logf
	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: newHTTPHandler(apiDomains, mcpDomains)}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}
