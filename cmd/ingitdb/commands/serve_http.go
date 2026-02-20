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
func newHTTPHandler() http.Handler {
	apiHandler := api.NewHandler()
	mcpHandler := mcp.NewHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		switch host {
		case "api.ingitdb.com":
			apiHandler.ServeHTTP(w, r)
		case "mcp.ingitdb.com":
			mcpHandler.ServeHTTP(w, r)
		default:
			// Static files will be served by Firebase hosting
			// http.ServeFile(w, r, "static/index.html")
			http.NotFound(w, r)
		}
	})
}

// serveHTTP starts the HTTP API server on port and blocks until ctx is done.
func serveHTTP(ctx context.Context, port string, logf func(...any)) error {
	_ = logf
	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: newHTTPHandler()}
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
