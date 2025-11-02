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

// TestParseExposureLabel tests the label parsing functionality for port exposure configuration.
func TestParseExposureLabel(t *testing.T) {
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
		labelKey   string
		labelValue interface{}
		expected   *ExposedPort
		shouldFail bool
	}{
		{
			name:       "i2p exposure",
			labelKey:   "i2p.expose.80",
			labelValue: "i2p",
			expected: &ExposedPort{
				ContainerPort: 80,
				Protocol:      "tcp",
				ServiceName:   "service-80",
				ExposureType:  ExposureTypeI2P,
				TargetIP:      "",
			},
			shouldFail: false,
		},
		{
			name:       "ip exposure with explicit IP",
			labelKey:   "i2p.expose.443",
			labelValue: "ip:127.0.0.1",
			expected: &ExposedPort{
				ContainerPort: 443,
				Protocol:      "tcp",
				ServiceName:   "service-443",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "127.0.0.1",
			},
			shouldFail: false,
		},
		{
			name:       "ip exposure with explicit external IP",
			labelKey:   "i2p.expose.8080",
			labelValue: "ip:192.168.1.100",
			expected: &ExposedPort{
				ContainerPort: 8080,
				Protocol:      "tcp",
				ServiceName:   "service-8080",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "192.168.1.100",
			},
			shouldFail: false,
		},
		{
			name:       "ip exposure defaults to localhost",
			labelKey:   "i2p.expose.9090",
			labelValue: "ip:",
			expected: &ExposedPort{
				ContainerPort: 9090,
				Protocol:      "tcp",
				ServiceName:   "service-9090",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "127.0.0.1",
			},
			shouldFail: false,
		},
		{
			name:       "invalid port number (too large)",
			labelKey:   "i2p.expose.99999",
			labelValue: "i2p",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid port number (zero)",
			labelKey:   "i2p.expose.0",
			labelValue: "i2p",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid port number (negative)",
			labelKey:   "i2p.expose.-1",
			labelValue: "i2p",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid port number (non-numeric)",
			labelKey:   "i2p.expose.abc",
			labelValue: "i2p",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid exposure type",
			labelKey:   "i2p.expose.80",
			labelValue: "invalid",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid value type",
			labelKey:   "i2p.expose.80",
			labelValue: 123,
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "invalid IP address",
			labelKey:   "i2p.expose.80",
			labelValue: "ip:invalid-ip",
			expected:   nil,
			shouldFail: true,
		},
		{
			name:       "IPv6 address support",
			labelKey:   "i2p.expose.80",
			labelValue: "ip:::1",
			expected: &ExposedPort{
				ContainerPort: 80,
				Protocol:      "tcp",
				ServiceName:   "service-80",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "::1",
			},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.parseExposureLabel(tt.labelKey, tt.labelValue)

			if tt.shouldFail {
				if result != nil {
					t.Errorf("Expected nil result for invalid label, got: %+v", result)
				}
			} else {
				if result == nil {
					t.Fatal("Expected valid port, got nil")
				}
				if result.ContainerPort != tt.expected.ContainerPort {
					t.Errorf("Expected port %d, got %d", tt.expected.ContainerPort, result.ContainerPort)
				}
				if result.Protocol != tt.expected.Protocol {
					t.Errorf("Expected protocol %s, got %s", tt.expected.Protocol, result.Protocol)
				}
				if result.ServiceName != tt.expected.ServiceName {
					t.Errorf("Expected service name %s, got %s", tt.expected.ServiceName, result.ServiceName)
				}
				if result.ExposureType != tt.expected.ExposureType {
					t.Errorf("Expected exposure type %s, got %s", tt.expected.ExposureType, result.ExposureType)
				}
				if result.TargetIP != tt.expected.TargetIP {
					t.Errorf("Expected target IP %s, got %s", tt.expected.TargetIP, result.TargetIP)
				}
			}
		})
	}
}

