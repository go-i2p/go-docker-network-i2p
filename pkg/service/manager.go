// Package service provides I2P service exposure functionality for Docker containers.package service

// This package implements automatic I2P service exposure for containers that expose ports.
// When a container exposes ports (e.g., via docker run -p or EXPOSE in Dockerfile),
// this package automatically creates I2P server tunnels and generates .b32.i2p addresses
// that external I2P users can use to access the services.
//
// Key features:
// - Automatic port detection from container options and environment variables
// - I2P server tunnel creation and management
// - .b32.i2p address generation from I2P destination keys
// - Persistent destination key storage for consistent service addresses
// - Integration with NetworkManager for endpoint lifecycle management
package service

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

// ExposureType represents how a port should be exposed.
type ExposureType string

const (
	// ExposureTypeI2P exposes the port to I2P network only (default)
	ExposureTypeI2P ExposureType = "i2p"
	// ExposureTypeIP exposes the port to specific IP interface
	ExposureTypeIP ExposureType = "ip"
)

// ExposedPort represents a port that should be exposed over I2P.
type ExposedPort struct {
	// ContainerPort is the port inside the container
	ContainerPort int
	// Protocol is the protocol (tcp/udp)
	Protocol string
	// ServiceName is an optional name for the service
	ServiceName string
	// ExposureType defines how the port should be exposed (i2p or ip)
	ExposureType ExposureType `json:"exposure_type,omitempty"`
	// TargetIP is the IP address for IP-based exposure (only used when ExposureType is "ip")
	TargetIP string `json:"target_ip,omitempty"`
}

// NetworkExposureConfig defines network-level exposure defaults.
type NetworkExposureConfig struct {
	// DefaultExposureType is the default exposure type for ports without explicit configuration
	DefaultExposureType ExposureType
	// AllowIPExposure determines if IP-based exposure is permitted
	AllowIPExposure bool
}

// ServiceExposure represents an I2P service exposure configuration.
type ServiceExposure struct {
	// ContainerID identifies the container providing the service
	ContainerID string
	// Port is the exposed port configuration
	Port ExposedPort
	// Tunnel is the I2P server tunnel for this service
	Tunnel *i2p.Tunnel
	// Destination is the I2P destination address (.b32.i2p format)
	Destination string
	// TunnelName is the internal name for the tunnel
	TunnelName string
}

// ServiceExposureManager manages I2P service exposure for containers.
//
// The manager handles automatic detection of exposed ports, creation of
// I2P server tunnels, and generation of .b32.i2p addresses for services.
type ServiceExposureManager struct {
	// tunnelMgr provides I2P tunnel management capabilities
	tunnelMgr *i2p.TunnelManager

	// exposures tracks all active service exposures by container ID
	exposures map[string][]*ServiceExposure

	// mutex protects concurrent access to exposures
	mutex sync.RWMutex

	// ctx provides cancellation context
	ctx context.Context

	// cancel cancels the context
	cancel context.CancelFunc
}

