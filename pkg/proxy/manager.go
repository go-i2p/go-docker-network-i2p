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

	interceptor := NewTrafficInterceptor(config.ContainerSubnet, config.SOCKSPort, config.DNSPort)
	socksProxy := NewSOCKSProxy(config.SOCKSBindAddr, tunnelManager)
	dnsResolver := NewI2PDNSResolver(config.DNSBindAddr)

	return &ProxyManager{
		interceptor:   interceptor,
		socksProxy:    socksProxy,
		dnsResolver:   dnsResolver,
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

// GetConfig returns the current proxy configuration.
func (pm *ProxyManager) GetConfig() *ProxyConfig {
	return pm.config
}