// TestExtractPortsFromLabels tests extraction of port configurations from Docker labels.
func TestExtractPortsFromLabels(t *testing.T) {
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
		options       map[string]interface{}
		expectedCount int
		validate      func(t *testing.T, ports []ExposedPort)
	}{
		{
			name:          "no labels",
			options:       map[string]interface{}{},
			expectedCount: 0,
		},
		{
			name: "single i2p exposure",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80": "i2p",
				},
			},
			expectedCount: 1,
			validate: func(t *testing.T, ports []ExposedPort) {
				if ports[0].ExposureType != ExposureTypeI2P {
					t.Errorf("Expected I2P exposure type, got %s", ports[0].ExposureType)
				}
				if ports[0].ContainerPort != 80 {
					t.Errorf("Expected port 80, got %d", ports[0].ContainerPort)
				}
			},
		},
		{
			name: "single ip exposure",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.443": "ip:127.0.0.1",
				},
			},
			expectedCount: 1,
			validate: func(t *testing.T, ports []ExposedPort) {
				if ports[0].ExposureType != ExposureTypeIP {
					t.Errorf("Expected IP exposure type, got %s", ports[0].ExposureType)
				}
				if ports[0].TargetIP != "127.0.0.1" {
					t.Errorf("Expected target IP 127.0.0.1, got %s", ports[0].TargetIP)
				}
			},
		},
		{
			name: "multiple mixed exposures",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80":   "i2p",
					"i2p.expose.443":  "ip:127.0.0.1",
					"i2p.expose.8080": "ip:0.0.0.0",
					"i2p.expose.9090": "i2p",
				},
			},
			expectedCount: 4,
			validate: func(t *testing.T, ports []ExposedPort) {
				// Count exposure types
				i2pCount := 0
				ipCount := 0
				for _, port := range ports {
					if port.ExposureType == ExposureTypeI2P {
						i2pCount++
					} else if port.ExposureType == ExposureTypeIP {
						ipCount++
					}
				}
				if i2pCount != 2 {
					t.Errorf("Expected 2 I2P exposures, got %d", i2pCount)
				}
				if ipCount != 2 {
					t.Errorf("Expected 2 IP exposures, got %d", ipCount)
				}
			},
		},
		{
			name: "labels with non-exposure labels",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80":       "i2p",
					"com.example.version": "1.0",
					"maintainer":          "someone@example.com",
				},
			},
			expectedCount: 1,
			validate: func(t *testing.T, ports []ExposedPort) {
				if ports[0].ContainerPort != 80 {
					t.Errorf("Expected port 80, got %d", ports[0].ContainerPort)
				}
			},
		},
		{
			name: "invalid labels are skipped",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80":    "i2p",
					"i2p.expose.99999": "i2p", // Invalid port
					"i2p.expose.abc":   "i2p", // Invalid port
					"i2p.expose.443":   "ip:127.0.0.1",
				},
			},
			expectedCount: 2, // Only valid labels
			validate: func(t *testing.T, ports []ExposedPort) {
				// Verify only valid ports are returned
				for _, port := range ports {
					if port.ContainerPort != 80 && port.ContainerPort != 443 {
						t.Errorf("Unexpected port %d in results", port.ContainerPort)
					}
				}
			},
		},
		{
			name: "labels field is not a map",
			options: map[string]interface{}{
				"Labels": "invalid",
			},
			expectedCount: 0,
		},
		{
			name: "empty labels map",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := manager.extractPortsFromLabels(tt.options)

			if len(ports) != tt.expectedCount {
				t.Errorf("Expected %d ports, got %d", tt.expectedCount, len(ports))
			}

			if tt.validate != nil && len(ports) > 0 {
				tt.validate(t, ports)
			}
		})
	}
}

// TestExposureTypeConstants verifies the exposure type constants.
func TestExposureTypeConstants(t *testing.T) {
	if ExposureTypeI2P != "i2p" {
		t.Errorf("Expected ExposureTypeI2P to be 'i2p', got %s", ExposureTypeI2P)
	}
	if ExposureTypeIP != "ip" {
		t.Errorf("Expected ExposureTypeIP to be 'ip', got %s", ExposureTypeIP)
	}
}

