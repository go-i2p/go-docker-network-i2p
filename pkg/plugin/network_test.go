// Package plugin provides comprehensive tests for I2P network lifecycle management.
//
// This file tests the NetworkManager and its integration with I2P tunnels,
// ensuring proper network creation, deletion, and resource management.
package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

// TestNetworkManager_CreateNetwork tests network creation functionality.
func TestNetworkManager_CreateNetwork(t *testing.T) {
	// Create a mock tunnel manager for testing
	tunnelMgr := createMockTunnelManager(t)

	// Create network manager
	nm, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	tests := []struct {
		name        string
		networkID   string
		options     map[string]interface{}
		ipamData    []IPAMData
		expectError bool
		errorMsg    string
	}{
		{
			name:      "basic network creation",
			networkID: "test-network-1",
			options: map[string]interface{}{
				"com.docker.network.bridge.name": "i2p-br0",
			},
			ipamData: []IPAMData{
				{
					Pool:    "172.20.0.0/16",
					Gateway: "172.20.0.1",
				},
			},
			expectError: false,
		},
		{
			name:        "empty network ID",
			networkID:   "",
			options:     map[string]interface{}{},
			ipamData:    []IPAMData{},
			expectError: true,
			errorMsg:    "network ID cannot be empty",
		},
		{
			name:        "duplicate network ID",
			networkID:   "test-network-1", // Same as first test
			options:     map[string]interface{}{},
			ipamData:    []IPAMData{},
			expectError: true,
			errorMsg:    "network test-network-1 already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nm.CreateNetwork(tt.networkID, tt.options, tt.ipamData)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify network was created
			network := nm.GetNetwork(tt.networkID)
			if network == nil {
				t.Errorf("Network %s was not found after creation", tt.networkID)
				return
			}

			// Verify network properties
			if network.ID != tt.networkID {
				t.Errorf("Expected network ID %s, got %s", tt.networkID, network.ID)
			}

			if network.TunnelManager == nil {
				t.Error("Network tunnel manager is nil")
			}

			if network.IPAllocator == nil {
				t.Error("Network IP allocator is nil")
			}

			// Verify subnet configuration if IPAM data was provided
			if len(tt.ipamData) > 0 {
				expectedCIDR := tt.ipamData[0].Pool
				if network.Subnet.String() != expectedCIDR {
					t.Errorf("Expected subnet %s, got %s", expectedCIDR, network.Subnet.String())
				}

				expectedGateway := tt.ipamData[0].Gateway
				if network.Gateway.String() != expectedGateway {
					t.Errorf("Expected gateway %s, got %s", expectedGateway, network.Gateway.String())
				}
			}
		})
	}
}

// TestNetworkManager_DeleteNetwork tests network deletion functionality.
func TestNetworkManager_DeleteNetwork(t *testing.T) {
	// Create a mock tunnel manager for testing
	tunnelMgr := createMockTunnelManager(t)

	// Create network manager
	nm, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	// Create test networks
	testNetworks := []string{"delete-test-1", "delete-test-2"}
	for _, networkID := range testNetworks {
		options := map[string]interface{}{}
		ipamData := []IPAMData{
			{
				Pool:    "172.30.0.0/16",
				Gateway: "172.30.0.1",
			},
		}
		if err := nm.CreateNetwork(networkID, options, ipamData); err != nil {
			t.Fatalf("Failed to create test network %s: %v", networkID, err)
		}
	}

	tests := []struct {
		name        string
		networkID   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful deletion",
			networkID:   "delete-test-1",
			expectError: false,
		},
		{
			name:        "delete non-existent network",
			networkID:   "non-existent",
			expectError: true,
			errorMsg:    "network non-existent not found",
		},
		{
			name:        "empty network ID",
			networkID:   "",
			expectError: true,
			errorMsg:    "network ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nm.DeleteNetwork(tt.networkID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify network was deleted
			network := nm.GetNetwork(tt.networkID)
			if network != nil {
				t.Errorf("Network %s still exists after deletion", tt.networkID)
			}
		})
	}
}

