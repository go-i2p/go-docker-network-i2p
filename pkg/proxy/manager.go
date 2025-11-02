package proxy

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

// ProxyManager coordinates transparent I2P proxying for Docker networks.
//
// The ProxyManager integrates traffic interception, SOCKS proxying, and DNS
// resolution to provide transparent I2P connectivity for Docker containers.
type ProxyManager struct {
	// interceptor manages iptables rules for traffic interception
	interceptor *TrafficInterceptor
	// socksProxy handles SOCKS5 connections and I2P routing
	socksProxy *SOCKSProxy
	// dnsResolver provides DNS resolution for I2P domains
	dnsResolver *I2PDNSResolver
	// trafficFilter provides traffic filtering and monitoring
	trafficFilter *TrafficFilter
	// tunnelManager manages I2P tunnels
	tunnelManager *i2p.TunnelManager
	// config holds proxy configuration
	config *ProxyConfig
	// ctx is the context for proxy operation
	ctx context.Context
	// cancel cancels the proxy context
	cancel context.CancelFunc
	// wg tracks running services
	wg sync.WaitGroup
}

// ProxyConfig holds configuration for the proxy manager.
type ProxyConfig struct {
	// ContainerSubnet is the subnet used by I2P containers
	ContainerSubnet *net.IPNet
	// SOCKSPort is the port for the SOCKS proxy
	SOCKSPort int
	// DNSPort is the port for the DNS resolver
	DNSPort int
	// SOCKSBindAddr is the address to bind the SOCKS proxy to
	SOCKSBindAddr string
	// DNSBindAddr is the address to bind the DNS resolver to
	DNSBindAddr string
}

// DefaultProxyConfig returns a default proxy configuration.
func DefaultProxyConfig(subnet *net.IPNet) *ProxyConfig {
	return &ProxyConfig{
		ContainerSubnet: subnet,
		SOCKSPort:       1080,
		DNSPort:         53,
		SOCKSBindAddr:   "127.0.0.1:1080",
		DNSBindAddr:     "127.0.0.1:53",
	}
}

// NewProxyManager creates a new proxy manager with the given configuration.
//
// The proxy manager will use the provided tunnel manager for I2P connectivity
// and configure all proxy components according to the configuration.
func NewProxyManager(config *ProxyConfig, tunnelManager *i2p.TunnelManager) *ProxyManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Create shared traffic filter for all components
	trafficFilter := NewTrafficFilter(DefaultFilterConfig())

	interceptor := NewTrafficInterceptor(config.ContainerSubnet, config.SOCKSPort, config.DNSPort)
	socksProxy := NewSOCKSProxy(config.SOCKSBindAddr, tunnelManager)
	socksProxy.SetTrafficFilter(trafficFilter)
	dnsResolver := NewI2PDNSResolver(config.DNSBindAddr)

	return &ProxyManager{
		interceptor:   interceptor,
		socksProxy:    socksProxy,
		dnsResolver:   dnsResolver,
		trafficFilter: trafficFilter,
		tunnelManager: tunnelManager,
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins all proxy services and sets up traffic interception.
//
// This method starts the SOCKS proxy, DNS resolver, and configures iptables
// rules for transparent traffic interception.
func (pm *ProxyManager) Start() error {
	// Check if iptables is available
	if err := pm.interceptor.IsAvailable(); err != nil {
		return fmt.Errorf("iptables not available: %w", err)
	}

	// Start SOCKS proxy
	pm.wg.Add(1)
	go func() {
		defer pm.wg.Done()
		if err := pm.socksProxy.Start(); err != nil && err != context.Canceled {
			// Log error but don't fail startup
		}
	}()

	// Start DNS resolver
	pm.wg.Add(1)
	go func() {
		defer pm.wg.Done()
		if err := pm.dnsResolver.Start(); err != nil && err != context.Canceled {
			// Log error but don't fail startup
		}
	}()

	// Set up traffic interception
	if err := pm.interceptor.SetupInterception(); err != nil {
		pm.Stop()
		return fmt.Errorf("failed to set up traffic interception: %w", err)
	}

	return nil
}

// Stop gracefully shuts down all proxy services and cleans up iptables rules.
//
// This method stops all running services and removes the iptables rules
// that were set up for traffic interception.
func (pm *ProxyManager) Stop() error {
	pm.cancel()

	var errors []string

	// Clean up traffic interception
	if err := pm.interceptor.CleanupInterception(); err != nil {
		errors = append(errors, fmt.Sprintf("iptables cleanup failed: %v", err))
	}

	// Stop SOCKS proxy
	if err := pm.socksProxy.Stop(); err != nil {
		errors = append(errors, fmt.Sprintf("SOCKS proxy stop failed: %v", err))
	}

	// Stop DNS resolver
	if err := pm.dnsResolver.Stop(); err != nil {
		errors = append(errors, fmt.Sprintf("DNS resolver stop failed: %v", err))
	}

	// Wait for all services to stop
	pm.wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("proxy manager stop errors: %v", errors)
	}

	return nil
}