// BenchmarkParseExposureLabel benchmarks label parsing performance.
func BenchmarkParseExposureLabel(b *testing.B) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		b.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		b.Fatalf("Failed to create service exposure manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.parseExposureLabel("i2p.expose.80", "i2p")
	}
}

// BenchmarkExtractPortsFromLabels benchmarks label extraction performance.
func BenchmarkExtractPortsFromLabels(b *testing.B) {
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
		"Labels": map[string]interface{}{
			"i2p.expose.80":   "i2p",
			"i2p.expose.443":  "ip:127.0.0.1",
			"i2p.expose.8080": "i2p",
			"other.label":     "value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.extractPortsFromLabels(options)
	}
}

// TestDetectExposedPortsWithLabels tests the enhanced DetectExposedPorts with label integration.
func TestDetectExposedPortsWithLabels(t *testing.T) {
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
		validate      func(t *testing.T, ports []ExposedPort)
	}{
		{
			name:        "labels take priority over EXPOSE",
			containerID: "test-container",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80": "ip:127.0.0.1", // Label says IP
				},
				"ExposedPorts": map[string]interface{}{
					"80/tcp": map[string]interface{}{}, // EXPOSE says port 80
				},
			},
			expectedPorts: 1,
			validate: func(t *testing.T, ports []ExposedPort) {
				if ports[0].ExposureType != ExposureTypeIP {
					t.Errorf("Expected IP exposure from label, got %s", ports[0].ExposureType)
				}
				if ports[0].TargetIP != "127.0.0.1" {
					t.Errorf("Expected target IP 127.0.0.1, got %s", ports[0].TargetIP)
				}
			},
		},
		{
			name:        "labels take priority over environment variables",
			containerID: "test-container",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.8080": "i2p",
				},
				"Env": []interface{}{
					"PORT=8080", // Env var also specifies 8080
				},
			},
			expectedPorts: 1,
			validate: func(t *testing.T, ports []ExposedPort) {
				if ports[0].ExposureType != ExposureTypeI2P {
					t.Errorf("Expected I2P exposure from label, got %s", ports[0].ExposureType)
				}
			},
		},
		{
			name:        "non-labeled ports default to I2P",
			containerID: "test-container",
			options: map[string]interface{}{
				"ExposedPorts": map[string]interface{}{
					"80/tcp":  map[string]interface{}{},
					"443/tcp": map[string]interface{}{},
				},
			},
			expectedPorts: 2,
			validate: func(t *testing.T, ports []ExposedPort) {
				for _, port := range ports {
					if port.ExposureType != ExposureTypeI2P {
						t.Errorf("Expected default I2P exposure for port %d, got %s", port.ContainerPort, port.ExposureType)
					}
				}
			},
		},
		{
			name:        "mixed sources with labels, EXPOSE, and env vars",
			containerID: "test-container",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80":  "ip:0.0.0.0",
					"i2p.expose.443": "i2p",
				},
				"ExposedPorts": map[string]interface{}{
					"8080/tcp": map[string]interface{}{},
					"80/tcp":   map[string]interface{}{}, // Should be ignored (label takes priority)
				},
				"Env": []interface{}{
					"PORT=3000",
					"HTTP_PORT=80", // Should be ignored (label takes priority)
				},
			},
			expectedPorts: 4, // 2 from labels + 1 from EXPOSE (8080) + 1 from env (3000)
			validate: func(t *testing.T, ports []ExposedPort) {
				// Count exposure types
				ipCount := 0
				i2pCount := 0
				for _, port := range ports {
					switch port.ContainerPort {
					case 80:
						if port.ExposureType != ExposureTypeIP {
							t.Errorf("Port 80 should be IP exposure from label, got %s", port.ExposureType)
						}
						ipCount++
					case 443:
						if port.ExposureType != ExposureTypeI2P {
							t.Errorf("Port 443 should be I2P exposure from label, got %s", port.ExposureType)
						}
						i2pCount++
					case 8080:
						if port.ExposureType != ExposureTypeI2P {
							t.Errorf("Port 8080 should default to I2P exposure, got %s", port.ExposureType)
						}
						i2pCount++
					case 3000:
						if port.ExposureType != ExposureTypeI2P {
							t.Errorf("Port 3000 should default to I2P exposure, got %s", port.ExposureType)
						}
						i2pCount++
					}
				}
				if ipCount != 1 {
					t.Errorf("Expected 1 IP exposure, got %d", ipCount)
				}
				if i2pCount != 3 {
					t.Errorf("Expected 3 I2P exposures, got %d", i2pCount)
				}
			},
		},
		{
			name:        "deduplication includes exposure type",
			containerID: "test-container",
			options: map[string]interface{}{
				"Labels": map[string]interface{}{
					"i2p.expose.80": "i2p",
				},
				"ExposedPorts": map[string]interface{}{
					"80/tcp": map[string]interface{}{}, // Same port, but would be I2P anyway
				},
			},
			expectedPorts: 1, // Should deduplicate
			validate: func(t *testing.T, ports []ExposedPort) {
				if ports[0].ExposureType != ExposureTypeI2P {
					t.Errorf("Expected I2P exposure, got %s", ports[0].ExposureType)
				}
			},
		},
		{
			name:          "no ports configured",
			containerID:   "test-container",
			options:       map[string]interface{}{},
			expectedPorts: 0,
		},
		{
			name:        "backward compatibility - existing behavior unchanged",
			containerID: "test-container",
			options: map[string]interface{}{
				"ExposedPorts": map[string]interface{}{
					"80/tcp":   map[string]interface{}{},
					"443/tcp":  map[string]interface{}{},
					"8080/tcp": map[string]interface{}{},
				},
				"Env": []interface{}{
					"PORT=3000",
					"HTTP_PORT=9000",
				},
			},
			expectedPorts: 5, // All ports detected
			validate: func(t *testing.T, ports []ExposedPort) {
				// All should default to I2P for backward compatibility
				for _, port := range ports {
					if port.ExposureType != ExposureTypeI2P {
						t.Errorf("Port %d should default to I2P for backward compatibility, got %s",
							port.ContainerPort, port.ExposureType)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports, err := manager.DetectExposedPorts(tt.containerID, tt.options)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(ports) != tt.expectedPorts {
				t.Errorf("Expected %d ports, got %d", tt.expectedPorts, len(ports))
				for i, p := range ports {
					t.Logf("  Port %d: %d/%s (type: %s, target: %s)",
						i, p.ContainerPort, p.Protocol, p.ExposureType, p.TargetIP)
				}
			}

			if tt.validate != nil {
				tt.validate(t, ports)
			}
		})
	}
}

// TestIsPortConfigured tests the port configuration check helper.
func TestIsPortConfigured(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	configuredPorts := []ExposedPort{
		{ContainerPort: 80, Protocol: "tcp", ExposureType: ExposureTypeI2P},
		{ContainerPort: 443, Protocol: "tcp", ExposureType: ExposureTypeIP, TargetIP: "127.0.0.1"},
		{ContainerPort: 8080, Protocol: "tcp", ExposureType: ExposureTypeI2P},
	}

	tests := []struct {
		name     string
		port     int
		expected bool
	}{
		{
			name:     "port is configured",
			port:     80,
			expected: true,
		},
		{
			name:     "port is not configured",
			port:     9090,
			expected: false,
		},
		{
			name:     "another configured port",
			port:     443,
			expected: true,
		},
		{
			name:     "yet another configured port",
			port:     8080,
			expected: true,
		},
		{
			name:     "unconfigured port",
			port:     3000,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isPortConfigured(tt.port, configuredPorts)
			if result != tt.expected {
				t.Errorf("Expected isPortConfigured(%d) to be %v, got %v", tt.port, tt.expected, result)
			}
		})
	}
}
