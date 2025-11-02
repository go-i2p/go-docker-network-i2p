# Multi-stage build for I2P Docker Network Plugin
# Stage 1: Build the plugin binary
# Using golang:alpine (latest) to support go 1.24+ requirements
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the plugin binary with version information
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT
RUN make build VERSION=${VERSION} BUILD_TIME=${BUILD_TIME} GIT_COMMIT=${GIT_COMMIT}

# Stage 2: Create minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
# - ca-certificates: for HTTPS connections
# - iptables: for traffic interception
RUN apk add --no-cache ca-certificates iptables

# Create plugin directories
RUN mkdir -p /run/docker/plugins /var/lib/i2p-network-plugin

# Copy the plugin binary from builder
COPY --from=builder /build/bin/i2p-network-plugin /usr/local/bin/i2p-network-plugin

# Make binary executable
RUN chmod +x /usr/local/bin/i2p-network-plugin

# Copy plugin manifest
COPY --from=builder /build/plugin.json /etc/docker/plugins/i2p.json

# Set up non-root user for better security
RUN addgroup -g 1000 i2pplugin && \
    adduser -D -u 1000 -G i2pplugin i2pplugin && \
    chown -R i2pplugin:i2pplugin /var/lib/i2p-network-plugin

# Switch to non-root user (commented out for now as plugin needs root for iptables)
# Docker plugins typically need elevated privileges for network operations
# USER i2pplugin

# Plugin socket path
ENV PLUGIN_SOCKET=/run/docker/plugins/i2p-network.sock

# Expose plugin socket volume
VOLUME ["/run/docker/plugins"]

# Set working directory
WORKDIR /var/lib/i2p-network-plugin

# Health check to verify plugin is running
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD test -S ${PLUGIN_SOCKET} || exit 1

# Default command to run the plugin
ENTRYPOINT ["/usr/local/bin/i2p-network-plugin"]
CMD ["-sock", "/run/docker/plugins/i2p-network.sock"]
