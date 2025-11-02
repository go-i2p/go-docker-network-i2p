package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test plugin configuration defaults
	if config.Plugin.SocketPath != "/run/docker/plugins/i2p-network.sock" {
		t.Errorf("Expected default socket path '/run/docker/plugins/i2p-network.sock', got '%s'", config.Plugin.SocketPath)
	}

	if config.Plugin.Debug != false {
		t.Errorf("Expected default debug false, got %v", config.Plugin.Debug)
	}

	if config.Plugin.NetworkName != "i2p" {
		t.Errorf("Expected default network name 'i2p', got '%s'", config.Plugin.NetworkName)
	}

	if config.Plugin.IPAMSubnet != "172.20.0.0/16" {
		t.Errorf("Expected default IPAM subnet '172.20.0.0/16', got '%s'", config.Plugin.IPAMSubnet)
	}

	if config.Plugin.Gateway != "172.20.0.1" {
		t.Errorf("Expected default gateway '172.20.0.1', got '%s'", config.Plugin.Gateway)
	}

	// Test SAM configuration defaults
	if config.SAM.Host != "localhost" {
		t.Errorf("Expected default SAM host 'localhost', got '%s'", config.SAM.Host)
	}

	if config.SAM.Port != 7656 {
		t.Errorf("Expected default SAM port 7656, got %d", config.SAM.Port)
	}

	if config.SAM.Timeout != 30*time.Second {
		t.Errorf("Expected default SAM timeout 30s, got %v", config.SAM.Timeout)
	}

	// Test tunnel defaults
	if config.TunnelDefaults.InboundTunnels != 2 {
		t.Errorf("Expected default inbound tunnels 2, got %d", config.TunnelDefaults.InboundTunnels)
	}

	if config.TunnelDefaults.OutboundTunnels != 2 {
		t.Errorf("Expected default outbound tunnels 2, got %d", config.TunnelDefaults.OutboundTunnels)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{}
	envVars := []string{
		"PLUGIN_SOCKET_PATH", "DEBUG", "NETWORK_NAME", "IPAM_SUBNET", "GATEWAY",
		"I2P_SAM_HOST", "I2P_SAM_PORT", "I2P_SAM_TIMEOUT", "I2P_SAM_USERNAME", "I2P_SAM_PASSWORD",
		"I2P_INBOUND_TUNNELS", "I2P_OUTBOUND_TUNNELS", "I2P_INBOUND_LENGTH", "I2P_OUTBOUND_LENGTH",
		"I2P_ENCRYPT_LEASESET", "I2P_CLOSE_IDLE", "I2P_CLOSE_IDLE_TIME",
	}

	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(*testing.T, *Config)
	}{
		{
			name: "plugin configuration",
			envVars: map[string]string{
				"PLUGIN_SOCKET_PATH": "/custom/path/plugin.sock",
				"DEBUG":              "true",
				"NETWORK_NAME":       "custom-i2p",
				"IPAM_SUBNET":        "192.168.0.0/16",
				"GATEWAY":            "192.168.0.1",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Plugin.SocketPath != "/custom/path/plugin.sock" {
					t.Errorf("Expected socket path '/custom/path/plugin.sock', got '%s'", c.Plugin.SocketPath)
				}
				if !c.Plugin.Debug {
					t.Errorf("Expected debug true, got %v", c.Plugin.Debug)
				}
				if c.Plugin.NetworkName != "custom-i2p" {
					t.Errorf("Expected network name 'custom-i2p', got '%s'", c.Plugin.NetworkName)
				}
				if c.Plugin.IPAMSubnet != "192.168.0.0/16" {
					t.Errorf("Expected IPAM subnet '192.168.0.0/16', got '%s'", c.Plugin.IPAMSubnet)
				}
				if c.Plugin.Gateway != "192.168.0.1" {
					t.Errorf("Expected gateway '192.168.0.1', got '%s'", c.Plugin.Gateway)
				}
			},
		},
		{
			name: "SAM configuration",
			envVars: map[string]string{
				"I2P_SAM_HOST":     "i2p-router.local",
				"I2P_SAM_PORT":     "7657",
				"I2P_SAM_TIMEOUT":  "45s",
				"I2P_SAM_USERNAME": "testuser",
				"I2P_SAM_PASSWORD": "testpass",
			},
			validate: func(t *testing.T, c *Config) {
				if c.SAM.Host != "i2p-router.local" {
					t.Errorf("Expected SAM host 'i2p-router.local', got '%s'", c.SAM.Host)
				}
				if c.SAM.Port != 7657 {
					t.Errorf("Expected SAM port 7657, got %d", c.SAM.Port)
				}
				if c.SAM.Timeout != 45*time.Second {
					t.Errorf("Expected SAM timeout 45s, got %v", c.SAM.Timeout)
				}
				if c.SAM.Username != "testuser" {
					t.Errorf("Expected SAM username 'testuser', got '%s'", c.SAM.Username)
				}
				if c.SAM.Password != "testpass" {
					t.Errorf("Expected SAM password 'testpass', got '%s'", c.SAM.Password)
				}
			},
		},
		{
			name: "tunnel configuration",
			envVars: map[string]string{
				"I2P_INBOUND_TUNNELS":  "5",
				"I2P_OUTBOUND_TUNNELS": "4",
				"I2P_INBOUND_LENGTH":   "2",
				"I2P_OUTBOUND_LENGTH":  "3",
				"I2P_ENCRYPT_LEASESET": "true",
				"I2P_CLOSE_IDLE":       "false",
				"I2P_CLOSE_IDLE_TIME":  "600",
			},
			validate: func(t *testing.T, c *Config) {
				if c.TunnelDefaults.InboundTunnels != 5 {
					t.Errorf("Expected inbound tunnels 5, got %d", c.TunnelDefaults.InboundTunnels)
				}
				if c.TunnelDefaults.OutboundTunnels != 4 {
					t.Errorf("Expected outbound tunnels 4, got %d", c.TunnelDefaults.OutboundTunnels)
				}
				if c.TunnelDefaults.InboundLength != 2 {
					t.Errorf("Expected inbound length 2, got %d", c.TunnelDefaults.InboundLength)
				}
				if c.TunnelDefaults.OutboundLength != 3 {
					t.Errorf("Expected outbound length 3, got %d", c.TunnelDefaults.OutboundLength)
				}
				if !c.TunnelDefaults.EncryptLeaseset {
					t.Errorf("Expected encrypt leaseset true, got %v", c.TunnelDefaults.EncryptLeaseset)
				}
				if c.TunnelDefaults.CloseIdle {
					t.Errorf("Expected close idle false, got %v", c.TunnelDefaults.CloseIdle)
				}
				if c.TunnelDefaults.CloseIdleTime != 600 {
					t.Errorf("Expected close idle time 600, got %d", c.TunnelDefaults.CloseIdleTime)
				}
			},
		},
		{
			name: "invalid values ignored",
			envVars: map[string]string{
				"I2P_SAM_PORT":         "invalid",
				"I2P_SAM_TIMEOUT":      "invalid",
				"I2P_INBOUND_TUNNELS":  "0",
				"I2P_OUTBOUND_TUNNELS": "-1",
			},
			validate: func(t *testing.T, c *Config) {
				// Should keep defaults when invalid values provided
				if c.SAM.Port != 7656 {
					t.Errorf("Expected default SAM port 7656 when invalid provided, got %d", c.SAM.Port)
				}
				if c.SAM.Timeout != 30*time.Second {
					t.Errorf("Expected default SAM timeout 30s when invalid provided, got %v", c.SAM.Timeout)
				}
				if c.TunnelDefaults.InboundTunnels != 2 {
					t.Errorf("Expected default inbound tunnels 2 when invalid provided, got %d", c.TunnelDefaults.InboundTunnels)
				}
				if c.TunnelDefaults.OutboundTunnels != 2 {
					t.Errorf("Expected default outbound tunnels 2 when invalid provided, got %d", c.TunnelDefaults.OutboundTunnels)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Load configuration
			config := DefaultConfig()
			err := config.LoadFromEnvironment()
			if err != nil {
				t.Fatalf("LoadFromEnvironment failed: %v", err)
			}

			// Validate
			tt.validate(t, config)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid configuration",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name:        "empty socket path",
			modify:      func(c *Config) { c.Plugin.SocketPath = "" },
			expectError: true,
			errorMsg:    "plugin socket path cannot be empty",
		},
		{
			name:        "empty network name",
			modify:      func(c *Config) { c.Plugin.NetworkName = "" },
			expectError: true,
			errorMsg:    "network name cannot be empty",
		},
		{
			name:        "empty IPAM subnet",
			modify:      func(c *Config) { c.Plugin.IPAMSubnet = "" },
			expectError: true,
			errorMsg:    "IPAM subnet cannot be empty",
		},
		{
			name:        "empty gateway",
			modify:      func(c *Config) { c.Plugin.Gateway = "" },
			expectError: true,
			errorMsg:    "gateway cannot be empty",
		},
		{
			name:        "empty SAM host",
			modify:      func(c *Config) { c.SAM.Host = "" },
			expectError: true,
			errorMsg:    "SAM host cannot be empty",
		},
		{
			name:        "invalid SAM port - zero",
			modify:      func(c *Config) { c.SAM.Port = 0 },
			expectError: true,
			errorMsg:    "SAM port must be between 1 and 65535, got 0",
		},
		{
			name:        "invalid SAM port - too high",
			modify:      func(c *Config) { c.SAM.Port = 65536 },
			expectError: true,
			errorMsg:    "SAM port must be between 1 and 65535, got 65536",
		},
		{
			name:        "invalid SAM timeout",
			modify:      func(c *Config) { c.SAM.Timeout = 0 },
			expectError: true,
			errorMsg:    "SAM timeout must be positive, got 0s",
		},
		{
			name:        "invalid inbound tunnels",
			modify:      func(c *Config) { c.TunnelDefaults.InboundTunnels = 0 },
			expectError: true,
			errorMsg:    "inbound tunnels must be positive, got 0",
		},
		{
			name:        "invalid outbound tunnels",
			modify:      func(c *Config) { c.TunnelDefaults.OutboundTunnels = -1 },
			expectError: true,
			errorMsg:    "outbound tunnels must be positive, got -1",
		},
		{
			name:        "invalid inbound length",
			modify:      func(c *Config) { c.TunnelDefaults.InboundLength = 0 },
			expectError: true,
			errorMsg:    "inbound length must be positive, got 0",
		},
		{
			name:        "invalid outbound length",
			modify:      func(c *Config) { c.TunnelDefaults.OutboundLength = -2 },
			expectError: true,
			errorMsg:    "outbound length must be positive, got -2",
		},
		{
			name:        "invalid close idle time",
			modify:      func(c *Config) { c.TunnelDefaults.CloseIdleTime = 0 },
			expectError: true,
			errorMsg:    "close idle time must be positive, got 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)

			err := config.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && err.Error() != tt.errorMsg {
				t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestGetters(t *testing.T) {
	config := DefaultConfig()

	// Test GetSAMConfig
	samConfig := config.GetSAMConfig()
	if samConfig != &config.SAM {
		t.Error("GetSAMConfig should return pointer to SAM config")
	}

	// Test GetTunnelDefaults
	tunnelDefaults := config.GetTunnelDefaults()
	if tunnelDefaults != &config.TunnelDefaults {
		t.Error("GetTunnelDefaults should return pointer to tunnel defaults")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue bool
		expected     bool
	}{
		// True values
		{"true string", "true", false, true},
		{"1 string", "1", false, true},
		{"yes string", "yes", false, true},
		{"on string", "on", false, true},
		{"enable string", "enable", false, true},
		{"enabled string", "enabled", false, true},

		// False values
		{"false string", "false", true, false},
		{"0 string", "0", true, false},
		{"no string", "no", true, false},
		{"off string", "off", true, false},
		{"disable string", "disable", true, false},
		{"disabled string", "disabled", true, false},

		// Default fallback
		{"invalid with false default", "invalid", false, false},
		{"invalid with true default", "invalid", true, true},
		{"empty with false default", "", false, false},
		{"empty with true default", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBool(tt.input, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("parseBool(%q, %v) = %v, expected %v", tt.input, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

// Test edge cases and error conditions
func TestLoadFromEnvironmentEdgeCases(t *testing.T) {
	// Save original environment
	originalPort := os.Getenv("I2P_SAM_PORT")
	defer func() {
		if originalPort != "" {
			os.Setenv("I2P_SAM_PORT", originalPort)
		} else {
			os.Unsetenv("I2P_SAM_PORT")
		}
	}()

	tests := []struct {
		name     string
		portVal  string
		expected int
	}{
		{"valid port", "8080", 8080},
		{"port 1", "1", 1},
		{"port 65535", "65535", 65535},
		{"port 0 ignored", "0", 7656},         // Should keep default
		{"port 65536 ignored", "65536", 7656}, // Should keep default
		{"negative port ignored", "-1", 7656}, // Should keep default
		{"invalid port ignored", "abc", 7656}, // Should keep default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("I2P_SAM_PORT", tt.portVal)

			config := DefaultConfig()
			err := config.LoadFromEnvironment()
			if err != nil {
				t.Fatalf("LoadFromEnvironment failed: %v", err)
			}

			if config.SAM.Port != tt.expected {
				t.Errorf("Expected port %d, got %d", tt.expected, config.SAM.Port)
			}
		})
	}
}

func TestValidateComplexScenarios(t *testing.T) {
	t.Run("multiple validation errors", func(t *testing.T) {
		config := DefaultConfig()
		config.Plugin.SocketPath = ""
		config.SAM.Host = ""
		config.SAM.Port = 0

		err := config.Validate()
		if err == nil {
			t.Error("Expected validation error but got none")
		}

		// Should get first validation error
		if err.Error() != "plugin socket path cannot be empty" {
			t.Errorf("Expected first validation error, got: %s", err.Error())
		}
	})

	t.Run("boundary values", func(t *testing.T) {
		config := DefaultConfig()

		// Test boundary port values
		config.SAM.Port = 1
		if err := config.Validate(); err != nil {
			t.Errorf("Port 1 should be valid: %v", err)
		}

		config.SAM.Port = 65535
		if err := config.Validate(); err != nil {
			t.Errorf("Port 65535 should be valid: %v", err)
		}

		// Test minimum positive values
		config.TunnelDefaults.InboundTunnels = 1
		config.TunnelDefaults.OutboundTunnels = 1
		config.TunnelDefaults.InboundLength = 1
		config.TunnelDefaults.OutboundLength = 1
		config.TunnelDefaults.CloseIdleTime = 1
		config.SAM.Timeout = 1 * time.Nanosecond

		if err := config.Validate(); err != nil {
			t.Errorf("Minimum positive values should be valid: %v", err)
		}
	})
}
