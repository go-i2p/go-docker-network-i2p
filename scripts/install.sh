#!/bin/bash
# I2P Docker Network Plugin Installation Script
# This script installs the I2P network plugin for Docker on Linux systems

set -e  # Exit on error
set -u  # Exit on undefined variable

# Configuration
PLUGIN_NAME="i2p-network-plugin"
INSTALL_DIR="/usr/local/bin"
PLUGIN_DIR="/run/docker/plugins"
SYSTEMD_DIR="/etc/systemd/system"
DATA_DIR="/var/lib/i2p-network-plugin"
PLUGIN_SOCKET="${PLUGIN_DIR}/i2p-network.sock"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print functions
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

# Check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    # Check for Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check Docker version (requires 20.10+)
    DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' | cut -d. -f1,2)
    REQUIRED_VERSION="20.10"
    if awk "BEGIN {exit !($DOCKER_VERSION < $REQUIRED_VERSION)}"; then
        print_warn "Docker version $DOCKER_VERSION detected. Version $REQUIRED_VERSION or higher recommended."
    fi
    
    # Check for iptables
    if ! command -v iptables &> /dev/null; then
        print_error "iptables is not installed. Please install iptables first."
        exit 1
    fi
    
    print_info "Prerequisites check passed"
}

# Check for I2P router
check_i2p() {
    print_info "Checking I2P router availability..."
    
    # Check if I2P SAM bridge is accessible
    if timeout 2 bash -c "cat < /dev/null > /dev/tcp/localhost/7656" 2>/dev/null; then
        print_info "I2P router with SAM bridge detected on localhost:7656"
    else
        print_warn "I2P router SAM bridge not detected on localhost:7656"
        print_warn "The plugin requires an I2P router with SAM bridge enabled"
        print_warn "You can install I2P router separately and enable SAM bridge"
    fi
}

# Create directories
create_directories() {
    print_info "Creating plugin directories..."
    
    mkdir -p "${PLUGIN_DIR}"
    mkdir -p "${DATA_DIR}"
    
    # Set permissions
    chmod 755 "${PLUGIN_DIR}"
    chmod 755 "${DATA_DIR}"
    
    print_info "Directories created successfully"
}

# Install binary
install_binary() {
    print_info "Installing plugin binary..."
    
    # Check if binary exists in current directory
    if [ -f "./bin/${PLUGIN_NAME}" ]; then
        cp "./bin/${PLUGIN_NAME}" "${INSTALL_DIR}/${PLUGIN_NAME}"
    elif [ -f "./${PLUGIN_NAME}" ]; then
        cp "./${PLUGIN_NAME}" "${INSTALL_DIR}/${PLUGIN_NAME}"
    else
        print_error "Plugin binary not found. Please build the plugin first with 'make build'"
        exit 1
    fi
    
    chmod +x "${INSTALL_DIR}/${PLUGIN_NAME}"
    
    print_info "Plugin binary installed to ${INSTALL_DIR}/${PLUGIN_NAME}"
}

# Create systemd service
create_systemd_service() {
    print_info "Creating systemd service..."
    
    cat > "${SYSTEMD_DIR}/${PLUGIN_NAME}.service" <<EOF
[Unit]
Description=I2P Docker Network Plugin
Documentation=https://github.com/go-i2p/go-docker-network-i2p
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${PLUGIN_NAME} -sock ${PLUGIN_SOCKET}
ExecStop=/bin/rm -f ${PLUGIN_SOCKET}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=false
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${PLUGIN_DIR} ${DATA_DIR}

# Environment
Environment="I2P_SAM_HOST=localhost"
Environment="I2P_SAM_PORT=7656"

[Install]
WantedBy=multi-user.target
EOF
    
    print_info "Systemd service created at ${SYSTEMD_DIR}/${PLUGIN_NAME}.service"
}

# Enable and start service
enable_service() {
    print_info "Enabling and starting systemd service..."
    
    systemctl daemon-reload
    systemctl enable "${PLUGIN_NAME}.service"
    systemctl start "${PLUGIN_NAME}.service"
    
    # Wait for service to start
    sleep 2
    
    # Check service status
    if systemctl is-active --quiet "${PLUGIN_NAME}.service"; then
        print_info "Plugin service started successfully"
    else
        print_error "Failed to start plugin service"
        print_info "Check logs with: journalctl -u ${PLUGIN_NAME}.service"
        exit 1
    fi
}

