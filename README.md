Anonymizing Docker Network Plugin
=================================

When complete, this docker network plugin will:

- Transparently set up I2P services exposed docker services
- Transparently proxy all I2P traffic initiated from inside the docker container over I2P
- Log and drop all non-I2P traffic initiated from inside the docker container

## Development Status

This project is currently in active development. See [PLAN.md](PLAN.md) for detailed development roadmap.

### Completed Features

- ✅ **Project Foundation**: Go module, build system, and project structure
- ✅ **Basic Plugin Framework**: Docker Plugin API v2 compliance with all required endpoints
- ✅ **I2P Integration**: SAM client connectivity and session management
- ✅ **Container Isolation**: Individual SAM connections and primary sessions per container
- ✅ **Service Exposure**: Automatic I2P server tunnel creation for exposed ports
- ✅ **Traffic Proxying**: Transparent I2P traffic routing with SOCKS and DNS proxying
- ✅ **Testing Infrastructure**: Comprehensive test suite with >60% coverage
- ✅ **Build System**: Makefile with common development tasks

### Current Architecture

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

**Key Components:**
```
cmd/i2p-network-plugin/    # Main plugin executable
pkg/plugin/                # Docker network plugin implementation (CNM)
pkg/i2p/                   # I2P SAM client and tunnel management
pkg/proxy/                 # Traffic interception and proxying
pkg/service/               # Automatic service exposure
internal/config/           # Internal configuration management
test/                      # Integration tests
```

**I2P Session Architecture:**
- **One SAM Client per Container**: Each container gets its own dedicated SAM connection to the I2P router
- **One Primary Session per Container**: Each SAM client creates a single primary session with unique I2P keys
- **Multiple Sub-sessions per Container**: Each primary session can create multiple sub-sessions for different purposes:
  - **Stream sub-sessions**: For TCP connections (HTTP, HTTPS, API calls)
  - **Server sub-sessions**: For exposing services (.b32.i2p addresses)
  - **Datagram sub-sessions**: For UDP traffic (future feature)
  - **Raw sub-sessions**: For custom protocols (future feature)

## Building

```bash
# Build the plugin
make build

# Run tests
make test

# View all available targets
make help
```

## Usage (Development)

The plugin binary can be built and tested locally:

```bash
# Build
make build

# Test version
./bin/i2p-network-plugin -version

# View help
./bin/i2p-network-plugin -help
```

> **Note**: Full Docker integration and I2P functionality is not yet implemented. See PLAN.md for upcoming features.
