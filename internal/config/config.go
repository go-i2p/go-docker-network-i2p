// Package config provides configuration management for the I2P Docker network plugin.
//
// This package handles loading configuration from environment variables,
// configuration files, and command-line flags to configure I2P connectivity
// and network plugin behavior.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

// Config represents the complete configuration for the I2P network plugin.
type Config struct {
	// Plugin configuration
	Plugin PluginConfig `json:"plugin"`

	// I2P SAM configuration
	SAM i2p.SAMConfig `json:"sam"`

	// Default tunnel options
	TunnelDefaults i2p.TunnelOptions `json:"tunnel_defaults"`
}

// PluginConfig contains plugin-specific configuration.
type PluginConfig struct {
	// SocketPath is the Unix socket path for plugin communication
	SocketPath string `json:"socket_path"`

	// Debug enables debug logging
	Debug bool `json:"debug"`

	// NetworkName is the default name for I2P networks
	NetworkName string `json:"network_name"`

	// IPAMSubnet is the default subnet for container IP allocation
	IPAMSubnet string `json:"ipam_subnet"`

	// Gateway is the default gateway IP for I2P networks
	Gateway string `json:"gateway"`
}

// DefaultConfig returns a default configuration.
//
// This provides the lowest-priority configuration values, which can be
// overridden by environment variables (via LoadFromEnvironment) or
// command-line flags (applied in main.go after config initialization).
//
// Configuration loading order:
//  1. Start with defaults (this function)
//  2. Override with environment variables (LoadFromEnvironment)
//  3. Override with command-line flags (main.go)
func DefaultConfig() *Config {
	return &Config{
		Plugin: PluginConfig{
			SocketPath:  "/run/docker/plugins/i2p-network.sock",
			Debug:       false,
			NetworkName: "i2p",
			IPAMSubnet:  "172.20.0.0/16",
			Gateway:     "172.20.0.1",
		},
		SAM:            *i2p.DefaultSAMConfig(),
		TunnelDefaults: i2p.DefaultTunnelOptions(),
	}
}

// LoadFromEnvironment loads configuration from environment variables.
//
// This method updates the configuration with values from environment
// variables, allowing for container-friendly configuration management.
//
// Configuration Precedence Order:
//  1. Command-line flags (highest priority, applied in main.go)
//  2. Environment variables (this method)
//  3. Default values (lowest priority, from DefaultConfig)
//
// Example: If both PLUGIN_SOCKET_PATH and -sock flag are specified,
// the -sock flag value will be used as it's applied after this method.
func (c *Config) LoadFromEnvironment() error {
	// Plugin configuration
	if sockPath := os.Getenv("PLUGIN_SOCKET_PATH"); sockPath != "" {
		if c.Plugin.Debug {
			log.Printf("DEBUG: Applying PLUGIN_SOCKET_PATH from environment: %s", sockPath)
		}
		c.Plugin.SocketPath = sockPath
	}

	if debug := os.Getenv("DEBUG"); debug != "" {
		c.Plugin.Debug = parseBool(debug, c.Plugin.Debug)
		if c.Plugin.Debug {
			log.Printf("DEBUG: Debug mode enabled via DEBUG environment variable")
		}
	}

	if networkName := os.Getenv("NETWORK_NAME"); networkName != "" {
		if c.Plugin.Debug {
			log.Printf("DEBUG: Applying NETWORK_NAME from environment: %s", networkName)
		}
		c.Plugin.NetworkName = networkName
	}

	if subnet := os.Getenv("IPAM_SUBNET"); subnet != "" {
		if c.Plugin.Debug {
			log.Printf("DEBUG: Applying IPAM_SUBNET from environment: %s", subnet)
		}
		c.Plugin.IPAMSubnet = subnet
	}

	if gateway := os.Getenv("GATEWAY"); gateway != "" {
		if c.Plugin.Debug {
			log.Printf("DEBUG: Applying GATEWAY from environment: %s", gateway)
		}
		c.Plugin.Gateway = gateway
	}

	// I2P SAM configuration
	if host := os.Getenv("I2P_SAM_HOST"); host != "" {
		if c.Plugin.Debug {
			log.Printf("DEBUG: Applying I2P_SAM_HOST from environment: %s", host)
		}
		c.SAM.Host = host
	}

	if portStr := os.Getenv("I2P_SAM_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 && port <= 65535 {
			if c.Plugin.Debug {
				log.Printf("DEBUG: Applying I2P_SAM_PORT from environment: %d", port)
			}
			c.SAM.Port = port
		}
	}

	if timeoutStr := os.Getenv("I2P_SAM_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil && timeout > 0 {
			c.SAM.Timeout = timeout
		}
	}

	if username := os.Getenv("I2P_SAM_USERNAME"); username != "" {
		c.SAM.Username = username
	}

	if password := os.Getenv("I2P_SAM_PASSWORD"); password != "" {
		c.SAM.Password = password
	}

	// Tunnel defaults
	if inTunnels := os.Getenv("I2P_INBOUND_TUNNELS"); inTunnels != "" {
		if val, err := strconv.Atoi(inTunnels); err == nil && val > 0 {
			c.TunnelDefaults.InboundTunnels = val
		}
	}

	if outTunnels := os.Getenv("I2P_OUTBOUND_TUNNELS"); outTunnels != "" {
		if val, err := strconv.Atoi(outTunnels); err == nil && val > 0 {
			c.TunnelDefaults.OutboundTunnels = val
		}
	}

	if inLength := os.Getenv("I2P_INBOUND_LENGTH"); inLength != "" {
		if val, err := strconv.Atoi(inLength); err == nil && val > 0 {
			c.TunnelDefaults.InboundLength = val
		}
	}

	if outLength := os.Getenv("I2P_OUTBOUND_LENGTH"); outLength != "" {
		if val, err := strconv.Atoi(outLength); err == nil && val > 0 {
			c.TunnelDefaults.OutboundLength = val
		}
	}

	if encryptLS := os.Getenv("I2P_ENCRYPT_LEASESET"); encryptLS != "" {
		c.TunnelDefaults.EncryptLeaseset = parseBool(encryptLS, c.TunnelDefaults.EncryptLeaseset)
	}

	if closeIdle := os.Getenv("I2P_CLOSE_IDLE"); closeIdle != "" {
		c.TunnelDefaults.CloseIdle = parseBool(closeIdle, c.TunnelDefaults.CloseIdle)
	}

	if idleTime := os.Getenv("I2P_CLOSE_IDLE_TIME"); idleTime != "" {
		if val, err := strconv.Atoi(idleTime); err == nil && val > 0 {
			c.TunnelDefaults.CloseIdleTime = val
		}
	}

	return nil
}

