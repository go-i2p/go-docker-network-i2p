// Package plugin provides network lifecycle management for I2P Docker networks.
//
// This file implements Docker's Container Network Model (CNM) network lifecycle
// operations, integrating I2P connectivity with Docker's networking system.
package plugin

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
	"github.com/go-i2p/go-docker-network-i2p/pkg/proxy"
	"github.com/go-i2p/go-docker-network-i2p/pkg/service"
)

// I2PNetwork represents an I2P network managed by the plugin.
//
// Each I2P network provides isolated networking for containers that need
// to communicate over I2P. Networks manage IP allocation and route traffic
// through I2P tunnels.
type I2PNetwork struct {
	// ID uniquely identifies this network in Docker
	ID string

	// Name is the human-readable network name
	Name string

	// Subnet defines the IP address range for this network
	Subnet *net.IPNet

	// Gateway is the gateway IP address for this network
	Gateway net.IP

	// TunnelManager handles I2P tunnel creation and management
	TunnelManager *i2p.TunnelManager

	// Endpoints tracks active endpoints (containers) on this network
	Endpoints map[string]*I2PEndpoint

	// IPAllocator manages IP address allocation for containers
	IPAllocator *IPAllocator

	// mutex protects concurrent access to network state
	mutex sync.RWMutex
}

// I2PEndpoint represents a container endpoint on an I2P network.
//
// Endpoints provide the connection point between containers and the I2P network,
// managing IP addresses and I2P tunnel configuration.
type I2PEndpoint struct {
	// ID uniquely identifies this endpoint
	ID string

	// NetworkID identifies the network this endpoint belongs to
	NetworkID string

	// ContainerID identifies the container using this endpoint
	ContainerID string

	// IPAddress is the assigned IP address for this endpoint
	IPAddress net.IP

	// MacAddress is the assigned MAC address for this endpoint
	MacAddress string

	// ClientTunnels are I2P client tunnels for outbound connections
	ClientTunnels map[string]*i2p.Tunnel

	// ServerTunnels are I2P server tunnels for inbound connections
	ServerTunnels map[string]*i2p.Tunnel
}

// NetworkManager manages I2P networks and their lifecycle.
//
// The NetworkManager maintains network state and coordinates between Docker's
// network operations and I2P tunnel management. It ensures proper cleanup
// and resource management throughout the network lifecycle.
type NetworkManager struct {
	// networks tracks all active I2P networks
	networks map[string]*I2PNetwork

	// tunnelMgr provides I2P tunnel management capabilities
	tunnelMgr *i2p.TunnelManager

	// proxyMgr handles transparent I2P proxying for containers
	proxyMgr *proxy.ProxyManager

	// serviceMgr handles I2P service exposure for containers
	serviceMgr *service.ServiceExposureManager

	// defaultSubnet defines the base subnet for I2P networks
	defaultSubnet *net.IPNet

	// mutex protects concurrent access to network manager state
	mutex sync.RWMutex
}

// NewNetworkManager creates a new network manager for I2P networks.
//
// The manager requires a TunnelManager to handle I2P connectivity for networks.
func NewNetworkManager(tunnelMgr *i2p.TunnelManager) (*NetworkManager, error) {
	if tunnelMgr == nil {
		return nil, fmt.Errorf("tunnel manager cannot be nil")
	}

	// Define the default subnet for I2P networks
	_, defaultSubnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		return nil, fmt.Errorf("failed to parse default subnet: %w", err)
	}

	// Create proxy configuration with default settings
	proxyConfig := proxy.DefaultProxyConfig(defaultSubnet)

	// Create proxy manager for transparent I2P proxying
	proxyMgr := proxy.NewProxyManager(proxyConfig, tunnelMgr)

	// Create service exposure manager for I2P service exposure
	serviceMgr, err := service.NewServiceExposureManager(tunnelMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to create service exposure manager: %w", err)
	}

	return &NetworkManager{
		networks:      make(map[string]*I2PNetwork),
		tunnelMgr:     tunnelMgr,
		proxyMgr:      proxyMgr,
		serviceMgr:    serviceMgr,
		defaultSubnet: defaultSubnet,
	}, nil
}

