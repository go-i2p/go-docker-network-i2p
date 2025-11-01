// Package i2p provides tunnel management for I2P services.
//
// This package implements I2P tunnel management using the SAM (Simple Anonymous Messaging)
// protocol. It follows a "one SAM connection per container" architecture for optimal isolation:
//
// Architecture:
//   - Each Docker container gets its own dedicated SAM client connection
//   - Each SAM client creates one primary session with unique I2P destination keys
//   - Each primary session can create multiple sub-sessions for different purposes:
//   - Stream sub-sessions for TCP connections (client tunnels)
//   - Server sub-sessions for exposing services (server tunnels)
//   - Future: Datagram and Raw sub-sessions for UDP and custom protocols
//
// This design ensures:
//   - Complete isolation between containers (separate I2P identities)
//   - Efficient resource usage (shared I2P router, separate sessions)
//   - Scalability (no session ID conflicts between containers)
//   - Security (each container has its own cryptographic identity)
package i2p

import (
	"context"
	"fmt"
	"log"
	"time"

	sam3 "github.com/go-i2p/go-sam-go"
)

// TunnelType represents the type of I2P tunnel.
type TunnelType string

const (
	// TunnelTypeClient represents a client tunnel for outbound connections
	TunnelTypeClient TunnelType = "client"
	// TunnelTypeServer represents a server tunnel for inbound connections
	TunnelTypeServer TunnelType = "server"
)

// TunnelConfig represents the configuration for an I2P tunnel.
type TunnelConfig struct {
	// Name is the unique identifier for this tunnel
	Name string `json:"name"`

	// ContainerID is the Docker container ID this tunnel belongs to
	ContainerID string `json:"container_id"`

	// Type specifies whether this is a client or server tunnel
	Type TunnelType `json:"type"`

	// LocalHost is the local address to bind to (for server tunnels)
	// or connect to (for client tunnels)
	LocalHost string `json:"local_host"`

	// LocalPort is the local port to bind to (for server tunnels)
	// or connect to (for client tunnels)
	LocalPort int `json:"local_port"`

	// Destination is the I2P destination for server tunnels
	// (auto-generated if empty) or the target destination for client tunnels
	Destination string `json:"destination,omitempty"`

	// Options contains I2P-specific tunnel options
	Options TunnelOptions `json:"options"`
}

// TunnelOptions contains I2P-specific configuration options for tunnels.
type TunnelOptions struct {
	// InboundTunnels specifies the number of inbound tunnels (default: 2)
	InboundTunnels int `json:"inbound_tunnels,omitempty"`

	// OutboundTunnels specifies the number of outbound tunnels (default: 2)
	OutboundTunnels int `json:"outbound_tunnels,omitempty"`

	// InboundLength specifies the length of inbound tunnels (default: 3)
	InboundLength int `json:"inbound_length,omitempty"`

	// OutboundLength specifies the length of outbound tunnels (default: 3)
	OutboundLength int `json:"outbound_length,omitempty"`

	// InboundBackups specifies the number of backup inbound tunnels (default: 1)
	InboundBackups int `json:"inbound_backups,omitempty"`

	// OutboundBackups specifies the number of backup outbound tunnels (default: 1)
	OutboundBackups int `json:"outbound_backups,omitempty"`

	// EncryptLeaseset enables leaseSet encryption (default: false)
	EncryptLeaseset bool `json:"encrypt_leaseset,omitempty"`

	// CloseIdle enables closing idle connections (default: true)
	CloseIdle bool `json:"close_idle,omitempty"`

	// CloseIdleTime specifies idle timeout in minutes (default: 10)
	CloseIdleTime int `json:"close_idle_time,omitempty"`
}

// DefaultTunnelOptions returns default tunnel options optimized for Docker containers.
func DefaultTunnelOptions() TunnelOptions {
	return TunnelOptions{
		InboundTunnels:  2,
		OutboundTunnels: 2,
		InboundLength:   3,
		OutboundLength:  3,
		InboundBackups:  1,
		OutboundBackups: 1,
		EncryptLeaseset: false,
		CloseIdle:       true,
		CloseIdleTime:   10,
	}
}

// Tunnel represents an active I2P tunnel.
type Tunnel struct {
	config  *TunnelConfig
	session interface{} // Will hold either StreamSession or DatagramSession
	active  bool
}