// TestNetworkManager_GetNetwork tests network retrieval functionality.
func TestNetworkManager_GetNetwork(t *testing.T) {
	// Create a mock tunnel manager for testing
	tunnelMgr := createMockTunnelManager(t)

	// Create network manager
	nm, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	// Create a test network
	networkID := "get-test-network"
	options := map[string]interface{}{
		"test.option": "value",
	}
	ipamData := []IPAMData{
		{
			Pool:    "10.100.0.0/16",
			Gateway: "10.100.0.1",
		},
	}

	if err := nm.CreateNetwork(networkID, options, ipamData); err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}

	tests := []struct {
		name        string
		networkID   string
		expectFound bool
	}{
		{
			name:        "get existing network",
			networkID:   networkID,
			expectFound: true,
		},
		{
			name:        "get non-existent network",
			networkID:   "non-existent",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := nm.GetNetwork(tt.networkID)

			if tt.expectFound {
				if network == nil {
					t.Errorf("Expected to find network %s, but it was not found", tt.networkID)
					return
				}

				if network.ID != tt.networkID {
					t.Errorf("Expected network ID %s, got %s", tt.networkID, network.ID)
				}
			} else {
				if network != nil {
					t.Errorf("Expected not to find network %s, but it was found", tt.networkID)
				}
			}
		})
	}
}

// TestNetworkManager_ListNetworks tests network listing functionality.
func TestNetworkManager_ListNetworks(t *testing.T) {
	// Create a mock tunnel manager for testing
	tunnelMgr := createMockTunnelManager(t)

	// Create network manager
	nm, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	// Test with empty manager
	networks := nm.ListNetworks()
	if len(networks) != 0 {
		t.Errorf("Expected 0 networks in empty manager, got %d", len(networks))
	}

	// Create test networks
	testNetworkIDs := []string{"list-test-1", "list-test-2", "list-test-3"}
	for _, networkID := range testNetworkIDs {
		options := map[string]interface{}{}
		ipamData := []IPAMData{
			{
				Pool:    "172.40.0.0/16",
				Gateway: "172.40.0.1",
			},
		}
		if err := nm.CreateNetwork(networkID, options, ipamData); err != nil {
			t.Fatalf("Failed to create test network %s: %v", networkID, err)
		}
	}

	// Test listing with networks
	networks = nm.ListNetworks()
	if len(networks) != len(testNetworkIDs) {
		t.Errorf("Expected %d networks, got %d", len(testNetworkIDs), len(networks))
	}

	// Verify all test networks are in the list
	networkMap := make(map[string]bool)
	for _, networkID := range networks {
		networkMap[networkID] = true
	}

	for _, expectedID := range testNetworkIDs {
		if !networkMap[expectedID] {
			t.Errorf("Expected network %s not found in list", expectedID)
		}
	}
}

// TestI2PNetwork_BasicOperations tests basic network operations.
func TestI2PNetwork_BasicOperations(t *testing.T) {
	// Create mock tunnel manager and network manager
	tunnelMgr := createMockTunnelManager(t)
	nm, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	// Create test network
	networkID := "basic-ops-test"
	options := map[string]interface{}{}
	ipamData := []IPAMData{
		{
			Pool:    "192.168.201.0/24",
			Gateway: "192.168.201.1",
		},
	}

	if err := nm.CreateNetwork(networkID, options, ipamData); err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}

	network := nm.GetNetwork(networkID)
	if network == nil {
		t.Fatal("Test network not found after creation")
	}

	// Test basic IP allocation
	ip1, err := network.IPAllocator.AllocateIP()
	if err != nil {
		t.Errorf("Failed to allocate IP: %v", err)
	}

	if ip1 == nil {
		t.Error("Allocated IP is nil")
	}

	// Verify IP is in the correct subnet
	if !network.Subnet.Contains(ip1) {
		t.Errorf("Allocated IP %s is not in network subnet %s", ip1, network.Subnet)
	}

	// Test IP release
	network.IPAllocator.ReleaseIP(ip1)

	// Test network cleanup
	if err := nm.DeleteNetwork(networkID); err != nil {
		t.Errorf("Failed to delete network: %v", err)
	}

	// Verify network was deleted
	deletedNetwork := nm.GetNetwork(networkID)
	if deletedNetwork != nil {
		t.Error("Network still exists after deletion")
	}
}

