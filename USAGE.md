# I2P Docker Network Plugin Usage Guide

This guide provides comprehensive instructions for installing, configuring, and using the I2P Docker Network Plugin to create anonymized container networks.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Basic Usage](#basic-usage)
- [Advanced Configuration](#advanced-configuration)
- [Service Exposure](#service-exposure)
- [Traffic Filtering](#traffic-filtering)
- [Monitoring and Logging](#monitoring-and-logging)
- [Use Cases](#use-cases)

## Prerequisites

Before using the I2P Docker Network Plugin, ensure you have:

### System Requirements

- **Docker Engine**: Version 20.10 or later with plugin support
- **I2P Router**: Running with SAM bridge enabled
- **Linux Environment**: iptables support for traffic filtering
- **Network Access**: Ability to reach I2P router on localhost:7656 (default)

### I2P Router Setup

1. **Install I2P Router**:

   ```bash
   # Ubuntu/Debian
   sudo apt-get update
   sudo apt-get install i2p
   
   # Or download from https://geti2p.net/
   ```

2. **Enable SAM Bridge**:
   - Access I2P router console: <http://127.0.0.1:7657/>
   - Navigate to "I2P Services" → "Clients"
   - Enable "SAM application bridge"
   - Verify SAM bridge is listening on localhost:7656

3. **Verify I2P Connectivity**:

   ```bash
   # Test SAM bridge connectivity
   telnet localhost 7656
   # Should connect successfully
   ```

## Installation

### Method 1: Docker Plugin Install (Production)

> **Note**: This method is for future releases when the plugin is distributed via Docker Hub.

```bash
# Install plugin from Docker Hub (future)
docker plugin install go-i2p/i2p-network-plugin:latest

# Enable the plugin
docker plugin enable go-i2p/i2p-network-plugin:latest
```

### Method 2: Manual Build and Install (Development)

```bash
# Clone the repository
git clone https://github.com/go-i2p/go-docker-network-i2p.git
cd go-docker-network-i2p

# Build the plugin binary
make build

# Install as Docker plugin (requires plugin manifest)
sudo mkdir -p /run/docker/plugins
sudo cp bin/i2p-network-plugin /usr/local/bin/
sudo cp plugin.json /etc/docker/plugins/i2p.json

# Start the plugin daemon
sudo i2p-network-plugin -sock /run/docker/plugins/i2p-network.sock
```

### Method 3: Development Testing

```bash
# Build and run locally for testing
make build

# Test plugin functionality
./bin/i2p-network-plugin -version
./bin/i2p-network-plugin -help
```

## Quick Start

### 1. Create an I2P Network

```bash
# Create a basic I2P network
docker network create --driver=i2p my-i2p-network

# Verify network creation
docker network ls | grep i2p
```

### 2. Run a Container with I2P Network

```bash
# Start a web server container on I2P network
docker run -d --name my-web-server \
  --network my-i2p-network \
  -p 80:80 \
  nginx:alpine

# Container traffic will be automatically routed through I2P
```

### 3. Access I2P Services

```bash
# Check container I2P address (when service exposure is configured)
docker logs my-web-server

# Look for log entries showing .b32.i2p addresses
# Example: "Service exposed on: abc123...def.b32.i2p"
```

## Basic Usage

### Creating Networks

```bash
# Create I2P network with default settings
docker network create --driver=i2p basic-i2p

# Create I2P network with custom subnet
docker network create --driver=i2p \
  --subnet=192.168.100.0/24 \
  --gateway=192.168.100.1 \
  advanced-i2p

# Create multiple isolated I2P networks
docker network create --driver=i2p app-network
docker network create --driver=i2p db-network
```

### Running Containers

```bash
# Run container with automatic I2P integration
docker run -d --name app-container \
  --network app-network \
  my-app:latest

# Run container with exposed service ports
docker run -d --name web-server \
  --network app-network \
  --expose 80 \
  --expose 443 \
  nginx:alpine

# Run container with environment-based port configuration
docker run -d --name api-server \
  --network app-network \
  -e PORT=8080 \
  -e API_PORT=8081 \
  my-api:latest
```

### Container-to-Container Communication

```bash
# Create network and containers
docker network create --driver=i2p microservices

# Frontend service
docker run -d --name frontend \
  --network microservices \
  --expose 3000 \
  frontend:latest

# Backend API
docker run -d --name backend-api \
  --network microservices \
  --expose 8080 \
  backend:latest

# Database (isolated, no external exposure)
docker run -d --name database \
  --network microservices \
  postgres:alpine

# Containers can communicate using container names
# Frontend can reach: http://backend-api:8080
# Backend can reach: postgresql://database:5432
```

## Advanced Configuration

### Environment Variable Configuration

```bash
# Run plugin with custom I2P router settings
I2P_SAM_HOST=192.168.1.100 \
I2P_SAM_PORT=7656 \
I2P_SAM_TIMEOUT=30 \
./bin/i2p-network-plugin

# Configure tunnel parameters
I2P_INBOUND_TUNNELS=3 \
I2P_OUTBOUND_TUNNELS=3 \
I2P_INBOUND_LENGTH=3 \
I2P_OUTBOUND_LENGTH=3 \
./bin/i2p-network-plugin

# Enable debugging and detailed logging
I2P_DEBUG=true \
I2P_LOG_LEVEL=debug \
./bin/i2p-network-plugin
```

### Network-Specific Configuration

```bash
# Create network with custom driver options
docker network create --driver=i2p \
  --opt i2p.sam.host=192.168.1.100 \
  --opt i2p.sam.port=7656 \
  --opt i2p.tunnels.inbound=5 \
  --opt i2p.tunnels.outbound=5 \
  production-network

# Create network with traffic filtering enabled
docker network create --driver=i2p \
  --opt i2p.filter.enabled=true \
  --opt i2p.filter.allowlist="example.i2p,*.trusted.i2p" \
  --opt i2p.filter.blocklist="*.malicious.i2p" \
  filtered-network
```

## Service Exposure

### Automatic Service Discovery

The plugin automatically detects and exposes container services through multiple methods:

```bash
# Method 1: Docker EXPOSE directive in Dockerfile
FROM nginx:alpine
EXPOSE 80 443
# Plugin will create I2P server tunnels for ports 80 and 443

# Method 2: Environment variables
docker run -d --name web-app \
  --network i2p-net \
  -e PORT=3000 \
  -e HTTP_PORT=8080 \
  -e HTTPS_PORT=8443 \
  web-app:latest
# Plugin detects PORT, HTTP_PORT, HTTPS_PORT variables

# Method 3: Docker port mappings (requires allow_ip=true)
# Create network with IP exposure enabled
docker network create --driver=i2p --opt allow_ip=true dev-net

docker run -d --name api \
  --network dev-net \
  -p 8080:8080 \
  -p 8443:8443 \
  api-server:latest
# Plugin creates localhost port forwarders for mapped ports
# Accessible via: http://localhost:8080 and https://localhost:8443
```

### Manual Service Configuration

```bash
# Create container with explicit service exposure
docker run -d --name custom-service \
  --network i2p-net \
  --label i2p.expose.web=80 \
  --label i2p.expose.api=8080 \
  --label i2p.expose.admin=9090 \
  custom-app:latest

# Check exposed services
docker exec custom-service cat /tmp/i2p-services.json
```

### I2P Address Generation

```bash
# View container's I2P service addresses
docker logs custom-service | grep "\.b32\.i2p"

# Example output:
# [INFO] Service 'web' exposed on: abc123def456.b32.i2p:80
# [INFO] Service 'api' exposed on: abc123def456.b32.i2p:8080
# [INFO] Service 'admin' exposed on: abc123def456.b32.i2p:9090
```

### Selective Port Exposure

The plugin supports flexible port exposure, allowing you to expose services either to the I2P network or to specific IP addresses:

#### Container Label-Based Configuration

```bash
# Expose port 80 to I2P and port 443 to localhost
docker run -d --name hybrid-service \
  --network my-i2p-network \
  --label i2p.expose.80=i2p \
  --label i2p.expose.443=ip:127.0.0.1 \
  nginx:alpine

# Expose multiple ports with different configurations
docker run -d --name multi-expose \
  --network my-i2p-network \
  --label i2p.expose.8080=i2p \
  --label i2p.expose.9090=ip:0.0.0.0 \
  --label i2p.expose.3000=ip:192.168.1.100 \
  my-app:latest

# IP exposure with default localhost (when IP not specified)
docker run -d --name local-service \
  --network my-i2p-network \
  --label i2p.expose.5000=ip \
  api-server:latest
# Port 5000 exposed on 127.0.0.1:5000
```

#### Network-Level Configuration

```bash
# Create network with IP exposure as default
docker network create --driver=i2p \
  --opt i2p.exposure.default=ip \
  --opt i2p.exposure.allow_ip=true \
  development-network

# All ports will default to IP exposure unless explicitly configured
docker run -d --name dev-app \
  --network development-network \
  --expose 3000 \
  node-app:latest
# Port 3000 exposed on 127.0.0.1:3000 (network default)

# Create network with IP exposure disabled (I2P only)
docker network create --driver=i2p \
  --opt i2p.exposure.allow_ip=false \
  secure-i2p-network

# Any IP exposure requests will be forced to I2P
docker run -d --name secure-service \
  --network secure-i2p-network \
  --label i2p.expose.80=ip:0.0.0.0 \
  web-app:latest
# Port 80 forced to I2P despite IP label (policy enforced)
```

#### Use Cases for Selective Exposure

**Development and Testing**:
```bash
# Create development network with mixed exposure
docker network create --driver=i2p \
  --opt i2p.exposure.default=ip \
  dev-network

# Frontend exposed locally for development
docker run -d --name frontend-dev \
  --network dev-network \
  --label i2p.expose.3000=ip:127.0.0.1 \
  --label i2p.expose.3001=ip:127.0.0.1 \
  -v $(pwd)/frontend:/app \
  frontend:dev

# Backend exposed on I2P for testing
docker run -d --name backend-dev \
  --network dev-network \
  --label i2p.expose.8080=i2p \
  backend:latest

# Access frontend locally: http://localhost:3000
# Access backend via I2P: http://<generated>.b32.i2p:8080
```

**Hybrid Deployments**:
```bash
# Create network allowing both exposure types
docker network create --driver=i2p hybrid-network

# Public API on I2P, admin interface on localhost
docker run -d --name api-server \
  --network hybrid-network \
  --label i2p.expose.8080=i2p \
  --label i2p.expose.9090=ip:127.0.0.1 \
  api-app:latest

# Public API accessible via I2P: http://<generated>.b32.i2p:8080
# Admin interface local only: http://localhost:9090
```

**Multi-Interface Exposure**:
```bash
# Expose service on specific network interfaces
docker run -d --name multi-interface \
  --network my-i2p-network \
  --label i2p.expose.80=i2p \
  --label i2p.expose.443=ip:192.168.1.100 \
  --label i2p.expose.8080=ip:10.0.0.50 \
  web-app:latest

# Port 80: I2P network
# Port 443: 192.168.1.100:443
# Port 8080: 10.0.0.50:8080
```

#### IPv6 Support

```bash
# Expose service on IPv6 address
docker run -d --name ipv6-service \
  --network my-i2p-network \
  --label i2p.expose.8080=ip:::1 \
  --label i2p.expose.8443=ip:fe80::1 \
  app:latest

# Port 8080 exposed on ::1 (IPv6 localhost)
# Port 8443 exposed on fe80::1 (link-local)
```

#### Priority and Precedence

Port exposure configuration follows this priority order:

1. **Container Labels** (`i2p.expose.*`) - Highest priority
2. **Docker EXPOSE Directives** - Medium priority  
3. **Environment Variables** (`PORT`, `HTTP_PORT`, etc.) - Lowest priority

```bash
# Container with multiple port sources
docker run -d --name priority-test \
  --network my-i2p-network \
  --label i2p.expose.80=ip:127.0.0.1 \
  --expose 80 \
  --expose 443 \
  -e PORT=8080 \
  nginx:alpine

# Result:
# Port 80: ip:127.0.0.1 (from label - highest priority)
# Port 443: I2P (from EXPOSE - defaults to I2P)
# Port 8080: I2P (from ENV - defaults to I2P)
```

## Traffic Filtering

### Allowlist Configuration

```bash
# Create network with allowlist filtering
docker network create --driver=i2p \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowed="trusted.i2p,*.example.i2p,abc123.b32.i2p" \
  allowlist-network

# Only specified I2P destinations will be accessible
# All other traffic will be blocked and logged
```

### Blocklist Configuration

```bash
# Create network with blocklist filtering
docker network create --driver=i2p \
  --opt i2p.filter.mode=blocklist \
  --opt i2p.filter.blocked="malicious.i2p,*.spam.i2p" \
  blocklist-network

# All I2P traffic allowed except blocked destinations
# Non-I2P traffic is blocked by default
```

### Dynamic Filter Management

```bash
# Add/remove destinations from running network filters
# (Requires management API - future feature)

# Add to allowlist
curl -X POST http://localhost:8080/api/v1/networks/my-network/filter/allow \
  -d '{"destination": "new-service.i2p"}'

# Add to blocklist  
curl -X POST http://localhost:8080/api/v1/networks/my-network/filter/block \
  -d '{"destination": "bad-actor.i2p"}'

# Remove from lists
curl -X DELETE http://localhost:8080/api/v1/networks/my-network/filter/allow/trusted.i2p
```

## Monitoring and Logging

### Plugin Logs

```bash
# View plugin logs (systemd)
sudo journalctl -u i2p-network-plugin -f

# View plugin logs (manual run)
./bin/i2p-network-plugin -debug 2>&1 | tee plugin.log

# Filter specific events
sudo journalctl -u i2p-network-plugin | grep "TRAFFIC BLOCK"
sudo journalctl -u i2p-network-plugin | grep "Service exposed"
```

### Container Network Status

```bash
# Check container I2P network configuration
docker inspect container-name | jq '.NetworkSettings.Networks'

# View container I2P tunnels
docker exec container-name netstat -tlnp | grep 127.0.0.1

# Check I2P service status
docker exec container-name cat /proc/net/tcp | grep "00000000:1080"
```

### Traffic Analysis

```bash
# Monitor I2P traffic patterns
sudo journalctl -u i2p-network-plugin | grep "TRAFFIC" | tail -50

# Analyze blocked connections
sudo journalctl -u i2p-network-plugin | grep "TRAFFIC BLOCK" | \
  awk '{print $NF}' | sort | uniq -c | sort -nr

# Monitor service exposures
sudo journalctl -u i2p-network-plugin | grep "Service exposed" | \
  grep -o "[a-z0-9]*\.b32\.i2p" | sort | uniq
```

## Use Cases

### 1. Anonymous Web Services

```bash
# Host anonymous website
docker network create --driver=i2p web-network

docker run -d --name anonymous-blog \
  --network web-network \
  --expose 80 \
  -v $(pwd)/blog:/usr/share/nginx/html:ro \
  nginx:alpine

# Blog accessible via generated .b32.i2p address
```

### 2. Secure Microservices

```bash
# Create isolated microservice architecture
docker network create --driver=i2p \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowed="*.internal.i2p" \
  microservices

# API Gateway
docker run -d --name api-gateway \
  --network microservices \
  --expose 8080 \
  --label i2p.hostname=gateway.internal.i2p \
  gateway:latest

# User Service
docker run -d --name user-service \
  --network microservices \
  --expose 9000 \
  --label i2p.hostname=users.internal.i2p \
  user-service:latest

# Payment Service
docker run -d --name payment-service \
  --network microservices \
  --expose 9001 \
  --label i2p.hostname=payments.internal.i2p \
  payment-service:latest
```

### 3. Development Environment

```bash
# Development network with full access
docker network create --driver=i2p \
  --opt i2p.filter.mode=disabled \
  dev-network

# Development containers can access any I2P service
docker run -d --name dev-app \
  --network dev-network \
  -v $(pwd):/workspace \
  -e NODE_ENV=development \
  node:alpine sh -c "npm install && npm start"
```

### 4. Tor-like Hidden Services

```bash
# Create network for hidden services
docker network create --driver=i2p hidden-services

# Chat application
docker run -d --name anon-chat \
  --network hidden-services \
  --expose 3000 \
  chat-app:latest

# File sharing service
docker run -d --name anon-files \
  --network hidden-services \
  --expose 8080 \
  -v $(pwd)/shared:/data \
  file-server:latest

# Services accessible only via I2P network
```

## Best Practices

### Security

1. **Use traffic filtering** in production environments
2. **Regularly rotate I2P keys** by recreating containers
3. **Monitor traffic logs** for suspicious activity
4. **Isolate sensitive services** in separate networks
5. **Use allowlist mode** for critical applications

### Performance

1. **Configure appropriate tunnel counts** based on load
2. **Use persistent volumes** for I2P key storage
3. **Monitor I2P router performance** and connectivity
4. **Consider I2P router clustering** for high availability
5. **Optimize container placement** for network locality

### Debugging

1. **Enable debug logging** during development
2. **Use network inspection tools** to verify configuration
3. **Test I2P connectivity** before deploying containers
4. **Monitor SAM bridge status** regularly
5. **Keep logs of service exposures** for troubleshooting

## Next Steps

- See [CONFIG.md](CONFIG.md) for detailed configuration reference
- See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for issue resolution
- Check [DISTRIBUTION.md](DISTRIBUTION.md) for packaging and distribution options
- Review [GitHub Issues](https://github.com/go-i2p/go-docker-network-i2p/issues) for current development status

## Support

For questions and support:

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/go-i2p/go-docker-network-i2p/issues)
- **Documentation**: Review all `.md` files in the repository
- **I2P Community**: [I2P Community Forums](https://i2pforum.net/)
- **Docker Networking**: [Docker Network Plugin Documentation](https://docs.docker.com/engine/extend/plugins_network/)
