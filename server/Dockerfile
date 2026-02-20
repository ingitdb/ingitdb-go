# Copyright 2026 Sneat Co
FROM gcr.io/distroless/static-debian12:nonroot

# WORKDIR /app is a conventional directory for application code.
# Cloud Run has no specific requirements for the working directory,
# but /app is widely used and clear in purpose. The binary and its
# dependencies will be placed here for execution.
WORKDIR /app

# Copy pre-built binary from GitHub Actions
COPY ingitdb .

# Expose the port
EXPOSE 8080

# Runtime auth configuration is provided by Cloud Run environment variables.
ENV INGITDB_GITHUB_OAUTH_CLIENT_ID=""
ENV INGITDB_GITHUB_OAUTH_CALLBACK_URL=""
ENV INGITDB_AUTH_COOKIE_DOMAIN=""
ENV INGITDB_AUTH_API_BASE_URL=""
ENV INGITDB_AUTH_COOKIE_NAME="ingitdb_github_token"
ENV INGITDB_AUTH_COOKIE_SECURE="true"

# Run the HTTP server with API and MCP handlers
CMD ["./ingitdb", "serve", "--http", "--http-port=8080", "--api-domains=api.ingitdb.com", "--mcp-domains=mcp.ingitdb.com"]