// TestNewNetworkManager tests network manager creation.
func TestNewNetworkManager(t *testing.T) {
	tests := []struct {
		name        string
		tunnelMgr   *i2p.TunnelManager
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid tunnel manager",
			tunnelMgr:   createMockTunnelManager(t),
			expectError: false,
		},
		{
			name:        "nil tunnel manager",
			tunnelMgr:   nil,
			expectError: true,
			errorMsg:    "tunnel manager cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nm, err := NewNetworkManager(tt.tunnelMgr)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error '%s', but got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if nm != nil {
					t.Error("Network manager should be nil when error occurs")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if nm == nil {
				t.Error("Network manager is nil")
				return
			}

			// Verify initial state
			networks := nm.ListNetworks()
			if len(networks) != 0 {
				t.Errorf("Expected 0 networks in new manager, got %d", len(networks))
			}
		})
	}
}

// createMockTunnelManager creates a tunnel manager for testing with real SAM connection.
//
// This function creates a tunnel manager that connects to the actual I2P SAM bridge
// on localhost:7656, suitable for integration testing.
func createMockTunnelManager(t *testing.T) *i2p.TunnelManager {
	// Create a SAM client that connects to the real SAM bridge
	samClient, err := i2p.NewSAMClient(&i2p.SAMConfig{
		Host:    "127.0.0.1",
		Port:    7656,
		Timeout: 30 * time.Second, // 30 seconds timeout
	})
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	// Connect to the SAM bridge for real testing
	ctx := context.Background()
	if err := samClient.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect to I2P SAM bridge at localhost:7656: %v", err)
	}

	return i2p.NewTunnelManager(samClient)
}