// NewServiceExposureManager creates a new service exposure manager.
//
// The manager requires a TunnelManager to create I2P server tunnels for exposed services.
func NewServiceExposureManager(tunnelMgr *i2p.TunnelManager) (*ServiceExposureManager, error) {
	if tunnelMgr == nil {
		return nil, fmt.Errorf("tunnel manager cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceExposureManager{
		tunnelMgr: tunnelMgr,
		exposures: make(map[string][]*ServiceExposure),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// DetectExposedPorts analyzes container options to identify exposed ports.
//
// This method examines Docker container options and environment variables to
// identify ports that should be exposed over I2P. It supports various formats:
// - Docker EXPOSE directives
// - Port mappings from container options
// - Environment variables indicating exposed services
func (sem *ServiceExposureManager) DetectExposedPorts(containerID string, options map[string]interface{}) ([]ExposedPort, error) {
	if containerID == "" {
		return nil, fmt.Errorf("container ID cannot be empty")
	}

	var ports []ExposedPort

	// Check for exposed ports in container options
	if exposedPorts := sem.extractPortsFromOptions(options); len(exposedPorts) > 0 {
		ports = append(ports, exposedPorts...)
	}

	// Check for environment variables indicating services
	if envPorts := sem.extractPortsFromEnvironment(options); len(envPorts) > 0 {
		ports = append(ports, envPorts...)
	}

	// Deduplicate ports
	seen := make(map[string]bool)
	var uniquePorts []ExposedPort
	for _, port := range ports {
		key := fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol)
		if !seen[key] {
			seen[key] = true
			uniquePorts = append(uniquePorts, port)
		}
	}

	log.Printf("Detected %d exposed ports for container %s", len(uniquePorts), containerID)
	return uniquePorts, nil
}

// extractPortsFromOptions extracts exposed ports from Docker container options.
//
// This method parses Docker container options looking for port specifications
// in various formats that Docker supports.
func (sem *ServiceExposureManager) extractPortsFromOptions(options map[string]interface{}) []ExposedPort {
	var ports []ExposedPort

	// Check for "ExposedPorts" option (Docker format)
	if exposedPorts, ok := options["ExposedPorts"]; ok {
		if portsMap, ok := exposedPorts.(map[string]interface{}); ok {
			for portSpec := range portsMap {
				if port := sem.parsePortSpec(portSpec); port != nil {
					ports = append(ports, *port)
				}
			}
		}
	}

	// Check for "com.docker.network.portmap" option
	if portMap, ok := options["com.docker.network.portmap"]; ok {
		if portList, ok := portMap.([]interface{}); ok {
			for _, portInfo := range portList {
				if portData, ok := portInfo.(map[string]interface{}); ok {
					if port := sem.parsePortMapping(portData); port != nil {
						ports = append(ports, *port)
					}
				}
			}
		}
	}

	return ports
}

// extractPortsFromEnvironment extracts port information from environment variables.
//
// This method looks for common environment variable patterns that indicate
// services and their ports (e.g., PORT=8080, HTTP_PORT=80, etc.).
func (sem *ServiceExposureManager) extractPortsFromEnvironment(options map[string]interface{}) []ExposedPort {
	var ports []ExposedPort

	// Check for environment variables in options
	if env, ok := options["Env"]; ok {
		if envList, ok := env.([]interface{}); ok {
			for _, envVar := range envList {
				if envStr, ok := envVar.(string); ok {
					if port := sem.parseEnvironmentPort(envStr); port != nil {
						ports = append(ports, *port)
					}
				}
			}
		}
	}

	return ports
}

// parsePortSpec parses a Docker port specification (e.g., "80/tcp", "443/tcp").
func (sem *ServiceExposureManager) parsePortSpec(portSpec string) *ExposedPort {
	// Match pattern like "80/tcp" or "443/udp"
	re := regexp.MustCompile(`^(\d+)/(tcp|udp)$`)
	matches := re.FindStringSubmatch(portSpec)
	if len(matches) != 3 {
		return nil
	}

	port, err := strconv.Atoi(matches[1])
	if err != nil || port <= 0 || port > 65535 {
		return nil
	}

	protocol := matches[2]

	return &ExposedPort{
		ContainerPort: port,
		Protocol:      protocol,
		ServiceName:   fmt.Sprintf("service-%d", port),
	}
}

// parsePortMapping parses Docker port mapping information.
func (sem *ServiceExposureManager) parsePortMapping(portData map[string]interface{}) *ExposedPort {
	// Extract container port
	containerPort, ok := portData["ContainerPort"]
	if !ok {
		return nil
	}

	var port int
	switch v := containerPort.(type) {
	case int:
		port = v
	case float64:
		port = int(v)
	case string:
		var err error
		port, err = strconv.Atoi(v)
		if err != nil {
			return nil
		}
	default:
		return nil
	}

	if port <= 0 || port > 65535 {
		return nil
	}

	// Extract protocol (default to TCP)
	protocol := "tcp"
	if proto, ok := portData["Protocol"]; ok {
		if protoStr, ok := proto.(string); ok {
			protocol = strings.ToLower(protoStr)
		}
	}

	return &ExposedPort{
		ContainerPort: port,
		Protocol:      protocol,
		ServiceName:   fmt.Sprintf("service-%d", port),
	}
}

// parseEnvironmentPort parses environment variables for port information.
func (sem *ServiceExposureManager) parseEnvironmentPort(envVar string) *ExposedPort {
	// Look for patterns like "PORT=8080", "HTTP_PORT=80", "SERVICE_PORT=3000"
	portPatterns := []string{
		`^PORT=(\d+)$`,
		`^HTTP_PORT=(\d+)$`,
		`^HTTPS_PORT=(\d+)$`,
		`^SERVICE_PORT=(\d+)$`,
		`^APP_PORT=(\d+)$`,
		`^SERVER_PORT=(\d+)$`,
	}

	for _, pattern := range portPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(envVar)
		if len(matches) == 2 {
			port, err := strconv.Atoi(matches[1])
			if err != nil || port <= 0 || port > 65535 {
				continue
			}

			// Determine service name from environment variable name
			serviceName := "service"
			if idx := strings.Index(envVar, "="); idx > 0 {
				envName := strings.ToLower(envVar[:idx])
				if strings.Contains(envName, "http") {
					serviceName = "http"
				} else if strings.Contains(envName, "app") {
					serviceName = "app"
				} else if strings.Contains(envName, "server") {
					serviceName = "server"
				}
			}

			return &ExposedPort{
				ContainerPort: port,
				Protocol:      "tcp", // Default to TCP for environment-specified ports
				ServiceName:   fmt.Sprintf("%s-%d", serviceName, port),
			}
		}
	}

	return nil
}

// extractPortsFromLabels extracts port exposure configuration from Docker labels.
//
// This method looks for labels with the prefix "i2p.expose." and parses them to
// determine how ports should be exposed. Label format:
//   - i2p.expose.80=i2p          (expose port 80 to I2P network)
//   - i2p.expose.443=ip:127.0.0.1 (expose port 443 to localhost IP)
func (sem *ServiceExposureManager) extractPortsFromLabels(options map[string]interface{}) []ExposedPort {
	var ports []ExposedPort

	// Look for Labels in options
	if labels, ok := options["Labels"]; ok {
		if labelMap, ok := labels.(map[string]interface{}); ok {
			for key, value := range labelMap {
				if strings.HasPrefix(key, "i2p.expose.") {
					if port := sem.parseExposureLabel(key, value); port != nil {
						ports = append(ports, *port)
					}
				}
			}
		}
	}

	return ports
}

// parseExposureLabel parses individual exposure labels.
//
// Label formats supported:
//   - i2p.expose.80=i2p          (expose port 80 to I2P)
//   - i2p.expose.443=ip:127.0.0.1 (expose port 443 to localhost)
//
// Returns nil if the label format is invalid.
func (sem *ServiceExposureManager) parseExposureLabel(key string, value interface{}) *ExposedPort {
	// Extract port number from label key (e.g., "i2p.expose.80" -> "80")
	portStr := strings.TrimPrefix(key, "i2p.expose.")
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		log.Printf("Warning: Invalid port in label %s: %v", key, err)
		return nil
	}

	// Parse value (exposure configuration)
	valueStr, ok := value.(string)
	if !ok {
		log.Printf("Warning: Invalid value type for label %s", key)
		return nil
	}

	// Parse exposure configuration
	// Format: "i2p" or "ip:127.0.0.1"
	parts := strings.SplitN(valueStr, ":", 2)
	exposureType := ExposureType(parts[0])

	// Validate exposure type
	if exposureType != ExposureTypeI2P && exposureType != ExposureTypeIP {
		log.Printf("Warning: Invalid exposure type in label %s: %s", key, exposureType)
		return nil
	}

	var targetIP string
	if len(parts) > 1 {
		targetIP = parts[1]
	}

	// If exposure type is IP but no target IP specified, default to localhost
	if exposureType == ExposureTypeIP && targetIP == "" {
		targetIP = "127.0.0.1"
	}

	// Validate IP address format when provided and not empty
	if targetIP != "" && net.ParseIP(targetIP) == nil {
		log.Printf("Warning: Invalid target IP in label %s: %s", key, targetIP)
		return nil
	}

	return &ExposedPort{
		ContainerPort: port,
		Protocol:      "tcp",
		ServiceName:   fmt.Sprintf("service-%d", port),
		ExposureType:  exposureType,
		TargetIP:      targetIP,
	}
}