// CreateNetwork creates a new I2P network.
//
// This method implements Docker's CreateNetwork operation, setting up the
// network infrastructure including IP allocation and I2P tunnel management.
func (nm *NetworkManager) CreateNetwork(networkID string, options map[string]interface{}, ipamData []IPAMData) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Validate network ID
	if networkID == "" {
		return fmt.Errorf("network ID cannot be empty")
	}

	// Check if network already exists
	if _, exists := nm.networks[networkID]; exists {
		return fmt.Errorf("network %s already exists", networkID)
	}

	log.Printf("Creating I2P network %s", networkID)

	// Determine subnet for this network
	subnet, gateway, err := nm.allocateNetworkSubnet(ipamData)
	if err != nil {
		return fmt.Errorf("failed to allocate network subnet: %w", err)
	}

	// Create tunnel manager for this network
	tunnelManager := nm.tunnelMgr

	// Create IP allocator for this network
	ipAllocator := NewIPAllocator(subnet, gateway)

	// Create the network
	network := &I2PNetwork{
		ID:            networkID,
		Name:          getNetworkName(options),
		Subnet:        subnet,
		Gateway:       gateway,
		TunnelManager: tunnelManager,
		Endpoints:     make(map[string]*I2PEndpoint),
		IPAllocator:   ipAllocator,
	}

	// Store the network
	nm.networks[networkID] = network

	// Start proxy manager if this is the first network
	if len(nm.networks) == 1 && !nm.proxyMgr.IsRunning() {
		if err := nm.proxyMgr.Start(); err != nil {
			// Clean up the network if proxy start fails
			delete(nm.networks, networkID)
			return fmt.Errorf("failed to start proxy manager: %w", err)
		}
		log.Printf("Started proxy manager for transparent I2P proxying")
	}

	log.Printf("Successfully created I2P network %s with subnet %s", networkID, subnet)
	return nil
}

// DeleteNetwork removes an I2P network and cleans up all resources.
//
// This method implements Docker's DeleteNetwork operation, ensuring proper
// cleanup of I2P tunnels and network resources.
func (nm *NetworkManager) DeleteNetwork(networkID string) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Validate network ID
	if networkID == "" {
		return fmt.Errorf("network ID cannot be empty")
	}

	network, exists := nm.networks[networkID]
	if !exists {
		return fmt.Errorf("network %s not found", networkID)
	}

	log.Printf("Deleting I2P network %s", networkID)

	// Clean up all endpoints first
	network.mutex.Lock()
	for endpointID := range network.Endpoints {
		if err := nm.deleteEndpointInternal(network, endpointID); err != nil {
			log.Printf("Warning: Failed to clean up endpoint %s: %v", endpointID, err)
		}
	}
	network.mutex.Unlock()

	// Clean up all I2P tunnels
	if err := network.TunnelManager.DestroyAllTunnels(); err != nil {
		log.Printf("Warning: Failed to destroy all tunnels: %v", err)
	}

	// Remove network from manager
	delete(nm.networks, networkID)

	// Stop proxy manager if this was the last network
	if len(nm.networks) == 0 && nm.proxyMgr.IsRunning() {
		if err := nm.proxyMgr.Stop(); err != nil {
			log.Printf("Warning: Failed to stop proxy manager: %v", err)
		} else {
			log.Printf("Stopped proxy manager (no networks remaining)")
		}
	}

	log.Printf("Successfully deleted I2P network %s", networkID)
	return nil
}

// GetNetwork retrieves a network by ID.
//
// Returns the network if it exists, or nil if not found.
func (nm *NetworkManager) GetNetwork(networkID string) *I2PNetwork {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	return nm.networks[networkID]
}

// ListNetworks returns a list of all network IDs.
//
// This provides visibility into active I2P networks for debugging and monitoring.
func (nm *NetworkManager) ListNetworks() []string {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	var networks []string
	for networkID := range nm.networks {
		networks = append(networks, networkID)
	}
	return networks
}

// CreateEndpoint creates a new endpoint for a container on an I2P network.
//
// This method implements Docker's CreateEndpoint operation, setting up
// the endpoint configuration but not yet connecting it to the network.
func (nm *NetworkManager) CreateEndpoint(networkID, endpointID string, options map[string]interface{}) (*I2PEndpoint, error) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Validate inputs
	if networkID == "" {
		return nil, fmt.Errorf("network ID cannot be empty")
	}
	if endpointID == "" {
		return nil, fmt.Errorf("endpoint ID cannot be empty")
	}

	// Get the network
	network, exists := nm.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network %s not found", networkID)
	}

	// Check if endpoint already exists
	if _, exists := network.Endpoints[endpointID]; exists {
		return nil, fmt.Errorf("endpoint %s already exists on network %s", endpointID, networkID)
	}

	log.Printf("Creating I2P endpoint %s on network %s", endpointID, networkID)

	// Allocate IP address for the endpoint
	ipAddr, err := network.IPAllocator.AllocateIP()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP address: %w", err)
	}

	// Generate MAC address for the endpoint
	macAddr := generateMACAddress(ipAddr)

	// Create the endpoint structure
	endpoint := &I2PEndpoint{
		ID:            endpointID,
		NetworkID:     networkID,
		IPAddress:     ipAddr,
		MacAddress:    macAddr,
		ClientTunnels: make(map[string]*i2p.Tunnel),
		ServerTunnels: make(map[string]*i2p.Tunnel),
	}

	// Store the endpoint
	network.Endpoints[endpointID] = endpoint

	log.Printf("Successfully created I2P endpoint %s on network %s", endpointID, networkID)
	return endpoint, nil
}

