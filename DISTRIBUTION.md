# Distribution Guide

This document describes the packaging and distribution infrastructure for the I2P Docker Network Plugin.

## Overview

The plugin provides multiple distribution methods to suit different deployment scenarios:
- **Docker Image**: Containerized deployment for consistency and isolation
- **Binary Distribution**: Direct installation via release artifacts
- **Source Build**: Build from source with full control

## Docker Image Distribution

### Building the Image

```bash
# Build Docker image
make docker-build

# Build with custom version
VERSION=1.0.0 make docker-build
```

The Docker image uses a multi-stage build process:
1. **Builder stage**: Compiles the Go binary with Alpine Linux
2. **Runtime stage**: Creates minimal image (27.3MB) with only necessary dependencies

### Running the Plugin Container

```bash
# Run plugin in Docker
make docker-run

# Or manually:
docker run -d \
  --name i2p-network-plugin \
  --privileged \
  --network host \
  -v /run/docker/plugins:/run/docker/plugins \
  -v /var/lib/i2p-network-plugin:/var/lib/i2p-network-plugin \
  golovers/i2p-network-plugin:latest
```

**Note**: The container requires:
- `--privileged` flag for iptables manipulation
- `--network host` for Docker plugin socket access
- Volume mounts for plugin communication and data persistence

### Pushing to Registry

```bash
# Push to Docker Hub (requires authentication)
make docker-push
```

## Binary Distribution

### Creating Release Artifacts

```bash
# Build release artifacts
make release-artifacts

# This creates in dist/:
# - i2p-network-plugin-VERSION-linux-amd64 (binary)
# - i2p-network-plugin-VERSION-linux-amd64.sha256 (checksum)
# - i2p-network-plugin-VERSION-linux-amd64.tar.gz (archive)
```

### Installing from Binary

```bash
# Extract tarball
tar xzf i2p-network-plugin-VERSION-linux-amd64.tar.gz

# Build the plugin
make build

# Install system-wide with systemd service
sudo make system-install

# Or use the installation script directly
sudo bash scripts/install.sh
```

## Installation Script Features

The `scripts/install.sh` script provides automated installation with the following features:

### Prerequisites Check
- Verifies Docker installation (version 20.10+)
- Checks for iptables availability
- Detects I2P router with SAM bridge

### Installation Options

**Full Installation** (default):
```bash
sudo bash scripts/install.sh
```
Creates:
- Binary installation in `/usr/local/bin/`
- Plugin socket directory `/run/docker/plugins/`
- Data directory `/var/lib/i2p-network-plugin/`
- Systemd service for automatic startup

**Binary-Only Installation** (no systemd):
```bash
sudo bash scripts/install.sh --no-service
```

**Uninstallation**:
```bash
sudo bash scripts/install.sh --uninstall
```

### Systemd Service

The installation creates a systemd service (`i2p-network-plugin.service`) that:
- Starts automatically on boot
- Restarts on failure
- Logs to journalctl
- Runs with security hardening

View service status:
```bash
sudo systemctl status i2p-network-plugin
```

View logs:
```bash
sudo journalctl -u i2p-network-plugin -f
```

## Release Process

### Complete Release Workflow

```bash
# 1. Ensure all tests pass
make test

# 2. Create release artifacts and Docker image
make release

# 3. Push Docker image to registry
make docker-push

# 4. Create and push git tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 5. Create GitHub release and upload artifacts from dist/
```

### Version Tagging

The build system uses git tags for versioning:
- **Tagged commits**: Use the tag as version (e.g., `v1.0.0`)
- **Untagged commits**: Use git hash with `-dirty` suffix if uncommitted changes exist

Version is embedded in the binary at build time:
```bash
./bin/i2p-network-plugin -version
```

## Build Configuration

### Build Variables

The Makefile supports customization via environment variables:

```bash
# Custom version
VERSION=1.0.0 make build

# Custom Docker image name
DOCKER_IMAGE=myregistry/i2p-plugin make docker-build

# Custom installation directories
INSTALL_DIR=/opt/plugins make install
```

### Build Arguments for Docker

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S') \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t my-i2p-plugin:1.0.0 \
  .
```

## Distribution Checklist

Before creating a release:

- [ ] All tests pass (`make test`)
- [ ] Code is properly formatted (`make fmt`)
- [ ] No linter warnings (`make lint`)
- [ ] Documentation is up-to-date
- [ ] Version tag is created
- [ ] Docker image builds successfully
- [ ] Release artifacts are created
- [ ] Checksums are verified
- [ ] Installation script tested

## Security Considerations

### Docker Image Security

- Uses minimal Alpine Linux base (reduces attack surface)
- Multi-stage build (no build tools in final image)
- Non-root user created (commented out due to iptables requirements)
- Health check included for monitoring

### Binary Distribution Security

- SHA256 checksums provided for all artifacts
- Signed releases recommended (GPG signatures)
- Verify checksums before installation:
  ```bash
  sha256sum -c i2p-network-plugin-VERSION-linux-amd64.sha256
  ```

### Installation Security

- Installation script requires root (sudo)
- Systemd service runs with security hardening:
  - NoNewPrivileges (when possible)
  - PrivateTmp
  - ProtectSystem=strict
  - ProtectHome

## Troubleshooting Distribution

### Docker Build Issues

**Problem**: Go version mismatch
```
Solution: Update Dockerfile FROM line to use golang:alpine (latest)
```

**Problem**: Local dependencies not found
```
Solution: Ensure go.mod has no local replace directives for Docker builds
```

### Installation Issues

**Problem**: Permission denied
```
Solution: Run installation script with sudo
```

**Problem**: Docker plugin not recognized
```
Solution: Wait a moment for Docker to discover the plugin socket
Check: docker plugin ls
```

**Problem**: I2P router not detected
```
Solution: Install I2P router and enable SAM bridge on localhost:7656
```

## Support

For distribution-related issues:
- Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common problems
- Review installation logs: `journalctl -u i2p-network-plugin`
- File an issue on GitHub with distribution details

## Future Improvements

Potential enhancements to the distribution system:
- [ ] Additional architectures (ARM, ARM64)
- [ ] Package managers (apt, yum, homebrew)
- [ ] Docker plugin v2 proper packaging
- [ ] Automated CI/CD pipeline
- [ ] GPG-signed releases
- [ ] Helm chart for Kubernetes
