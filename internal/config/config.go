// Package config provides configuration management for the I2P Docker network plugin.
//
// This package handles loading configuration from environment variables,
// configuration files, and command-line flags to configure I2P connectivity
// and network plugin behavior.
package config

import (
	"encoding/json"
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

// LoadFromFile loads configuration from a JSON file.
//
// This method reads a JSON configuration file and merges it with the existing
// configuration. Only fields present in the JSON file will override existing values.
//
// Configuration Precedence Order:
//  1. Command-line flags (highest priority, applied in main.go)
//  2. Environment variables (LoadFromEnvironment)
//  3. Configuration file (this method)
//  4. Default values (lowest priority, from DefaultConfig)
//
// Example usage:
//
//	cfg := DefaultConfig()
//	if err := cfg.LoadFromFile("/etc/i2p-network/config.json"); err != nil {
//	    return err
//	}
//	if err := cfg.LoadFromEnvironment(); err != nil {
//	    return err
//	}
func (c *Config) LoadFromFile(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("configuration file path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s", filePath)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file %s: %w", filePath, err)
	}

	// Parse JSON
	var fileConfig Config
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse configuration file %s: %w", filePath, err)
	}

	// Merge configuration - only override non-zero values from file
	// This allows partial configuration files that only specify some fields

	// Plugin configuration
	if fileConfig.Plugin.SocketPath != "" {
		c.Plugin.SocketPath = fileConfig.Plugin.SocketPath
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded PLUGIN_SOCKET_PATH from file: %s", fileConfig.Plugin.SocketPath)
		}
	}

	// Debug flag is merged if explicitly set in file (even if false)
	c.Plugin.Debug = fileConfig.Plugin.Debug
	if c.Plugin.Debug {
		log.Printf("DEBUG: Loaded DEBUG from file: %v", fileConfig.Plugin.Debug)
	}

	if fileConfig.Plugin.NetworkName != "" {
		c.Plugin.NetworkName = fileConfig.Plugin.NetworkName
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded NETWORK_NAME from file: %s", fileConfig.Plugin.NetworkName)
		}
	}

	if fileConfig.Plugin.IPAMSubnet != "" {
		c.Plugin.IPAMSubnet = fileConfig.Plugin.IPAMSubnet
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded IPAM_SUBNET from file: %s", fileConfig.Plugin.IPAMSubnet)
		}
	}

	if fileConfig.Plugin.Gateway != "" {
		c.Plugin.Gateway = fileConfig.Plugin.Gateway
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded GATEWAY from file: %s", fileConfig.Plugin.Gateway)
		}
	}

	// SAM configuration
	if fileConfig.SAM.Host != "" {
		c.SAM.Host = fileConfig.SAM.Host
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded I2P_SAM_HOST from file: %s", fileConfig.SAM.Host)
		}
	}

	if fileConfig.SAM.Port > 0 {
		c.SAM.Port = fileConfig.SAM.Port
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded I2P_SAM_PORT from file: %d", fileConfig.SAM.Port)
		}
	}

	if fileConfig.SAM.Timeout > 0 {
		c.SAM.Timeout = fileConfig.SAM.Timeout
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded I2P_SAM_TIMEOUT from file: %v", fileConfig.SAM.Timeout)
		}
	}

	// Tunnel defaults
	if fileConfig.TunnelDefaults.InboundTunnels > 0 {
		c.TunnelDefaults.InboundTunnels = fileConfig.TunnelDefaults.InboundTunnels
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded InboundTunnels from file: %d", fileConfig.TunnelDefaults.InboundTunnels)
		}
	}

	if fileConfig.TunnelDefaults.OutboundTunnels > 0 {
		c.TunnelDefaults.OutboundTunnels = fileConfig.TunnelDefaults.OutboundTunnels
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded OutboundTunnels from file: %d", fileConfig.TunnelDefaults.OutboundTunnels)
		}
	}

	if fileConfig.TunnelDefaults.InboundLength > 0 {
		c.TunnelDefaults.InboundLength = fileConfig.TunnelDefaults.InboundLength
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded InboundLength from file: %d", fileConfig.TunnelDefaults.InboundLength)
		}
	}

	if fileConfig.TunnelDefaults.OutboundLength > 0 {
		c.TunnelDefaults.OutboundLength = fileConfig.TunnelDefaults.OutboundLength
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded OutboundLength from file: %d", fileConfig.TunnelDefaults.OutboundLength)
		}
	}

	if fileConfig.TunnelDefaults.InboundBackups > 0 {
		c.TunnelDefaults.InboundBackups = fileConfig.TunnelDefaults.InboundBackups
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded InboundBackups from file: %d", fileConfig.TunnelDefaults.InboundBackups)
		}
	}

	if fileConfig.TunnelDefaults.OutboundBackups > 0 {
		c.TunnelDefaults.OutboundBackups = fileConfig.TunnelDefaults.OutboundBackups
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded OutboundBackups from file: %d", fileConfig.TunnelDefaults.OutboundBackups)
		}
	}

	// Boolean fields - only set if explicitly in file
	if fileConfig.TunnelDefaults.EncryptLeaseset {
		c.TunnelDefaults.EncryptLeaseset = fileConfig.TunnelDefaults.EncryptLeaseset
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded EncryptLeaseset from file: %v", fileConfig.TunnelDefaults.EncryptLeaseset)
		}
	}

	if fileConfig.TunnelDefaults.CloseIdle {
		c.TunnelDefaults.CloseIdle = fileConfig.TunnelDefaults.CloseIdle
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded CloseIdle from file: %v", fileConfig.TunnelDefaults.CloseIdle)
		}
	}

	if fileConfig.TunnelDefaults.CloseIdleTime > 0 {
		c.TunnelDefaults.CloseIdleTime = fileConfig.TunnelDefaults.CloseIdleTime
		if c.Plugin.Debug {
			log.Printf("DEBUG: Loaded CloseIdleTime from file: %d", fileConfig.TunnelDefaults.CloseIdleTime)
		}
	}

	if c.Plugin.Debug {
		log.Printf("DEBUG: Successfully loaded configuration from file: %s", filePath)
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
