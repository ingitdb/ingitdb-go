FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy pre-built binary from GitHub Actions
COPY ingitdb .

# Expose the port
EXPOSE 8080

# Run the HTTP server with API and MCP handlers
CMD ["./ingitdb", "serve", "--http", "--http-port=8080", "--api-domains=api.ingitdb.com", "--mcp-domains=mcp.ingitdb.com"]
