package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/ingitdb/ingitdb-cli/server/api"
	"github.com/ingitdb/ingitdb-cli/server/mcp"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, newHandler()))
}

// newHandler returns an http.Handler that dispatches requests based on the Host header.
func newHandler() http.Handler {
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
			http.ServeFile(w, r, "static/index.html")
		}
	})
}