// TestParseNetworkExposureConfig tests network exposure configuration parsing.
func TestParseNetworkExposureConfig(t *testing.T) {
	tests := []struct {
		name                    string
		options                 map[string]interface{}
		expectedDefaultType     string
		expectedAllowIPExposure bool
	}{
		{
			name:                    "nil options defaults to I2P with IP allowed",
			options:                 nil,
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name:                    "empty options defaults to I2P with IP allowed",
			options:                 map[string]interface{}{},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name: "explicit I2P default",
			options: map[string]interface{}{
				"i2p.exposure.default": "i2p",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name: "explicit IP default",
			options: map[string]interface{}{
				"i2p.exposure.default": "ip",
			},
			expectedDefaultType:     "ip",
			expectedAllowIPExposure: true,
		},
		{
			name: "disallow IP exposure",
			options: map[string]interface{}{
				"i2p.exposure.allow_ip": "false",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: false,
		},
		{
			name: "allow IP exposure with 'true'",
			options: map[string]interface{}{
				"i2p.exposure.allow_ip": "true",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name: "allow IP exposure with '1'",
			options: map[string]interface{}{
				"i2p.exposure.allow_ip": "1",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name: "allow IP exposure with 'yes'",
			options: map[string]interface{}{
				"i2p.exposure.allow_ip": "yes",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name: "combined settings - IP default with IP allowed",
			options: map[string]interface{}{
				"i2p.exposure.default":  "ip",
				"i2p.exposure.allow_ip": "true",
			},
			expectedDefaultType:     "ip",
			expectedAllowIPExposure: true,
		},
		{
			name: "combined settings - IP default with IP disallowed",
			options: map[string]interface{}{
				"i2p.exposure.default":  "ip",
				"i2p.exposure.allow_ip": "false",
			},
			expectedDefaultType:     "ip",
			expectedAllowIPExposure: false,
		},
		{
			name: "invalid exposure type defaults to I2P",
			options: map[string]interface{}{
				"i2p.exposure.default": "invalid",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name: "non-string option types handled gracefully",
			options: map[string]interface{}{
				"i2p.exposure.default":  123,
				"i2p.exposure.allow_ip": 456,
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := parseNetworkExposureConfig(tt.options)

			if string(config.DefaultExposureType) != tt.expectedDefaultType {
				t.Errorf("Expected default exposure type %s, got %s",
					tt.expectedDefaultType, config.DefaultExposureType)
			}

			if config.AllowIPExposure != tt.expectedAllowIPExposure {
				t.Errorf("Expected AllowIPExposure %v, got %v",
					tt.expectedAllowIPExposure, config.AllowIPExposure)
			}
		})
	}
}

// TestNetworkCreationWithExposureConfig tests that networks are created with proper exposure configuration.
func TestNetworkCreationWithExposureConfig(t *testing.T) {
	tunnelMgr := createMockTunnelManager(t)
	nm, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	tests := []struct {
		name                    string
		networkID               string
		options                 map[string]interface{}
		expectedDefaultType     string
		expectedAllowIPExposure bool
	}{
		{
			name:      "network with default I2P exposure",
			networkID: "test-network-config-1",
			options: map[string]interface{}{
				"com.docker.network.bridge.name": "i2p-br0",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: true,
		},
		{
			name:      "network with explicit IP default",
			networkID: "test-network-config-2",
			options: map[string]interface{}{
				"com.docker.network.bridge.name": "i2p-br1",
				"i2p.exposure.default":           "ip",
			},
			expectedDefaultType:     "ip",
			expectedAllowIPExposure: true,
		},
		{
			name:      "network with IP exposure disabled",
			networkID: "test-network-config-3",
			options: map[string]interface{}{
				"com.docker.network.bridge.name": "i2p-br2",
				"i2p.exposure.allow_ip":          "false",
			},
			expectedDefaultType:     "i2p",
			expectedAllowIPExposure: false,
		},
		{
			name:      "network with IP default but IP disallowed",
			networkID: "test-network-config-4",
			options: map[string]interface{}{
				"com.docker.network.bridge.name": "i2p-br3",
				"i2p.exposure.default":           "ip",
				"i2p.exposure.allow_ip":          "false",
			},
			expectedDefaultType:     "ip",
			expectedAllowIPExposure: false,
		},
	}

	// Create all networks first (proxy manager starts with first network)
	ipamData := []IPAMData{
		{
			Pool:    "172.20.0.0/16",
			Gateway: "172.20.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nm.CreateNetwork(tt.networkID, tt.options, ipamData)
			if err != nil {
				// Skip test if iptables not available (expected in test environments)
				if err.Error() == "failed to start proxy manager: iptables not available: iptables command not found: exec: \"iptables\": executable file not found in $PATH" {
					t.Skip("Skipping test: iptables not available in test environment")
				}
				t.Fatalf("Failed to create network: %v", err)
			}

			network := nm.GetNetwork(tt.networkID)
			if network == nil {
				t.Fatal("Network not found after creation")
			}

			if string(network.ExposureConfig.DefaultExposureType) != tt.expectedDefaultType {
				t.Errorf("Expected network default exposure type %s, got %s",
					tt.expectedDefaultType, network.ExposureConfig.DefaultExposureType)
			}

			if network.ExposureConfig.AllowIPExposure != tt.expectedAllowIPExposure {
				t.Errorf("Expected network AllowIPExposure %v, got %v",
					tt.expectedAllowIPExposure, network.ExposureConfig.AllowIPExposure)
			}
		})
	}

	// Clean up all networks at the end
	for _, tt := range tests {
		if nm.GetNetwork(tt.networkID) != nil {
			if err := nm.DeleteNetwork(tt.networkID); err != nil {
				t.Logf("Warning: Failed to delete network %s: %v", tt.networkID, err)
			}
		}
	}
}
