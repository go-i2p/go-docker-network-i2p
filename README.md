Anonymizing Docker Network Plugin
=================================

A Docker network plugin that provides transparent I2P connectivity for containers, enabling anonymous and secure containerized services.

## Features

✅ **Complete I2P Integration**: Transparent I2P connectivity for Docker containers  
✅ **Automatic Service Exposure**: Generate .b32.i2p addresses for container services (via `--expose`)  
✅ **Traffic Filtering**: Block non-I2P traffic with allowlist/blocklist support  
✅ **Container Isolation**: Separate I2P identity per container for security  
✅ **Docker Plugin API v2**: Full CNM compliance (uses .b32.i2p addresses instead of host port mappings)  
🔄 **Beta - Near Production Ready**: Comprehensive testing and configuration options

**Note**: I2P networks don't support `-p` port mappings (e.g., `-p 8080:80`) since services are exposed via .b32.i2p addresses, not host ports. Use `--expose` to expose container ports to the I2P network.  

## Quick Start

### Prerequisites

1. **I2P Router** with SAM bridge enabled on localhost:7656
2. **Docker Engine** 20.10+ with plugin support
3. **Linux system** with **iptables** (required for traffic filtering and interception)

⚠️ **Security Note**: iptables is mandatory for traffic filtering. Without it, non-I2P traffic may leak to clearnet, compromising anonymity. Network creation will fail if iptables is unavailable.

### Installation

```bash
# Clone and build
git clone https://github.com/go-i2p/go-docker-network-i2p.git
cd go-docker-network-i2p
make build

# Install plugin
sudo cp bin/i2p-network-plugin /usr/local/bin/
sudo mkdir -p /run/docker/plugins

# Start plugin daemon (uses default socket path)
sudo i2p-network-plugin
```

### Basic Usage

```bash
# Create I2P network
docker network create --driver=i2p my-i2p-network

# Run anonymous web service
docker run -d --name anonymous-web \
  --network my-i2p-network \
  --expose 80 \
  nginx:alpine

# Get service .b32.i2p address (recommended: use docker inspect)
docker inspect anonymous-web | jq -r '.NetworkSettings.Networks[].com.i2p.service.addresses'

# Alternative: check plugin logs
# If running plugin as system daemon (default installation):
sudo journalctl -u i2p-network-plugin | grep "exposed as"

# If running plugin as Docker container (via make docker-run):
docker logs i2p-network-plugin 2>&1 | grep "exposed as"
```

## Documentation

📖 **[USAGE.md](USAGE.md)** - Installation, configuration, and usage examples  
⚙️ **[CONFIG.md](CONFIG.md)** - Complete configuration reference  
🔧 **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Diagnostic and troubleshooting guide  
⚠️ **[KNOWN_ISSUES.md](KNOWN_ISSUES.md)** - Current limitations and known issues  
📦 **[DISTRIBUTION.md](DISTRIBUTION.md)** - Distribution and packaging guide  

## Retrieving Service Addresses

The plugin provides multiple ways to retrieve your container's I2P service addresses:

### Method 1: Docker Inspect (Recommended)

Use `docker inspect` to programmatically retrieve service addresses. This is the most reliable method for scripts and automation:

```bash
# Get all service addresses as JSON
docker inspect <container-name> | jq '.NetworkSettings.Networks[].com.i2p.service.addresses'

# Example output:
# {
#   "service-80": "abc123def456.b32.i2p"
# }

# Extract a specific service address
docker inspect <container-name> | jq -r '.NetworkSettings.Networks[].com.i2p.service.addresses["service-80"]'
```

### Method 2: Plugin Logs

Service addresses are also logged when containers start:

```bash
# For system daemon installations:
sudo journalctl -u i2p-network-plugin | grep "exposed as"

# For Docker container installations:
docker logs i2p-network-plugin 2>&1 | grep "exposed as"
```

**Note**: Log-based retrieval is less reliable for automation as logs may rotate or be unavailable. Use `docker inspect` for scripts and programmatic access.

## Architecture

The plugin implements a **one SAM connection per container** architecture for optimal isolation:

```
Docker Container 1          Docker Container 2          Docker Container N
       |                           |                           |
   SAM Client 1                SAM Client 2                SAM Client N
       |                           |                           |
  Primary Session 1          Primary Session 2          Primary Session N
       |                           |                           |
Sub-sessions:                Sub-sessions:                Sub-sessions:
- Stream (HTTP)              - Stream (HTTPS)             - Stream (SSH)
- Stream (API)               - Datagram (UDP)             - Raw (Custom)
- Server (Port 80)           - Server (Port 443)          - Server (Port 22)
```

### Key Components

