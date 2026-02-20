# Copyright 2026 Sneat Co
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy pre-built binary from GitHub Actions
COPY ingitdb .

# Copy static files
COPY server/static ./static

# Set environment variable for port
ENV PORT=8080

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./ingitdb"]