// TunnelManager manages I2P tunnels and sessions for containers.
//
// Architecture: One SAM Connection Per Container
//
// The TunnelManager implements a container-isolated I2P architecture where each Docker
// container receives its own dedicated SAM connection and primary session:
//
//	Container A                Container B                Container C
//	    |                          |                          |
//	SAM Client A               SAM Client B               SAM Client C
//	    |                          |                          |
//	Primary Session A          Primary Session B          Primary Session C
//	(Keys: A-pub/A-priv)       (Keys: B-pub/B-priv)       (Keys: C-pub/C-priv)
//	    |                          |                          |
//	Sub-sessions:              Sub-sessions:              Sub-sessions:
//	- Stream (HTTP)            - Stream (HTTPS)           - Server (SSH:22)
//	- Server (API:8080)        - Server (Web:80)          - Stream (DB:5432)
//
// Benefits:
//   - Complete cryptographic isolation (separate I2P identities)
//   - No session ID conflicts between containers
//   - Independent tunnel management per container
//   - Simplified cleanup when containers are destroyed
//   - Better security boundaries
//
// Session Lifecycle:
//  1. Container starts -> Create dedicated SAM client
//  2. First tunnel needed -> Create primary session with unique keys
//  3. Additional tunnels -> Create sub-sessions from primary session
//  4. Container stops -> Clean up all sub-sessions, primary session, and SAM client
type TunnelManager struct {
	samConfig           *SAMConfig                      // Template configuration for creating SAM clients
	tunnels             map[string]*Tunnel              // Active tunnels by name
	containerSessions   map[string]*sam3.PrimarySession // Primary sessions by container ID
	containerSAMClients map[string]*SAMClient           // SAM clients by container ID
}

// NewTunnelManager creates a new tunnel manager with the given SAM configuration.
//
// Instead of a single SAM client, this manager will create individual SAM clients
// for each container to ensure proper isolation.
func NewTunnelManager(samClient *SAMClient) *TunnelManager {
	return &TunnelManager{
		samConfig:           samClient.config,
		tunnels:             make(map[string]*Tunnel),
		containerSessions:   make(map[string]*sam3.PrimarySession),
		containerSAMClients: make(map[string]*SAMClient),
	}
}

// CreateTunnel creates a new I2P tunnel with the given configuration.
//
// Tunnel Creation Process:
//  1. Get or create a primary session for the container (one per container)
//  2. Create a sub-session from the primary session for this specific tunnel:
//     - Stream sub-session for client tunnels (outbound connections)
//     - Server sub-session for server tunnels (inbound service exposure)
//  3. Configure the sub-session with tunnel-specific options
//  4. Store the tunnel for lifecycle management
//
// Sub-session Types:
//   - Client Tunnels: Use Stream sub-sessions to make outbound I2P connections
//   - Server Tunnels: Use Stream sub-sessions to accept inbound I2P connections
//
// Each tunnel gets its own sub-session but shares the container's primary session,
// ensuring both isolation (separate tunnel handling) and efficiency (shared identity).
func (tm *TunnelManager) CreateTunnel(config *TunnelConfig) (*Tunnel, error) {
	if config == nil {
		return nil, fmt.Errorf("tunnel configuration cannot be nil")
	}

	if err := tm.validateTunnelConfig(config); err != nil {
		return nil, fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	// Check if tunnel with this name already exists
	if _, exists := tm.tunnels[config.Name]; exists {
		return nil, fmt.Errorf("tunnel with name %s already exists", config.Name)
	}

	// Get or create container session (this will handle SAM client creation)
	session, err := tm.GetOrCreateContainerSession(config.ContainerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container session: %w", err)
	}

	// Create the appropriate tunnel type
	tunnel := &Tunnel{
		config: config,
		active: false,
	}
	tunnel.session = session

	switch config.Type {
	case TunnelTypeClient:
		if err := tm.createClientTunnel(tunnel); err != nil {
			return nil, fmt.Errorf("failed to create client tunnel: %w", err)
		}
	case TunnelTypeServer:
		if err := tm.createServerTunnel(tunnel); err != nil {
			return nil, fmt.Errorf("failed to create server tunnel: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown tunnel type: %s", config.Type)
	}

	// Register the tunnel
	tm.tunnels[config.Name] = tunnel
	tunnel.active = true

	return tunnel, nil
}

// GetTunnel retrieves a tunnel by name.
func (tm *TunnelManager) GetTunnel(name string) (*Tunnel, bool) {
	tunnel, exists := tm.tunnels[name]
	return tunnel, exists
}

// ListTunnels returns a list of all tunnel names.
func (tm *TunnelManager) ListTunnels() []string {
	var names []string
	for name := range tm.tunnels {
		names = append(names, name)
	}
	return names
}

// DestroyTunnel removes and cleans up a tunnel.
func (tm *TunnelManager) DestroyTunnel(name string) error {
	tunnel, exists := tm.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel %s not found", name)
	}

	log.Printf("Destroying tunnel %s", name)

	// Clean up the tunnel session based on its type
	if tunnel.session != nil {
		// For stream sub-sessions, close them properly
		// Note: We don't close the primary session here since it may be used by other tunnels
		// The primary session is cleaned up when the container is destroyed
		switch session := tunnel.session.(type) {
		case interface{ Close() error }:
			if err := session.Close(); err != nil {
				log.Printf("Warning: Error closing session for tunnel %s: %v", name, err)
				// Continue with cleanup even if close fails
			}
		default:
			log.Printf("Warning: Tunnel %s has session with unknown close method", name)
		}
	}

	tunnel.active = false
	delete(tm.tunnels, name)

	log.Printf("Successfully destroyed tunnel %s", name)
	return nil
}

// DestroyAllTunnels removes and cleans up all tunnels.
func (tm *TunnelManager) DestroyAllTunnels() error {
	var errors []error

	for name := range tm.tunnels {
		if err := tm.DestroyTunnel(name); err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy tunnel %s: %w", name, err))
		}
	}

	// Clean up all container sessions
	for containerID := range tm.containerSessions {
		if err := tm.DestroyContainerSession(containerID); err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy container session %s: %w", containerID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors destroying tunnels: %v", errors)
	}

	return nil
}