// ExposeServices creates I2P server tunnels for the specified exposed ports.
//
// This method creates I2P server tunnels for each exposed port and generates
// .b32.i2p addresses that external users can use to access the services.
func (sem *ServiceExposureManager) ExposeServices(containerID string, networkID string, containerIP net.IP, ports []ExposedPort) ([]*ServiceExposure, error) {
	if containerID == "" {
		return nil, fmt.Errorf("container ID cannot be empty")
	}
	if networkID == "" {
		return nil, fmt.Errorf("network ID cannot be empty")
	}
	if containerIP == nil {
		return nil, fmt.Errorf("container IP cannot be nil")
	}

	sem.mutex.Lock()
	defer sem.mutex.Unlock()

	var exposures []*ServiceExposure

	for _, port := range ports {
		exposure, err := sem.createServiceExposure(containerID, networkID, containerIP, port)
		if err != nil {
			// Note: Using go-sam-go NewStreamSubSessionWithPort for multiple server tunnels
			log.Printf("Warning: Failed to expose service on port %d for container %s: %v", port.ContainerPort, containerID, err)
			continue
		}

		exposures = append(exposures, exposure)
		log.Printf("Successfully exposed service %s for container %s on I2P destination %s",
			exposure.TunnelName, containerID, exposure.Destination)
	}

	// Store exposures for this container
	sem.exposures[containerID] = exposures

	log.Printf("Successfully exposed %d services for container %s", len(exposures), containerID)
	return exposures, nil
}