// DeleteEndpoint removes an endpoint from an I2P network.
//
// This method implements Docker's DeleteEndpoint operation, cleaning up
// all I2P resources associated with the endpoint.
func (nm *NetworkManager) DeleteEndpoint(networkID, endpointID string) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Validate inputs
	if networkID == "" {
		return fmt.Errorf("network ID cannot be empty")
	}
	if endpointID == "" {
		return fmt.Errorf("endpoint ID cannot be empty")
	}

	// Get the network
	network, exists := nm.networks[networkID]
	if !exists {
		return fmt.Errorf("network %s not found", networkID)
	}

	// Check if endpoint exists
	if _, exists := network.Endpoints[endpointID]; !exists {
		return fmt.Errorf("endpoint %s not found on network %s", endpointID, networkID)
	}

	log.Printf("Deleting I2P endpoint %s from network %s", endpointID, networkID)

	// Use the existing internal cleanup method
	if err := nm.deleteEndpointInternal(network, endpointID); err != nil {
		return fmt.Errorf("failed to delete endpoint %s: %w", endpointID, err)
	}

	log.Printf("Successfully deleted I2P endpoint %s from network %s", endpointID, networkID)
	return nil
}

// JoinEndpoint connects a container to an I2P network through an endpoint.
//
// This method implements Docker's Join operation, allocating IP addresses
// and setting up I2P tunnels for the container.
func (nm *NetworkManager) JoinEndpoint(networkID, endpointID, containerID, sandboxKey string, options map[string]interface{}) (*I2PEndpoint, error) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Validate inputs
	if networkID == "" {
		return nil, fmt.Errorf("network ID cannot be empty")
	}
	if endpointID == "" {
		return nil, fmt.Errorf("endpoint ID cannot be empty")
	}
	if containerID == "" {
		return nil, fmt.Errorf("container ID cannot be empty")
	}

	// Get the network
	network, exists := nm.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network %s not found", networkID)
	}

	// Get the endpoint
	endpoint, exists := network.Endpoints[endpointID]
	if !exists {
		return nil, fmt.Errorf("endpoint %s not found on network %s", endpointID, networkID)
	}

	// Check if endpoint is already joined
	if endpoint.ContainerID != "" {
		return nil, fmt.Errorf("endpoint %s is already joined to container %s", endpointID, endpoint.ContainerID)
	}

	log.Printf("Joining container %s to I2P network %s via endpoint %s", containerID, networkID, endpointID)

	// Update endpoint with container information
	endpoint.ContainerID = containerID

	// Detect and expose I2P services for this container
	if options != nil {
		exposedPorts, err := nm.serviceMgr.DetectExposedPorts(containerID, options)
		if err != nil {
			log.Printf("Warning: Failed to detect exposed ports for container %s: %v", containerID, err)
		} else if len(exposedPorts) > 0 {
			log.Printf("Container %s has %d exposed ports, creating I2P service exposures", containerID, len(exposedPorts))

			exposures, err := nm.serviceMgr.ExposeServices(containerID, networkID, endpoint.IPAddress, exposedPorts)
			if err != nil {
				log.Printf("Warning: Failed to expose services for container %s: %v", containerID, err)
			} else {
				log.Printf("Successfully exposed %d I2P services for container %s", len(exposures), containerID)

				// Log the .b32.i2p addresses for user visibility
				for _, exposure := range exposures {
					log.Printf("Service %s:%d exposed as %s", containerID, exposure.Port.ContainerPort, exposure.Destination)
				}
			}
		}
	}

	log.Printf("Container %s joined I2P network %s with IP %s via endpoint %s",
		containerID, networkID, endpoint.IPAddress.String(), endpointID)

	return endpoint, nil
}