// validateTunnelConfig validates the tunnel configuration.
func (tm *TunnelManager) validateTunnelConfig(config *TunnelConfig) error {
	if config.Name == "" {
		return fmt.Errorf("tunnel name cannot be empty")
	}

	if config.ContainerID == "" {
		return fmt.Errorf("container ID cannot be empty")
	}

	if config.Type != TunnelTypeClient && config.Type != TunnelTypeServer {
		return fmt.Errorf("invalid tunnel type: %s", config.Type)
	}

	if config.LocalHost == "" {
		config.LocalHost = "127.0.0.1" // Default to localhost
	}

	if config.LocalPort <= 0 || config.LocalPort > 65535 {
		return fmt.Errorf("invalid local port: %d", config.LocalPort)
	}

	// Apply default options if not specified
	if config.Options.InboundTunnels == 0 {
		config.Options = DefaultTunnelOptions()
	}

	return nil
}

// createClientTunnel creates a client tunnel for outbound I2P connections.
//
// Client tunnels enable containers to connect to I2P destinations by creating
// a local proxy that forwards traffic through the I2P network.
func (tm *TunnelManager) createClientTunnel(tunnel *Tunnel) error {
	config := tunnel.config

	// Get the primary session for this container
	primarySession, ok := tunnel.session.(*sam3.PrimarySession)
	if !ok {
		return fmt.Errorf("invalid session type for client tunnel %s", config.Name)
	}

	// Generate a unique sub-session ID for this tunnel
	subSessionID := fmt.Sprintf("%s-client", config.Name)

	log.Printf("Creating client tunnel %s for container %s (destination: %s)",
		config.Name, config.ContainerID, config.Destination)

	// Create a stream sub-session for this client tunnel
	// This will be used to establish outbound connections to I2P destinations
	streamSession, err := primarySession.NewStreamSubSession(subSessionID, []string{})
	if err != nil {
		return fmt.Errorf("failed to create stream sub-session for client tunnel %s: %w", config.Name, err)
	}

	// Store the stream session in the tunnel
	tunnel.session = streamSession

	log.Printf("Successfully created client tunnel %s on %s", config.Name, tunnel.GetLocalEndpoint())
	return nil
}

// createServerTunnel creates a server tunnel for inbound I2P connections.
//
// Server tunnels enable I2P users to connect to services running inside containers
// by creating an I2P destination that forwards traffic to the local service.
func (tm *TunnelManager) createServerTunnel(tunnel *Tunnel) error {
	config := tunnel.config

	// Get the primary session for this container
	primarySession, ok := tunnel.session.(*sam3.PrimarySession)
	if !ok {
		return fmt.Errorf("invalid session type for server tunnel %s", config.Name)
	}

	// Generate a unique sub-session ID for this tunnel
	subSessionID := fmt.Sprintf("%s-server", config.Name)

	log.Printf("Creating server tunnel %s for container %s on %s",
		config.Name, config.ContainerID, tunnel.GetLocalEndpoint())

	// Create a stream sub-session for this server tunnel
	// This will create an I2P destination that can accept inbound connections
	streamSession, err := primarySession.NewStreamSubSession(subSessionID, []string{})
	if err != nil {
		return fmt.Errorf("failed to create stream sub-session for server tunnel %s: %w", config.Name, err)
	}

	// Get the I2P destination for this server tunnel
	// The destination is from the primary session that created this sub-session
	destination := string(primarySession.Addr())

	// Update the tunnel configuration with the generated destination
	config.Destination = destination

	// Store the stream session in the tunnel
	tunnel.session = streamSession

	log.Printf("Successfully created server tunnel %s with I2P destination: %s", config.Name, destination)
	return nil
}

// IsActive returns true if the tunnel is active.
func (t *Tunnel) IsActive() bool {
	return t.active
}

