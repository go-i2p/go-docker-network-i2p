package service

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

func TestNewServiceExposureManager(t *testing.T) {
	// Create a mock tunnel manager
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)

	tests := []struct {
		name          string
		tunnelMgr     *i2p.TunnelManager
		shouldError   bool
		expectedError string
	}{
		{
			name:        "valid tunnel manager",
			tunnelMgr:   tunnelMgr,
			shouldError: false,
		},
		{
			name:          "nil tunnel manager",
			tunnelMgr:     nil,
			shouldError:   true,
			expectedError: "tunnel manager cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewServiceExposureManager(tt.tunnelMgr)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error, but got none")
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if manager == nil {
					t.Error("Expected manager to be created, but got nil")
				}
			}
		})
	}
}

func TestDetectExposedPorts(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	tests := []struct {
		name          string
		containerID   string
		options       map[string]interface{}
		expectedPorts int
		shouldError   bool
	}{
		{
			name:        "empty container ID",
			containerID: "",
			options:     map[string]interface{}{},
			shouldError: true,
		},
		{
			name:          "no exposed ports",
			containerID:   "test-container",
			options:       map[string]interface{}{},
			expectedPorts: 0,
			shouldError:   false,
		},
		{
			name:        "Docker ExposedPorts format",
			containerID: "test-container",
			options: map[string]interface{}{
				"ExposedPorts": map[string]interface{}{
					"80/tcp":  map[string]interface{}{},
					"443/tcp": map[string]interface{}{},
					"53/udp":  map[string]interface{}{},
				},
			},
			expectedPorts: 3,
			shouldError:   false,
		},
		{
			name:        "Environment variables",
			containerID: "test-container",
			options: map[string]interface{}{
				"Env": []interface{}{
					"PORT=8080",
					"HTTP_PORT=80",
					"HTTPS_PORT=443",
					"OTHER_VAR=value",
				},
			},
			expectedPorts: 3,
			shouldError:   false,
		},
		{
			name:        "Port mapping format",
			containerID: "test-container",
			options: map[string]interface{}{
				"com.docker.network.portmap": []interface{}{
					map[string]interface{}{
						"ContainerPort": 8080,
						"Protocol":      "tcp",
					},
					map[string]interface{}{
						"ContainerPort": "9090",
						"Protocol":      "udp",
					},
				},
			},
			expectedPorts: 2,
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports, err := manager.DetectExposedPorts(tt.containerID, tt.options)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if len(ports) != tt.expectedPorts {
					t.Errorf("Expected %d ports, got %d", tt.expectedPorts, len(ports))
				}
			}
		})
	}
}

func TestParsePortSpec(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	tests := []struct {
		name       string
		portSpec   string
		expected   *ExposedPort
		shouldFail bool
	}{
		{
			name:     "valid TCP port",
			portSpec: "80/tcp",
			expected: &ExposedPort{
				ContainerPort: 80,
				Protocol:      "tcp",
				ServiceName:   "service-80",
			},
			shouldFail: false,
		},
		{
			name:     "valid UDP port",
			portSpec: "53/udp",
			expected: &ExposedPort{
				ContainerPort: 53,
				Protocol:      "udp",
				ServiceName:   "service-53",
			},
			shouldFail: false,
		},
		{
			name:       "invalid format",
			portSpec:   "80",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid port number",
			portSpec:   "99999/tcp",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "zero port",
			portSpec:   "0/tcp",
			expected:   nil,
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.parsePortSpec(tt.portSpec)

			if tt.shouldFail {
				if result != nil {
					t.Error("Expected nil result for invalid port spec")
				}
			} else {
				if result == nil {
					t.Error("Expected valid port, got nil")
				} else {
					if result.ContainerPort != tt.expected.ContainerPort {
						t.Errorf("Expected port %d, got %d", tt.expected.ContainerPort, result.ContainerPort)
					}
					if result.Protocol != tt.expected.Protocol {
						t.Errorf("Expected protocol %s, got %s", tt.expected.Protocol, result.Protocol)
					}
					if result.ServiceName != tt.expected.ServiceName {
						t.Errorf("Expected service name %s, got %s", tt.expected.ServiceName, result.ServiceName)
					}
				}
			}
		})
	}
}

func TestParseEnvironmentPort(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	tests := []struct {
		name       string
		envVar     string
		expected   *ExposedPort
		shouldFail bool
	}{
		{
			name:   "PORT variable",
			envVar: "PORT=8080",
			expected: &ExposedPort{
				ContainerPort: 8080,
				Protocol:      "tcp",
				ServiceName:   "service-8080",
			},
			shouldFail: false,
		},
		{
			name:   "HTTP_PORT variable",
			envVar: "HTTP_PORT=80",
			expected: &ExposedPort{
				ContainerPort: 80,
				Protocol:      "tcp",
				ServiceName:   "http-80",
			},
			shouldFail: false,
		},
		{
			name:   "APP_PORT variable",
			envVar: "APP_PORT=3000",
			expected: &ExposedPort{
				ContainerPort: 3000,
				Protocol:      "tcp",
				ServiceName:   "app-3000",
			},
			shouldFail: false,
		},
		{
			name:       "invalid format",
			envVar:     "INVALID=abc",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "non-port variable",
			envVar:     "PATH=/usr/bin",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid port number",
			envVar:     "PORT=99999",
			expected:   nil,
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.parseEnvironmentPort(tt.envVar)

			if tt.shouldFail {
				if result != nil {
					t.Error("Expected nil result for invalid environment variable")
				}
			} else {
				if result == nil {
					t.Error("Expected valid port, got nil")
				} else {
					if result.ContainerPort != tt.expected.ContainerPort {
						t.Errorf("Expected port %d, got %d", tt.expected.ContainerPort, result.ContainerPort)
					}
					if result.Protocol != tt.expected.Protocol {
						t.Errorf("Expected protocol %s, got %s", tt.expected.Protocol, result.Protocol)
					}
					if result.ServiceName != tt.expected.ServiceName {
						t.Errorf("Expected service name %s, got %s", tt.expected.ServiceName, result.ServiceName)
					}
				}
			}
		})
	}
}