// LeaveEndpoint disconnects a container from an I2P network.
//
// This method implements Docker's Leave operation, cleaning up
// IP allocations but preserving the endpoint for potential reuse.
func (nm *NetworkManager) LeaveEndpoint(networkID, endpointID string) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Validate inputs
	if networkID == "" {
		return fmt.Errorf("network ID cannot be empty")
	}
	if endpointID == "" {
		return fmt.Errorf("endpoint ID cannot be empty")
	}

	// Get the network
	network, exists := nm.networks[networkID]
	if !exists {
		return fmt.Errorf("network %s not found", networkID)
	}

	// Get the endpoint
	endpoint, exists := network.Endpoints[endpointID]
	if !exists {
		return fmt.Errorf("endpoint %s not found on network %s", endpointID, networkID)
	}

	// Check if endpoint is actually joined
	if endpoint.ContainerID == "" {
		return nil // Already left
	}

	log.Printf("Container %s leaving I2P network %s via endpoint %s",
		endpoint.ContainerID, networkID, endpointID)

	// Clean up I2P tunnels for this endpoint
	for tunnelName, tunnel := range endpoint.ClientTunnels {
		if err := network.TunnelManager.DestroyTunnel(tunnel.GetConfig().Name); err != nil {
			log.Printf("Warning: Failed to destroy client tunnel %s: %v", tunnelName, err)
		}
	}
	endpoint.ClientTunnels = make(map[string]*i2p.Tunnel)

	for tunnelName, tunnel := range endpoint.ServerTunnels {
		if err := network.TunnelManager.DestroyTunnel(tunnel.GetConfig().Name); err != nil {
			log.Printf("Warning: Failed to destroy server tunnel %s: %v", tunnelName, err)
		}
	}
	endpoint.ServerTunnels = make(map[string]*i2p.Tunnel)

	// Check if this is the last endpoint for the container
	containerID := endpoint.ContainerID
	hasOtherEndpoints := false
	for _, ep := range network.Endpoints {
		if ep.ContainerID == containerID && ep.ID != endpointID {
			hasOtherEndpoints = true
			break
		}
	}

	// Clean up I2P service exposures if this was the last endpoint
	if !hasOtherEndpoints {
		if err := nm.serviceMgr.CleanupServices(containerID); err != nil {
			log.Printf("Warning: Failed to cleanup I2P services for container %s: %v", containerID, err)
		}
	}

	// Clean up container session if this was the last endpoint
	if !hasOtherEndpoints {
		if err := network.TunnelManager.DestroyContainerSession(containerID); err != nil {
			log.Printf("Warning: Failed to destroy container session for %s: %v", containerID, err)
		}
	}

	// Release IP address
	if endpoint.IPAddress != nil {
		network.IPAllocator.ReleaseIP(endpoint.IPAddress)
		endpoint.IPAddress = nil
	}

	// Clear container information but keep endpoint for reuse
	endpoint.ContainerID = ""
	endpoint.MacAddress = ""

	log.Printf("Container %s left I2P network %s via endpoint %s",
		containerID, networkID, endpointID)

	return nil
}

// GetEndpoint retrieves an endpoint by ID from a network.
//
// This method provides access to endpoint information for debugging and monitoring.
func (nm *NetworkManager) GetEndpoint(networkID, endpointID string) (*I2PEndpoint, error) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	// Get the network
	network, exists := nm.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network %s not found", networkID)
	}

	// Get the endpoint
	endpoint, exists := network.Endpoints[endpointID]
	if !exists {
		return nil, fmt.Errorf("endpoint %s not found on network %s", endpointID, networkID)
	}

	return endpoint, nil
}

// generateMACAddress generates a MAC address based on IP address.
//
// This ensures consistent MAC addresses for the same IP allocation.
func generateMACAddress(ip net.IP) string {
	// Use a fixed prefix for I2P networks and derive from IP
	// Format: 02:42:XX:XX:XX:XX where XX comes from IP
	ipv4 := ip.To4()
	if ipv4 == nil {
		// Fallback for IPv6 or invalid IP
		return "02:42:00:00:00:01"
	}

	return fmt.Sprintf("02:42:%02x:%02x:%02x:%02x",
		ipv4[0], ipv4[1], ipv4[2], ipv4[3])
}