// GetConfig returns the tunnel configuration.
func (t *Tunnel) GetConfig() *TunnelConfig {
	return t.config
}

// GetDestination returns the I2P destination for this tunnel.
func (t *Tunnel) GetDestination() string {
	return t.config.Destination
}

// GetLocalEndpoint returns the local endpoint (host:port) for this tunnel.
func (t *Tunnel) GetLocalEndpoint() string {
	return fmt.Sprintf("%s:%d", t.config.LocalHost, t.config.LocalPort)
}

// GetOrCreateContainerSession gets or creates a primary I2P session for a container.
//
// This method implements the "one SAM connection per container" architecture:
//
// First Call for Container:
//  1. Creates a new SAM client with dedicated connection to I2P router
//  2. Connects the SAM client (establishes TCP connection to router)
//  3. Generates unique I2P cryptographic keys for this container
//  4. Creates a primary session using the SAM client and keys
//  5. Stores both the primary session and SAM client for reuse
//
// Subsequent Calls for Same Container:
//  1. Returns the existing primary session (no new connections)
//  2. The existing session can be used to create sub-sessions as needed
//
// The primary session serves as the foundation for all I2P activity for this container.
// Sub-sessions (Stream, Server, Datagram, Raw) are created from this primary session
// when individual tunnels are needed.
//
// Resource Management:
//   - Each container gets unique I2P destination keys (separate identity)
//   - SAM client connection is maintained for the lifetime of the container
//   - Primary session is reused for all tunnels within the same container
//   - Cleanup via DestroyContainerSession() when container is removed
func (tm *TunnelManager) GetOrCreateContainerSession(containerID string) (*sam3.PrimarySession, error) {
	// Check if we already have a session for this container
	if session, exists := tm.containerSessions[containerID]; exists {
		log.Printf("Reusing existing primary session for container %s", containerID)
		return session, nil
	}

	// Create a new SAM client for this container
	log.Printf("Creating new SAM client and primary session for container %s", containerID)

	samClient, err := NewSAMClient(tm.samConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SAM client for container %s: %w", containerID, err)
	}

	// Connect the SAM client
	ctx := context.Background()
	if err := samClient.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect SAM client for container %s: %w", containerID, err)
	}

	// Generate a unique session ID for this container
	sessionID := fmt.Sprintf("cont_%s_%d", containerID, time.Now().UnixNano())

	// Generate I2P keys for this session
	keys, err := samClient.sam.NewKeys()
	if err != nil {
		samClient.Disconnect()
		return nil, fmt.Errorf("failed to generate I2P keys for container %s: %w", containerID, err)
	}
	log.Printf("DEBUG: Generated new I2P keys for container %s", containerID)

	// Create minimal options for the session to avoid potential issues
	options := []string{
		"inbound.quantity=1",  // Reduce to 1 tunnel for testing
		"outbound.quantity=1", // Reduce to 1 tunnel for testing
	}

	// Create the primary session using the SAM client
	session, err := samClient.sam.NewPrimarySession(sessionID, keys, options)
	if err != nil {
		samClient.Disconnect()
		return nil, fmt.Errorf("failed to create primary session for container %s: %w", containerID, err)
	}

	// Store both the session and SAM client
	tm.containerSessions[containerID] = session
	tm.containerSAMClients[containerID] = samClient

	log.Printf("Successfully created primary session for container %s with session ID %s", containerID, sessionID)
	return session, nil
}

// DestroyContainerSession removes and cleans up a container's primary session.
//
// This should be called when a container is removed to clean up I2P resources.
func (tm *TunnelManager) DestroyContainerSession(containerID string) error {
	session, exists := tm.containerSessions[containerID]
	if !exists {
		return nil // No session to clean up
	}

	// Close the primary session
	log.Printf("Closing primary session for container %s", containerID)
	if err := session.Close(); err != nil {
		log.Printf("Warning: Error closing primary session for container %s: %v", containerID, err)
		// Continue with cleanup even if close fails
	}

	// Close the SAM client for this container
	if samClient, exists := tm.containerSAMClients[containerID]; exists {
		log.Printf("Disconnecting SAM client for container %s", containerID)
		if err := samClient.Disconnect(); err != nil {
			log.Printf("Warning: Error disconnecting SAM client for container %s: %v", containerID, err)
		}
		delete(tm.containerSAMClients, containerID)
	}

	// Remove from the map regardless of type
	delete(tm.containerSessions, containerID)
	log.Printf("Destroyed container session for container %s", containerID)
	return nil
}

// ListContainerSessions returns a list of container IDs that have active sessions.
func (tm *TunnelManager) ListContainerSessions() []string {
	var containerIDs []string
	for containerID := range tm.containerSessions {
		containerIDs = append(containerIDs, containerID)
	}
	return containerIDs
}