// IsRunning returns true if the proxy manager is currently running.
func (pm *ProxyManager) IsRunning() bool {
	select {
	case <-pm.ctx.Done():
		return false
	default:
		return true
	}
}

// CheckIptablesAvailability verifies that iptables is available and usable.
//
// This method should be called before creating networks to enforce the security
// requirement that iptables must be available for traffic filtering.
// Returns an error if iptables is not available or cannot be used.
func (pm *ProxyManager) CheckIptablesAvailability() error {
	return pm.interceptor.IsAvailable()
}

// GetConfig returns the current proxy configuration.
func (pm *ProxyManager) GetConfig() *ProxyConfig {
	return pm.config
}

// GetTrafficFilter returns the traffic filter for configuration and monitoring.
func (pm *ProxyManager) GetTrafficFilter() *TrafficFilter {
	return pm.trafficFilter
}

// AddToAllowlist adds a destination to the traffic filter allowlist.
func (pm *ProxyManager) AddToAllowlist(destination string) error {
	return pm.trafficFilter.AddToAllowlist(destination)
}

// AddToBlocklist adds a destination to the traffic filter blocklist.
func (pm *ProxyManager) AddToBlocklist(destination string) error {
	return pm.trafficFilter.AddToBlocklist(destination)
}

// RemoveFromAllowlist removes a destination from the allowlist.
func (pm *ProxyManager) RemoveFromAllowlist(destination string) {
	pm.trafficFilter.RemoveFromAllowlist(destination)
}

// RemoveFromBlocklist removes a destination from the blocklist.
func (pm *ProxyManager) RemoveFromBlocklist(destination string) {
	pm.trafficFilter.RemoveFromBlocklist(destination)
}

// GetTrafficStats returns current traffic statistics.
func (pm *ProxyManager) GetTrafficStats() TrafficStats {
	return pm.trafficFilter.GetStats()
}

// GetRecentTrafficLogs returns recent traffic log entries.
func (pm *ProxyManager) GetRecentTrafficLogs(limit int) []TrafficLogEntry {
	return pm.trafficFilter.GetRecentLogs(limit)
}

// ClearTrafficStats resets all traffic statistics and logs.
func (pm *ProxyManager) ClearTrafficStats() {
	pm.trafficFilter.ClearStats()
}

// GetAllowlist returns the current allowlist
func (pm *ProxyManager) GetAllowlist() []string {
	if pm.trafficFilter != nil {
		return pm.trafficFilter.GetAllowlist()
	}
	return []string{}
}

// GetBlocklist returns the current blocklist
func (pm *ProxyManager) GetBlocklist() []string {
	if pm.trafficFilter != nil {
		return pm.trafficFilter.GetBlocklist()
	}
	return []string{}
}