// allocateNetworkSubnet determines the subnet and gateway for a new network.
//
// This method handles IPAM (IP Address Management) data from Docker and
// allocates appropriate network ranges for I2P networks.
func (nm *NetworkManager) allocateNetworkSubnet(ipamData []IPAMData) (*net.IPNet, net.IP, error) {
	// If IPAM data is provided, use the first IPv4 pool
	if len(ipamData) > 0 {
		for _, data := range ipamData {
			if data.Pool != "" {
				// Parse the provided subnet
				_, subnet, err := net.ParseCIDR(data.Pool)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid subnet in IPAM data: %w", err)
				}

				// Use provided gateway or calculate default
				var gateway net.IP
				if data.Gateway != "" {
					gateway = net.ParseIP(data.Gateway)
					if gateway == nil {
						return nil, nil, fmt.Errorf("invalid gateway IP: %s", data.Gateway)
					}
				} else {
					// Default to first usable IP in subnet as gateway
					gateway = calculateDefaultGateway(subnet)
				}

				return subnet, gateway, nil
			}
		}
	}

	// No IPAM data provided, allocate from default subnet
	// For simplicity, we'll use /24 subnets within our /16 default
	// In production, this would need more sophisticated allocation
	subnet := &net.IPNet{
		IP:   net.IPv4(172, 20, 1, 0),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}
	gateway := net.IPv4(172, 20, 1, 1)

	return subnet, gateway, nil
}

// calculateDefaultGateway calculates the default gateway IP for a subnet.
//
// Returns the first usable IP address in the subnet (network address + 1).
func calculateDefaultGateway(subnet *net.IPNet) net.IP {
	// Get network address
	network := subnet.IP.Mask(subnet.Mask)

	// Calculate first usable IP (network + 1)
	gateway := make(net.IP, len(network))
	copy(gateway, network)

	// Increment the last byte
	for i := len(gateway) - 1; i >= 0; i-- {
		gateway[i]++
		if gateway[i] != 0 {
			break
		}
	}

	return gateway
}

// getNetworkName extracts the network name from options.
//
// Returns the network name if provided in options, or empty string.
func getNetworkName(options map[string]interface{}) string {
	if name, ok := options["com.docker.network.generic"].(map[string]interface{}); ok {
		if networkName, ok := name["name"].(string); ok {
			return networkName
		}
	}
	return ""
}

// deleteEndpointInternal removes an endpoint from a network (internal helper).
//
// This is called during network cleanup and assumes locks are already held.
func (nm *NetworkManager) deleteEndpointInternal(network *I2PNetwork, endpointID string) error {
	endpoint, exists := network.Endpoints[endpointID]
	if !exists {
		return nil // Already deleted
	}

	log.Printf("Cleaning up endpoint %s on network %s", endpointID, network.ID)

	// Clean up I2P tunnels for this endpoint
	for _, tunnel := range endpoint.ClientTunnels {
		if err := network.TunnelManager.DestroyTunnel(tunnel.GetConfig().Name); err != nil {
			log.Printf("Warning: Failed to destroy client tunnel: %v", err)
		}
	}

	for _, tunnel := range endpoint.ServerTunnels {
		if err := network.TunnelManager.DestroyTunnel(tunnel.GetConfig().Name); err != nil {
			log.Printf("Warning: Failed to destroy server tunnel: %v", err)
		}
	}

	// Clean up container session if this was the last endpoint for the container
	if endpoint.ContainerID != "" {
		hasOtherEndpoints := false
		for _, ep := range network.Endpoints {
			if ep.ContainerID == endpoint.ContainerID && ep.ID != endpointID {
				hasOtherEndpoints = true
				break
			}
		}

		if !hasOtherEndpoints {
			if err := network.TunnelManager.DestroyContainerSession(endpoint.ContainerID); err != nil {
				log.Printf("Warning: Failed to destroy container session: %v", err)
			}
		}
	}

	// Release IP address
	if endpoint.IPAddress != nil {
		network.IPAllocator.ReleaseIP(endpoint.IPAddress)
	}

	// Remove endpoint
	delete(network.Endpoints, endpointID)

	return nil
}

// Shutdown gracefully shuts down the NetworkManager and all associated resources.
//
// This method should be called when the plugin is being stopped to ensure
// proper cleanup of all networks, proxy services, and I2P connections.
func (nm *NetworkManager) Shutdown() error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	log.Printf("Shutting down NetworkManager...")

	// Stop proxy manager first
	if nm.proxyMgr.IsRunning() {
		if err := nm.proxyMgr.Stop(); err != nil {
			log.Printf("Warning: Failed to stop proxy manager during shutdown: %v", err)
		}
	}

	// Stop service exposure manager
	if err := nm.serviceMgr.Shutdown(); err != nil {
		log.Printf("Warning: Failed to shutdown service exposure manager: %v", err)
	}

	// Clean up all networks
	for networkID := range nm.networks {
		if err := nm.DeleteNetwork(networkID); err != nil {
			log.Printf("Warning: Failed to delete network %s during shutdown: %v", networkID, err)
		}
	}

	log.Printf("NetworkManager shutdown complete")
	return nil
}
