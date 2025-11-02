# I2P Docker Network Plugin Configuration Reference

This document provides a comprehensive reference for configuring the I2P Docker Network Plugin.

## Table of Contents

- [Overview](#overview)
- [Command-Line Flags](#command-line-flags)
- [Environment Variables](#environment-variables)
- [Configuration File](#configuration-file)
- [Network Driver Options](#network-driver-options)
- [Validation Rules](#validation-rules)
- [Examples](#examples)

## Overview

The I2P Docker Network Plugin supports configuration through multiple methods:

1. **Command-line flags** - Used when starting the plugin daemon
2. **Environment variables** - Container-friendly configuration
3. **Configuration files** - Structured YAML/JSON configuration
4. **Docker network options** - Network-specific settings

Configuration precedence (highest to lowest):
1. Command-line flags
2. Environment variables  
3. Configuration files
4. Default values

## Command-Line Flags

The plugin daemon accepts the following command-line flags:

### Basic Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-sock` | string | `/run/docker/plugins/i2p-network.sock` | Unix socket path for plugin communication |
| `-debug` | bool | `false` | Enable debug logging |
| `-version` | bool | `false` | Show version and exit |

### Usage Examples

```bash
# Start plugin with custom socket path
./i2p-network-plugin -sock /var/run/docker/plugins/custom.sock

# Enable debug logging
./i2p-network-plugin -debug

# Show version information
./i2p-network-plugin -version
```

## Environment Variables

### Plugin Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `PLUGIN_SOCKET_PATH` | string | `/run/docker/plugins/i2p-network.sock` | Unix socket path for plugin communication |
| `DEBUG` | bool | `false` | Enable debug logging |
| `NETWORK_NAME` | string | `i2p` | Default name for I2P networks |
| `IPAM_SUBNET` | string | `172.20.0.0/16` | Default subnet for container IP allocation |
| `GATEWAY` | string | `172.20.0.1` | Default gateway IP for I2P networks |

### I2P SAM Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `I2P_SAM_HOST` | string | `localhost` | I2P SAM bridge hostname or IP address |
| `I2P_SAM_PORT` | int | `7656` | I2P SAM bridge port number |
| `I2P_SAM_TIMEOUT` | duration | `30s` | Connection timeout for SAM bridge |
| `I2P_SAM_USERNAME` | string | - | SAM authentication username (optional) |
| `I2P_SAM_PASSWORD` | string | - | SAM authentication password (optional) |

### I2P Tunnel Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `I2P_INBOUND_TUNNELS` | int | `2` | Number of inbound tunnels per session |
| `I2P_OUTBOUND_TUNNELS` | int | `2` | Number of outbound tunnels per session |
| `I2P_INBOUND_LENGTH` | int | `3` | Length (hops) of inbound tunnels |
| `I2P_OUTBOUND_LENGTH` | int | `3` | Length (hops) of outbound tunnels |
| `I2P_INBOUND_BACKUPS` | int | `1` | Number of backup inbound tunnels |
| `I2P_OUTBOUND_BACKUPS` | int | `1` | Number of backup outbound tunnels |
| `I2P_ENCRYPT_LEASESET` | bool | `false` | Enable leaseset encryption |
| `I2P_CLOSE_IDLE` | bool | `true` | Enable closing idle connections |
| `I2P_CLOSE_IDLE_TIME` | int | `10` | Idle timeout in minutes |

### Boolean Value Parsing

Boolean environment variables accept multiple formats:

**True values**: `true`, `1`, `yes`, `on`, `enable`, `enabled`
**False values**: `false`, `0`, `no`, `off`, `disable`, `disabled`

### Usage Examples

```bash
# Basic plugin configuration
export PLUGIN_SOCKET_PATH="/var/run/docker/plugins/i2p.sock"
export DEBUG="true"
export NETWORK_NAME="my-i2p"

# I2P router on different host
export I2P_SAM_HOST="192.168.1.100"
export I2P_SAM_PORT="7656"
export I2P_SAM_TIMEOUT="60s"

# High-security tunnel configuration
export I2P_INBOUND_TUNNELS="5"
export I2P_OUTBOUND_TUNNELS="5"
export I2P_INBOUND_LENGTH="5"
export I2P_OUTBOUND_LENGTH="5"
export I2P_ENCRYPT_LEASESET="true"

# Start plugin with environment configuration
./i2p-network-plugin
```

## Configuration File

The plugin supports structured configuration files in JSON format:

### Configuration Structure

```json
{
  "plugin": {
    "socket_path": "/run/docker/plugins/i2p-network.sock",
    "debug": false,
    "network_name": "i2p",
    "ipam_subnet": "172.20.0.0/16",
    "gateway": "172.20.0.1"
  },
  "sam": {
    "host": "localhost",
    "port": 7656,
    "timeout": "30s",
    "username": "",
    "password": ""
  },
  "tunnel_defaults": {
    "inbound_tunnels": 2,
    "outbound_tunnels": 2,
    "inbound_length": 3,
    "outbound_length": 3,
    "inbound_backups": 1,
    "outbound_backups": 1,
    "encrypt_leaseset": false,
    "close_idle": true,
    "close_idle_time": 10
  }
}
```

### Example Configuration Files

**Development Configuration** (`config-dev.json`):
```json
{
  "plugin": {
    "debug": true,
    "network_name": "i2p-dev"
  },
  "sam": {
    "host": "localhost",
    "port": 7656,
    "timeout": "60s"
  },
  "tunnel_defaults": {
    "inbound_tunnels": 1,
    "outbound_tunnels": 1,
    "close_idle_time": 5
  }
}
```

**Production Configuration** (`config-prod.json`):
```json
{
  "plugin": {
    "debug": false,
    "network_name": "i2p-prod",
    "ipam_subnet": "172.30.0.0/16",
    "gateway": "172.30.0.1"
  },
  "sam": {
    "host": "i2p-router.internal",
    "port": 7656,
    "timeout": "30s",
    "username": "docker-plugin",
    "password": "secure-password"
  },
  "tunnel_defaults": {
    "inbound_tunnels": 3,
    "outbound_tunnels": 3,
    "inbound_length": 4,
    "outbound_length": 4,
    "inbound_backups": 2,
    "outbound_backups": 2,
    "encrypt_leaseset": true,
    "close_idle_time": 30
  }
}
```

**High-Security Configuration** (`config-secure.json`):
```json
{
  "plugin": {
    "debug": false,
    "network_name": "i2p-secure"
  },
  "sam": {
    "host": "localhost",
    "port": 7656,
    "timeout": "45s"
  },
  "tunnel_defaults": {
    "inbound_tunnels": 5,
    "outbound_tunnels": 5,
    "inbound_length": 6,
    "outbound_length": 6,
    "inbound_backups": 3,
    "outbound_backups": 3,
    "encrypt_leaseset": true,
    "close_idle": false
  }
}
```

## Network Driver Options

Docker network creation supports driver-specific options:

### Available Options

| Option | Type | Description |
|--------|------|-------------|
| `i2p.sam.host` | string | Override SAM bridge host for this network |
| `i2p.sam.port` | int | Override SAM bridge port for this network |
| `i2p.tunnels.inbound` | int | Number of inbound tunnels |
| `i2p.tunnels.outbound` | int | Number of outbound tunnels |
| `i2p.tunnels.length.inbound` | int | Inbound tunnel length (hops) |
| `i2p.tunnels.length.outbound` | int | Outbound tunnel length (hops) |
| `i2p.encrypt.leaseset` | bool | Enable leaseset encryption |
| `i2p.filter.enabled` | bool | Enable traffic filtering |
| `i2p.filter.mode` | string | Filter mode: `allowlist`, `blocklist`, or `disabled` |
| `i2p.filter.allowlist` | string | Comma-separated list of allowed destinations |
| `i2p.filter.blocklist` | string | Comma-separated list of blocked destinations |
| `i2p.exposure.default` | string | Default port exposure type: `i2p` or `ip` (default: `i2p`) |
| `i2p.exposure.allow_ip` | bool | Allow IP-based port exposure (default: `true`) |

### Selective Port Exposure Options

The plugin supports flexible port exposure, allowing services to be exposed either to the I2P network or to specific IP addresses.

#### Container Labels

Container labels provide per-port exposure configuration:

| Label | Format | Description |
|-------|--------|-------------|
| `i2p.expose.<port>` | `i2p` or `ip[:address]` | Configure exposure for specific port |

**Label Formats:**
- `i2p.expose.80=i2p` - Expose port 80 to I2P network (.b32.i2p address)
- `i2p.expose.443=ip` - Expose port 443 to localhost (127.0.0.1:443)
- `i2p.expose.8080=ip:0.0.0.0` - Expose port 8080 to all interfaces
- `i2p.expose.3000=ip:192.168.1.100` - Expose port 3000 to specific IP
- `i2p.expose.9090=ip:::1` - Expose port 9090 to IPv6 localhost

**Validation**: Invalid IP addresses in exposure labels will cause the port to not be exposed (fail-safe behavior). Check plugin logs for validation warnings if ports aren't exposed as expected:
```bash
# Check for IP validation errors in plugin logs
sudo journalctl -u i2p-network-plugin | grep "Invalid target IP"
# Or if running as container:
docker logs i2p-network-plugin 2>&1 | grep "Invalid target IP"
```

#### Network-Level Configuration

Network-level options set defaults for all containers on the network:

**`i2p.exposure.default`** (string, default: `i2p`)
- Sets the default exposure type for ports without explicit configuration
- Values: `i2p` (I2P network) or `ip` (IP address)
- Applies to ports detected from EXPOSE directives and environment variables

**`i2p.exposure.allow_ip`** (bool, default: `true`)
- Controls whether IP-based exposure is permitted on this network
- When `false`, all IP exposure requests are forced to I2P
- Provides network-level security policy enforcement

#### Configuration Precedence

Port exposure sources are combined with the following precedence:
1. **Container labels** (`i2p.expose.*`) - Explicit port configuration
2. **Docker EXPOSE directives** - Automatic port detection, defaults to network's `i2p.exposure.default`
3. **Environment variables** (`PORT`, `HTTP_PORT`, etc.) - Automatic port detection, defaults to network's `i2p.exposure.default`

**Important**: Labels *augment* rather than override automatic detection. If you specify a label for a port that's also in EXPOSE, both configurations will be applied if they have different exposure types (e.g., `i2p.expose.80=ip` + `EXPOSE 80` results in both IP and I2P exposure for port 80). To prevent auto-exposure of a port, explicitly configure all ports you want exposed via labels.

Network policy (`i2p.exposure.allow_ip`) is always enforced regardless of configuration source.

### Network Driver Options Examples

```bash
# Create network with custom SAM host
docker network create --driver=i2p \
  --opt i2p.sam.host=192.168.1.100 \
  --opt i2p.sam.port=7656 \
  remote-i2p

# Create high-performance network
docker network create --driver=i2p \
  --opt i2p.tunnels.inbound=5 \
  --opt i2p.tunnels.outbound=5 \
  --opt i2p.tunnels.length.inbound=2 \
  --opt i2p.tunnels.length.outbound=2 \
  fast-i2p

# Create secure network with filtering
docker network create --driver=i2p \
  --opt i2p.encrypt.leaseset=true \
  --opt i2p.filter.enabled=true \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowlist="trusted.i2p,*.example.i2p" \
  secure-i2p

# Create development network with permissive settings
docker network create --driver=i2p \
  --opt i2p.tunnels.inbound=1 \
  --opt i2p.tunnels.outbound=1 \
  --opt i2p.filter.mode=disabled \
  dev-i2p

# Create network with IP exposure as default
docker network create --driver=i2p \
  --opt i2p.exposure.default=ip \
  --opt i2p.exposure.allow_ip=true \
  dev-network

# Create secure I2P-only network (disallow IP exposure)
docker network create --driver=i2p \
  --opt i2p.exposure.allow_ip=false \
  secure-i2p-network
```

### Container Label Examples

```bash
# Expose port 80 to I2P, port 443 to localhost
docker run -d --name web-service \
  --network my-i2p-network \
  --label i2p.expose.80=i2p \
  --label i2p.expose.443=ip:127.0.0.1 \
  nginx:alpine

# Expose multiple ports with different configurations
docker run -d --name multi-service \
  --network my-i2p-network \
  --label i2p.expose.8080=i2p \
  --label i2p.expose.9090=ip:0.0.0.0 \
  --label i2p.expose.3000=ip:192.168.1.100 \
  multi-app:latest

# IPv6 exposure
docker run -d --name ipv6-service \
  --network my-i2p-network \
  --label i2p.expose.8080=ip:::1 \
  --label i2p.expose.8443=ip:fe80::1 \
  app:latest

# Default localhost exposure (omit IP address)
docker run -d --name local-service \
  --network my-i2p-network \
  --label i2p.expose.5000=ip \
  api:latest
# Port 5000 exposed on 127.0.0.1:5000
```

## Validation Rules

### Plugin Configuration

| Field | Validation Rules |
|-------|------------------|
| `socket_path` | Must not be empty |
| `network_name` | Must not be empty |
| `ipam_subnet` | Must be valid CIDR notation |
| `gateway` | Must be valid IP address |

### SAM Configuration

| Field | Validation Rules |
|-------|------------------|
| `host` | Must not be empty |
| `port` | Must be between 1 and 65535 |
| `timeout` | Must be positive duration |
| `username` | Optional, any string |
| `password` | Optional, any string |

### Tunnel Configuration

| Field | Validation Rules |
|-------|------------------|
| `inbound_tunnels` | Must be positive integer |
| `outbound_tunnels` | Must be positive integer |
| `inbound_length` | Must be positive integer (recommended: 1-10) |
| `outbound_length` | Must be positive integer (recommended: 1-10) |
| `inbound_backups` | Must be non-negative integer |
| `outbound_backups` | Must be non-negative integer |
| `close_idle_time` | Must be positive integer (minutes) |

### Performance Recommendations

| Use Case | Tunnels | Length | Notes |
|----------|---------|--------|-------|
| **Development** | 1-2 | 1-2 | Fast startup, low security |
| **Testing** | 2-3 | 2-3 | Balanced performance |
| **Production** | 3-5 | 3-4 | Good security/performance balance |
| **High Security** | 5+ | 5+ | Maximum security, slower performance |
| **High Performance** | 5+ | 1-2 | Maximum speed, lower anonymity |

## Examples

### Minimal Configuration

```bash
# Environment variables only
export I2P_SAM_HOST="localhost"
export DEBUG="false"

# Start plugin
./i2p-network-plugin -sock /tmp/i2p.sock
```

### Development Setup

```bash
# Development environment variables
export DEBUG="true"
export NETWORK_NAME="dev-i2p"
export I2P_INBOUND_TUNNELS="1"
export I2P_OUTBOUND_TUNNELS="1"
export I2P_CLOSE_IDLE_TIME="2"

# Create development network
docker network create --driver=i2p \
  --opt i2p.filter.mode=disabled \
  dev-network

# Run development container
docker run -d --name dev-app \
  --network dev-network \
  -e NODE_ENV=development \
  my-app:dev
```

### Production Setup

```bash
# Production environment variables
export I2P_SAM_HOST="i2p-router.internal"
export I2P_SAM_USERNAME="docker-plugin"
export I2P_SAM_PASSWORD="secure-password"
export I2P_INBOUND_TUNNELS="3"
export I2P_OUTBOUND_TUNNELS="3"
export I2P_INBOUND_LENGTH="4"
export I2P_OUTBOUND_LENGTH="4"
export I2P_ENCRYPT_LEASESET="true"

# Create production network with filtering
docker network create --driver=i2p \
  --subnet=172.30.0.0/16 \
  --gateway=172.30.0.1 \
  --opt i2p.filter.enabled=true \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowlist="*.production.i2p,trusted-service.i2p" \
  production-network

# Deploy production services
docker run -d --name web-service \
  --network production-network \
  --restart unless-stopped \
  -e NODE_ENV=production \
  web-service:latest
```

### High-Security Setup

```bash
# Maximum security configuration
export I2P_INBOUND_TUNNELS="5"
export I2P_OUTBOUND_TUNNELS="5"
export I2P_INBOUND_LENGTH="6"
export I2P_OUTBOUND_LENGTH="6"
export I2P_INBOUND_BACKUPS="3"
export I2P_OUTBOUND_BACKUPS="3"
export I2P_ENCRYPT_LEASESET="true"
export I2P_CLOSE_IDLE="false"

# Create highly secure network
docker network create --driver=i2p \
  --opt i2p.encrypt.leaseset=true \
  --opt i2p.tunnels.inbound=5 \
  --opt i2p.tunnels.outbound=5 \
  --opt i2p.tunnels.length.inbound=6 \
  --opt i2p.tunnels.length.outbound=6 \
  --opt i2p.filter.enabled=true \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowlist="verified.i2p" \
  secure-network
```

### Multi-Network Setup

```bash
# Create separate networks for different security levels
docker network create --driver=i2p \
  --opt i2p.filter.mode=disabled \
  public-i2p

docker network create --driver=i2p \
  --opt i2p.filter.mode=allowlist \
  --opt i2p.filter.allowlist="*.internal.i2p" \
  internal-i2p

docker network create --driver=i2p \
  --opt i2p.encrypt.leaseset=true \
  --opt i2p.tunnels.inbound=5 \
  --opt i2p.tunnels.outbound=5 \
  secure-i2p

# Deploy services to appropriate networks
docker run -d --name public-web \
  --network public-i2p \
  public-website:latest

docker run -d --name internal-api \
  --network internal-i2p \
  internal-api:latest

docker run -d --name secure-db \
  --network secure-i2p \
  secure-database:latest
```

## See Also

- [USAGE.md](USAGE.md) - Usage examples and tutorials
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues and solutions
- [KNOWN_ISSUES.md](KNOWN_ISSUES.md) - Current limitations
- [Docker Network Plugin API](https://docs.docker.com/engine/extend/plugins_network/)
- [I2P SAM Documentation](https://geti2p.net/en/docs/api/sam)