func TestGenerateB32Address(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	tests := []struct {
		name        string
		destination string
		shouldError bool
	}{
		{
			name:        "valid destination",
			destination: "test-destination-string",
			shouldError: false,
		},
		{
			name:        "empty destination",
			destination: "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := manager.generateB32Address(tt.destination)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error for empty destination")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if address == "" {
					t.Error("Expected non-empty address")
				}
				if !strings.HasSuffix(address, ".b32.i2p") {
					t.Errorf("Expected address to end with .b32.i2p, got: %s", address)
				}

				// Test that same destination generates same address
				address2, err := manager.generateB32Address(tt.destination)
				if err != nil {
					t.Errorf("Failed to generate address again: %v", err)
				}
				if address != address2 {
					t.Errorf("Expected consistent address generation, got %s and %s", address, address2)
				}
			}
		})
	}
}

func TestExposeServices(t *testing.T) {
	// This test requires a real I2P connection for tunnel creation

	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	// Connect to the SAM bridge
	ctx := context.Background()
	if err := samClient.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect SAM client: %v", err)
	}
	defer samClient.Disconnect()

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	containerID := "test-container"
	networkID := "test-network"
	containerIP := net.ParseIP("172.20.0.2")

	// Test multiple server tunnels with go-sam-go NewStreamSubSessionWithPort support
	ports := []ExposedPort{
		{
			ContainerPort: 80,
			Protocol:      "tcp",
			ServiceName:   "web",
		},
		{
			ContainerPort: 443,
			Protocol:      "tcp",
			ServiceName:   "https",
		},
	}

	exposures, err := manager.ExposeServices(containerID, networkID, containerIP, ports)
	if err != nil {
		t.Fatalf("Failed to expose services: %v", err)
	}

	if len(exposures) != len(ports) {
		t.Errorf("Expected %d exposures, got %d", len(ports), len(exposures))
	}

	// Verify exposures are tracked
	trackedExposures := manager.GetServiceExposures(containerID)
	if len(trackedExposures) != len(exposures) {
		t.Errorf("Expected %d tracked exposures, got %d", len(exposures), len(trackedExposures))
	}

	// Clean up
	err = manager.CleanupServices(containerID)
	if err != nil {
		t.Errorf("Failed to cleanup services: %v", err)
	}

	// Verify cleanup
	afterCleanup := manager.GetServiceExposures(containerID)
	if len(afterCleanup) != 0 {
		t.Errorf("Expected no exposures after cleanup, got %d", len(afterCleanup))
	}
}

func TestGetServiceExposures(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	// Test with non-existent container
	exposures := manager.GetServiceExposures("non-existent")
	if exposures != nil {
		t.Error("Expected nil for non-existent container")
	}

	// Test with existing container (would need real exposures for full test)
	// This is tested in TestExposeServices which is skipped due to I2P dependency
}

func TestCleanupServices(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	tests := []struct {
		name          string
		containerID   string
		shouldError   bool
		expectedError string
	}{
		{
			name:          "empty container ID",
			containerID:   "",
			shouldError:   true,
			expectedError: "container ID cannot be empty",
		},
		{
			name:        "non-existent container",
			containerID: "non-existent",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.CleanupServices(tt.containerID)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error, but got none")
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	// Test shutdown with no active services
	err = manager.Shutdown()
	if err != nil {
		t.Errorf("Expected no error during shutdown, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkGenerateB32Address(b *testing.B) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		b.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		b.Fatalf("Failed to create service exposure manager: %v", err)
	}

	destination := "test-destination-for-benchmarking"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.generateB32Address(destination)
		if err != nil {
			b.Fatalf("Failed to generate address: %v", err)
		}
	}
}

func BenchmarkDetectExposedPorts(b *testing.B) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		b.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		b.Fatalf("Failed to create service exposure manager: %v", err)
	}

	options := map[string]interface{}{
		"ExposedPorts": map[string]interface{}{
			"80/tcp":   map[string]interface{}{},
			"443/tcp":  map[string]interface{}{},
			"8080/tcp": map[string]interface{}{},
		},
		"Env": []interface{}{
			"PORT=3000",
			"HTTP_PORT=80",
			"OTHER_VAR=value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.DetectExposedPorts("test-container", options)
		if err != nil {
			b.Fatalf("Failed to detect ports: %v", err)
		}
	}
}