// createServiceExposure creates a single I2P service exposure.
func (sem *ServiceExposureManager) createServiceExposure(containerID string, networkID string, containerIP net.IP, port ExposedPort) (*ServiceExposure, error) {
	// Generate unique tunnel name
	tunnelName := fmt.Sprintf("%s-%s-%d", containerID, port.ServiceName, port.ContainerPort)

	// Create tunnel configuration
	tunnelConfig := &i2p.TunnelConfig{
		Name:        tunnelName,
		Type:        i2p.TunnelTypeServer,
		LocalHost:   containerIP.String(),
		LocalPort:   port.ContainerPort,
		ContainerID: containerID,
		Options:     i2p.DefaultTunnelOptions(),
	}

	// Create the I2P server tunnel
	tunnel, err := sem.tunnelMgr.CreateTunnel(tunnelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create server tunnel for port %d: %w", port.ContainerPort, err)
	}

	// Generate .b32.i2p address from tunnel destination
	b32Address, err := sem.generateB32Address(tunnel.GetConfig().Destination)
	if err != nil {
		// Clean up tunnel on failure
		sem.tunnelMgr.DestroyTunnel(tunnelName)
		return nil, fmt.Errorf("failed to generate .b32.i2p address: %w", err)
	}

	return &ServiceExposure{
		ContainerID: containerID,
		Port:        port,
		Tunnel:      tunnel,
		Destination: b32Address,
		TunnelName:  tunnelName,
	}, nil
}

// generateB32Address generates a .b32.i2p address from an I2P destination.
//
// I2P destinations are base64-encoded, but .b32.i2p addresses use base32 encoding
// with a specific format. This method converts the destination appropriately.
func (sem *ServiceExposureManager) generateB32Address(destination string) (string, error) {
	if destination == "" {
		return "", fmt.Errorf("destination cannot be empty")
	}

	// Hash the destination to generate a consistent shorter address
	// In a real I2P implementation, this would use the actual destination key
	// For now, we'll create a deterministic hash-based address
	hash := sha256.Sum256([]byte(destination))

	// Take first 20 bytes for base32 encoding (similar to I2P's approach)
	b32 := base32.StdEncoding.EncodeToString(hash[:20])

	// Convert to lowercase and remove padding
	b32 = strings.ToLower(strings.TrimRight(b32, "="))

	return fmt.Sprintf("%s.b32.i2p", b32), nil
}

// GetServiceExposures returns all service exposures for a container.
func (sem *ServiceExposureManager) GetServiceExposures(containerID string) []*ServiceExposure {
	sem.mutex.RLock()
	defer sem.mutex.RUnlock()

	exposures, exists := sem.exposures[containerID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]*ServiceExposure, len(exposures))
	copy(result, exposures)
	return result
}

// CleanupServices removes all service exposures for a container.
//
// This method should be called when a container is being removed to clean up
// associated I2P server tunnels and free resources.
func (sem *ServiceExposureManager) CleanupServices(containerID string) error {
	if containerID == "" {
		return fmt.Errorf("container ID cannot be empty")
	}

	sem.mutex.Lock()
	defer sem.mutex.Unlock()

	exposures, exists := sem.exposures[containerID]
	if !exists {
		return nil // Nothing to clean up
	}

	var errors []string

	// Clean up all tunnels for this container
	for _, exposure := range exposures {
		if err := sem.tunnelMgr.DestroyTunnel(exposure.TunnelName); err != nil {
			errors = append(errors, fmt.Sprintf("failed to destroy tunnel %s: %v", exposure.TunnelName, err))
		}
	}

	// Remove exposures from tracking
	delete(sem.exposures, containerID)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	log.Printf("Successfully cleaned up %d service exposures for container %s", len(exposures), containerID)
	return nil
}

// Shutdown gracefully shuts down the service exposure manager.
func (sem *ServiceExposureManager) Shutdown() error {
	sem.cancel()

	sem.mutex.Lock()
	defer sem.mutex.Unlock()

	var errors []string

	// Clean up all exposures
	for containerID := range sem.exposures {
		if err := sem.CleanupServices(containerID); err != nil {
			errors = append(errors, fmt.Sprintf("failed to cleanup services for container %s: %v", containerID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errors, "; "))
	}

	log.Printf("ServiceExposureManager shutdown complete")
	return nil
}