// Validate validates the configuration for correctness.
func (c *Config) Validate() error {
	if c.Plugin.Debug {
		log.Printf("DEBUG: Validating configuration...")
		log.Printf("DEBUG: Socket path: %s", c.Plugin.SocketPath)
		log.Printf("DEBUG: Network name: %s", c.Plugin.NetworkName)
		log.Printf("DEBUG: IPAM subnet: %s", c.Plugin.IPAMSubnet)
		log.Printf("DEBUG: Gateway: %s", c.Plugin.Gateway)
		log.Printf("DEBUG: SAM host: %s", c.SAM.Host)
		log.Printf("DEBUG: SAM port: %d", c.SAM.Port)
	}

	// Validate plugin configuration
	if c.Plugin.SocketPath == "" {
		return fmt.Errorf("plugin socket path cannot be empty")
	}

	if c.Plugin.NetworkName == "" {
		return fmt.Errorf("network name cannot be empty")
	}

	if c.Plugin.IPAMSubnet == "" {
		return fmt.Errorf("IPAM subnet cannot be empty")
	}

	if c.Plugin.Gateway == "" {
		return fmt.Errorf("gateway cannot be empty")
	}

	// Validate SAM configuration
	if c.SAM.Host == "" {
		return fmt.Errorf("SAM host cannot be empty")
	}

	if c.SAM.Port <= 0 || c.SAM.Port > 65535 {
		return fmt.Errorf("SAM port must be between 1 and 65535, got %d", c.SAM.Port)
	}

	if c.SAM.Timeout <= 0 {
		return fmt.Errorf("SAM timeout must be positive, got %v", c.SAM.Timeout)
	}

	// Validate tunnel defaults
	if c.TunnelDefaults.InboundTunnels <= 0 {
		return fmt.Errorf("inbound tunnels must be positive, got %d", c.TunnelDefaults.InboundTunnels)
	}

	if c.TunnelDefaults.OutboundTunnels <= 0 {
		return fmt.Errorf("outbound tunnels must be positive, got %d", c.TunnelDefaults.OutboundTunnels)
	}

	if c.TunnelDefaults.InboundLength <= 0 {
		return fmt.Errorf("inbound length must be positive, got %d", c.TunnelDefaults.InboundLength)
	}

	if c.TunnelDefaults.OutboundLength <= 0 {
		return fmt.Errorf("outbound length must be positive, got %d", c.TunnelDefaults.OutboundLength)
	}

	if c.TunnelDefaults.CloseIdleTime <= 0 {
		return fmt.Errorf("close idle time must be positive, got %d", c.TunnelDefaults.CloseIdleTime)
	}

	if c.Plugin.Debug {
		log.Printf("DEBUG: Configuration validation successful")
	}

	return nil
}

// GetSAMConfig returns the SAM configuration.
func (c *Config) GetSAMConfig() *i2p.SAMConfig {
	return &c.SAM
}

// GetTunnelDefaults returns the default tunnel options.
func (c *Config) GetTunnelDefaults() *i2p.TunnelOptions {
	return &c.TunnelDefaults
}

// parseBool parses a string as a boolean with a default fallback.
func parseBool(s string, defaultValue bool) bool {
	switch s {
	case "true", "1", "yes", "on", "enable", "enabled":
		return true
	case "false", "0", "no", "off", "disable", "disabled":
		return false
	default:
		return defaultValue
	}
}