```
cmd/i2p-network-plugin/    # Main plugin executable
pkg/plugin/                # Docker network plugin implementation (CNM)
pkg/i2p/                   # I2P SAM client and tunnel management
pkg/proxy/                 # Traffic interception and proxying
pkg/service/               # Automatic service exposure
internal/config/           # Internal configuration management
test/                      # Integration tests
```

### Security Design

- **Cryptographic Isolation**: Each container gets unique I2P destination keys
- **Traffic Filtering**: Block non-I2P traffic by default with configurable policies
- **No Traffic Leakage**: All container traffic routed through I2P network
- **Session Management**: Proper cleanup and key rotation on container lifecycle

## Use Cases

🌐 **Anonymous Web Services** - Host websites accessible only via I2P  
🔒 **Secure Microservices** - Internal service communication over I2P  
🛡️ **Privacy-First Applications** - Applications that never touch clearnet  
🧪 **Development/Testing** - Test I2P integration without exposing services  
📡 **Hidden APIs** - Provide APIs accessible only through I2P network  

## Quick Examples

### Anonymous Blog

```bash
# Create I2P network
docker network create --driver=i2p blog-network

# Run blog with automatic I2P exposure
docker run -d --name my-blog \
  --network blog-network \
  --expose 80 \
  -v $(pwd)/content:/usr/share/nginx/html:ro \
  nginx:alpine

# Blog accessible via .b32.i2p address
```

### Secure API Service

```bash
# Create filtered I2P network
docker network create --driver=i2p \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowlist="*.trusted.i2p" \
  secure-api

# Run API with traffic filtering
docker run -d --name secure-api \
  --network secure-api \
  -e PORT=8080 \
  my-secure-api:latest
```

### Development Environment

```bash
# Create development network
docker network create --driver=i2p \
  --opt i2p.filter.mode=disabled \
  --opt i2p.tunnels.inbound=1 \
  --opt i2p.tunnels.outbound=1 \
  dev-network

# Fast startup for development
docker run -d --name dev-app \
  --network dev-network \
  -v $(pwd):/workspace \
  node:alpine npm start
```

## Building and Testing

```bash
# Build the plugin
make build

# Run comprehensive test suite
make test

# View test coverage
make coverage

# Build with race detection
make test-race

# View all available targets (organized by category)
make help
```

## Configuration

The plugin supports multiple configuration methods:

### Environment Variables

```bash
# I2P router configuration
export I2P_SAM_HOST="localhost"
export I2P_SAM_PORT="7656"

# Tunnel optimization
export I2P_INBOUND_TUNNELS="3"
export I2P_OUTBOUND_TUNNELS="3"

# Debug mode
export DEBUG="true"
```

### Network Options

```bash
# Create network with custom settings
docker network create --driver=i2p \
  --opt i2p.sam.host=192.168.1.100 \
  --opt i2p.tunnels.inbound=5 \
  --opt i2p.filter.mode=allowlist \
  production-network
```

See [CONFIG.md](CONFIG.md) for complete configuration reference.

## Project Status

🟢 **Phase 1-4: Complete** - All core functionality implemented  
🔄 **Phase 5: In Progress** - Documentation and distribution  

### Completed Features

- ✅ **Project Foundation**: Go module, build system, and project structure
- ✅ **Docker Plugin Framework**: CNM compliance with all required endpoints
- ✅ **I2P Integration**: SAM client connectivity and session management
- ✅ **Container Isolation**: Individual SAM connections per container
- ✅ **Service Exposure**: Automatic I2P server tunnel creation
- ✅ **Traffic Proxying**: Transparent SOCKS and DNS proxying
- ✅ **Traffic Filtering**: Allowlist/blocklist with wildcard support
- ✅ **Testing Infrastructure**: Test suite with ~67% average coverage (Config: 89%, I2P: 78%, Service: 92%, Plugin: 28%, Proxy: 47%)

## Contributing

We welcome contributions! Please see:

- **Issues**: [GitHub Issues](https://github.com/go-i2p/go-docker-network-i2p/issues)
- **Development**: Check GitHub Issues, Milestones, and [KNOWN_ISSUES.md](KNOWN_ISSUES.md) for current development status
- **Testing**: Run `make test` to verify changes
- **Documentation**: Update relevant `.md` files for new features

## Support

📚 **Documentation**: Start with [USAGE.md](USAGE.md) for comprehensive guides  
🐛 **Bug Reports**: [GitHub Issues](https://github.com/go-i2p/go-docker-network-i2p/issues)  
💬 **Community**: [I2P Community Forums](https://i2pforum.net/)  
🔧 **Troubleshooting**: [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for diagnostic help  

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Security Notice

This software is in active development. While functional, please review the security considerations in [KNOWN_ISSUES.md](KNOWN_ISSUES.md) before production use.

⚠️ **Important**: Always verify your I2P router configuration and ensure proper traffic filtering for security-critical applications.