# Verify installation
verify_installation() {
    print_info "Verifying installation..."
    
    # Check if socket exists
    if [ -S "${PLUGIN_SOCKET}" ]; then
        print_info "Plugin socket created successfully"
    else
        print_error "Plugin socket not found at ${PLUGIN_SOCKET}"
        exit 1
    fi
    
    # Check if Docker recognizes the plugin
    if docker plugin ls --format '{{.Name}}' 2>/dev/null | grep -q "i2p"; then
        print_info "Plugin recognized by Docker"
    else
        print_warn "Plugin not yet recognized by Docker (this may take a moment)"
    fi
    
    print_info "Installation verification complete"
}

# Print usage instructions
print_usage() {
    cat <<EOF

${GREEN}Installation Complete!${NC}

The I2P Docker Network Plugin has been successfully installed.

${YELLOW}Next Steps:${NC}

1. Verify the plugin is running:
   sudo systemctl status ${PLUGIN_NAME}

2. View plugin logs:
   sudo journalctl -u ${PLUGIN_NAME} -f

3. Create an I2P network:
   docker network create --driver=i2p my-i2p-network

4. Run a container on the I2P network:
   docker run -d --name test --network my-i2p-network nginx:alpine

${YELLOW}Configuration:${NC}

- Plugin binary: ${INSTALL_DIR}/${PLUGIN_NAME}
- Plugin socket: ${PLUGIN_SOCKET}
- Data directory: ${DATA_DIR}
- Systemd service: ${SYSTEMD_DIR}/${PLUGIN_NAME}.service

${YELLOW}Documentation:${NC}

- Usage: https://github.com/go-i2p/go-docker-network-i2p/blob/master/USAGE.md
- Configuration: https://github.com/go-i2p/go-docker-network-i2p/blob/master/CONFIG.md
- Troubleshooting: https://github.com/go-i2p/go-docker-network-i2p/blob/master/TROUBLESHOOTING.md

EOF
}

# Uninstall function
uninstall() {
    print_info "Uninstalling I2P Docker Network Plugin..."
    
    # Stop and disable service
    if systemctl is-active --quiet "${PLUGIN_NAME}.service"; then
        systemctl stop "${PLUGIN_NAME}.service"
    fi
    systemctl disable "${PLUGIN_NAME}.service" 2>/dev/null || true
    
    # Remove files
    rm -f "${INSTALL_DIR}/${PLUGIN_NAME}"
    rm -f "${SYSTEMD_DIR}/${PLUGIN_NAME}.service"
    rm -f "${PLUGIN_SOCKET}"
    
    # Optionally remove data directory
    read -p "Remove plugin data directory ${DATA_DIR}? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "${DATA_DIR}"
        print_info "Data directory removed"
    fi
    
    systemctl daemon-reload
    
    print_info "Uninstallation complete"
}

# Main installation flow
main() {
    # Check for uninstall flag
    if [ "${1:-}" = "--uninstall" ] || [ "${1:-}" = "-u" ]; then
        check_root
        uninstall
        exit 0
    fi
    
    # Check for help flag
    if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
        cat <<EOF
I2P Docker Network Plugin Installation Script

Usage: $0 [OPTIONS]

Options:
  -h, --help       Show this help message
  -u, --uninstall  Uninstall the plugin
  --no-service     Install binary only, don't create systemd service

Examples:
  sudo $0                    # Full installation
  sudo $0 --uninstall        # Uninstall plugin
  sudo $0 --no-service       # Install without systemd
EOF
        exit 0
    fi
    
    print_info "Starting I2P Docker Network Plugin installation..."
    
    check_root
    check_prerequisites
    check_i2p
    create_directories
    install_binary
    
    # Check for --no-service flag
    if [ "${1:-}" != "--no-service" ]; then
        create_systemd_service
        enable_service
        verify_installation
    else
        print_info "Skipping systemd service creation (--no-service flag)"
    fi
    
    print_usage
}

# Run main function
main "$@"
