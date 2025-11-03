package service

import (
	"context"
	"fmt"
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
			expectedPorts: 2, // Both IP (label) and I2P (EXPOSE) exposures allowed per Issue #14 fix
			validate: func(t *testing.T, ports []ExposedPort) {
				// Port 80 should have both IP and I2P exposures
				ipFound := false
				i2pFound := false
				for _, port := range ports {
					if port.ContainerPort == 80 {
						if port.ExposureType == ExposureTypeIP {
							if port.TargetIP != "127.0.0.1" {
								t.Errorf("Expected target IP 127.0.0.1, got %s", port.TargetIP)
							}
							ipFound = true
						} else if port.ExposureType == ExposureTypeI2P {
							i2pFound = true
						}
					}
				}
				if !ipFound {
					t.Errorf("Expected IP exposure from label for port 80")
				}
				if !i2pFound {
					t.Errorf("Expected I2P exposure from EXPOSE for port 80")
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
					"80/tcp":   map[string]interface{}{}, // Creates I2P exposure (different type than label's IP)
				},
				"Env": []interface{}{
					"PORT=3000",
					"HTTP_PORT=80", // Creates I2P exposure (different type than label's IP)
				},
			},
			expectedPorts: 5, // Per Issue #14 fix: 2 from labels + 1 EXPOSE(8080) + 1 EXPOSE/env(80 I2P deduplicated) + 1 env(3000)
			validate: func(t *testing.T, ports []ExposedPort) {
				// Count exposures per port/type
				portExposures := make(map[string]int) // key: "port/type"
				for _, port := range ports {
					key := fmt.Sprintf("%d/%s", port.ContainerPort, port.ExposureType)
					portExposures[key]++

					// Validate specific configurations
					if port.ContainerPort == 80 && port.ExposureType == ExposureTypeIP {
						if port.TargetIP != "0.0.0.0" {
							t.Errorf("Port 80 IP exposure should have target 0.0.0.0, got %s", port.TargetIP)
						}
					}
				}

				// Verify expected exposures
				expectedExposures := map[string]int{
					"80/ip":    1, // From label
					"80/i2p":   1, // From EXPOSE (deduplicated with env's HTTP_PORT=80)
					"443/i2p":  1, // From label
					"8080/i2p": 1, // From EXPOSE
					"3000/i2p": 1, // From env
				}

				for key, expectedCount := range expectedExposures {
					if actualCount := portExposures[key]; actualCount != expectedCount {
						t.Errorf("Expected %d exposure for %s, got %d", expectedCount, key, actualCount)
					}
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
		name         string
		port         int
		exposureType ExposureType
		expected     bool
	}{
		{
			name:         "port is configured for I2P",
			port:         80,
			exposureType: ExposureTypeI2P,
			expected:     true,
		},
		{
			name:         "port is not configured",
			port:         9090,
			exposureType: ExposureTypeI2P,
			expected:     false,
		},
		{
			name:         "port configured for IP",
			port:         443,
			exposureType: ExposureTypeIP,
			expected:     true,
		},
		{
			name:         "another I2P configured port",
			port:         8080,
			exposureType: ExposureTypeI2P,
			expected:     true,
		},
		{
			name:         "unconfigured port",
			port:         3000,
			exposureType: ExposureTypeI2P,
			expected:     false,
		},
		{
			name:         "port configured for different type",
			port:         443,
			exposureType: ExposureTypeI2P,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isPortConfigured(tt.port, tt.exposureType, configuredPorts)
			if result != tt.expected {
				t.Errorf("Expected isPortConfigured(%d, %s) to be %v, got %v", tt.port, tt.exposureType, tt.expected, result)
			}
		})
	}
}

// TestCreateIPServiceExposure tests IP-based service exposure creation.
func TestCreateIPServiceExposure(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	containerID := "test-container-123456789012"
	containerIP := net.ParseIP("172.20.0.5")

	tests := []struct {
		name        string
		port        ExposedPort
		shouldError bool
		validate    func(t *testing.T, exposure *ServiceExposure, err error)
	}{
		{
			name: "valid IP exposure with explicit target",
			port: ExposedPort{
				ContainerPort: 18080, // Use unprivileged port
				Protocol:      "tcp",
				ServiceName:   "web",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "127.0.0.1",
			},
			shouldError: false,
			validate: func(t *testing.T, exposure *ServiceExposure, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if exposure == nil {
					t.Fatal("Expected exposure to be created")
				}
				if exposure.Tunnel != nil {
					t.Error("IP exposure should not have I2P tunnel")
				}
				if exposure.Forwarder == nil {
					t.Error("IP exposure should have forwarder")
				}
				if exposure.Destination != "127.0.0.1:18080" {
					t.Errorf("Expected destination 127.0.0.1:18080, got %s", exposure.Destination)
				}
				if !strings.HasPrefix(exposure.TunnelName, "ip-") {
					t.Errorf("Expected tunnel name to start with 'ip-', got %s", exposure.TunnelName)
				}
				if exposure.ContainerID != containerID {
					t.Errorf("Expected container ID %s, got %s", containerID, exposure.ContainerID)
				}
				// Cleanup
				if exposure.Forwarder != nil {
					exposure.Forwarder.Stop()
				}
			},
		},
		{
			name: "IP exposure defaults to localhost",
			port: ExposedPort{
				ContainerPort: 18443, // Use unprivileged port
				Protocol:      "tcp",
				ServiceName:   "https",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "", // Empty - should default
			},
			shouldError: false,
			validate: func(t *testing.T, exposure *ServiceExposure, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if exposure.Destination != "127.0.0.1:18443" {
					t.Errorf("Expected default destination 127.0.0.1:18443, got %s", exposure.Destination)
				}
				// Cleanup
				if exposure != nil && exposure.Forwarder != nil {
					exposure.Forwarder.Stop()
				}
			},
		},
		{
			name: "IP exposure with external IP",
			port: ExposedPort{
				ContainerPort: 18081, // Use unprivileged port
				Protocol:      "tcp",
				ServiceName:   "api",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "0.0.0.0",
			},
			shouldError: false,
			validate: func(t *testing.T, exposure *ServiceExposure, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if exposure.Destination != "0.0.0.0:18081" {
					t.Errorf("Expected destination 0.0.0.0:18081, got %s", exposure.Destination)
				}
				// Cleanup
				if exposure != nil && exposure.Forwarder != nil {
					exposure.Forwarder.Stop()
				}
			},
		},
		{
			name: "IP exposure with IPv6 address",
			port: ExposedPort{
				ContainerPort: 19090, // Use unprivileged port
				Protocol:      "tcp",
				ServiceName:   "metrics",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "::1",
			},
			shouldError: false,
			validate: func(t *testing.T, exposure *ServiceExposure, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if exposure.Destination != "::1:19090" {
					t.Errorf("Expected destination ::1:19090, got %s", exposure.Destination)
				}
				// Cleanup
				if exposure != nil && exposure.Forwarder != nil {
					exposure.Forwarder.Stop()
				}
			},
		},
		{
			name: "invalid target IP address",
			port: ExposedPort{
				ContainerPort: 80,
				Protocol:      "tcp",
				ServiceName:   "web",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "invalid-ip",
			},
			shouldError: true,
			validate: func(t *testing.T, exposure *ServiceExposure, err error) {
				if err == nil {
					t.Error("Expected error for invalid IP address")
				}
				if !strings.Contains(err.Error(), "invalid target IP address") {
					t.Errorf("Expected error about invalid IP, got: %v", err)
				}
				if exposure != nil {
					t.Error("Expected no exposure to be created for invalid IP")
				}
			},
		},
		{
			name: "IP exposure with localhost (skip bind test)",
			port: ExposedPort{
				ContainerPort: 13000, // Use unprivileged port
				Protocol:      "tcp",
				ServiceName:   "dev",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "127.0.0.1",
			},
			shouldError: false,
			validate: func(t *testing.T, exposure *ServiceExposure, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if exposure.Destination != "127.0.0.1:13000" {
					t.Errorf("Expected destination 127.0.0.1:13000, got %s", exposure.Destination)
				}
				// Cleanup
				if exposure != nil && exposure.Forwarder != nil {
					exposure.Forwarder.Stop()
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exposure, err := manager.createIPServiceExposure(containerID, containerIP, tt.port)
			tt.validate(t, exposure, err)
		})
	}
}

// TestExposeServicesWithMixedTypes tests ExposeServices with both I2P and IP exposure types.
func TestExposeServicesWithMixedTypes(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	// Connect to SAM bridge
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

	containerID := "test-container-mixed"
	networkID := "test-network"
	containerIP := net.ParseIP("172.20.0.10")

	// Test with mixed exposure types (using unprivileged ports)
	ports := []ExposedPort{
		{
			ContainerPort: 80,
			Protocol:      "tcp",
			ServiceName:   "web",
			ExposureType:  ExposureTypeI2P,
		},
		{
			ContainerPort: 18443, // Use unprivileged port
			Protocol:      "tcp",
			ServiceName:   "https",
			ExposureType:  ExposureTypeIP,
			TargetIP:      "127.0.0.1",
		},
		{
			ContainerPort: 18082, // Use unprivileged port
			Protocol:      "tcp",
			ServiceName:   "api",
			ExposureType:  ExposureTypeIP,
			TargetIP:      "127.0.0.1", // Use localhost to avoid port conflicts
		},
	}

	exposures, err := manager.ExposeServices(containerID, networkID, containerIP, ports)
	if err != nil {
		t.Fatalf("Failed to expose services: %v", err)
	}

	// Cleanup forwarders
	defer func() {
		for _, exp := range exposures {
			if exp.Forwarder != nil {
				exp.Forwarder.Stop()
			}
		}
	}()

	if len(exposures) != 3 {
		t.Fatalf("Expected 3 exposures, got %d", len(exposures))
	}

	// Verify I2P exposure
	i2pExposure := exposures[0]
	if i2pExposure.Tunnel == nil {
		t.Error("I2P exposure should have a tunnel")
	}
	if !strings.HasSuffix(i2pExposure.Destination, ".b32.i2p") {
		t.Errorf("I2P exposure should have .b32.i2p destination, got %s", i2pExposure.Destination)
	}

	// Verify IP exposures
	for i := 1; i < 3; i++ {
		if exposures[i].Tunnel != nil {
			t.Errorf("IP exposure %d should not have a tunnel", i)
		}
		if !strings.HasPrefix(exposures[i].TunnelName, "ip-") {
			t.Errorf("IP exposure %d should have 'ip-' prefix in tunnel name, got %s", i, exposures[i].TunnelName)
		}
	}

	// Verify destinations (using the updated port numbers from the test)
	if exposures[1].Destination != "127.0.0.1:18443" {
		t.Errorf("Expected destination 127.0.0.1:18443, got %s", exposures[1].Destination)
	}
	if exposures[2].Destination != "127.0.0.1:18082" {
		t.Errorf("Expected destination 127.0.0.1:18082, got %s", exposures[2].Destination)
	}

	// Verify forwarders exist for IP exposures
	if exposures[1].Forwarder == nil {
		t.Error("IP exposure should have forwarder")
	}
	if exposures[2].Forwarder == nil {
		t.Error("IP exposure should have forwarder")
	}

	// Clean up
	err = manager.CleanupServices(containerID)
	if err != nil {
		t.Errorf("Failed to cleanup services: %v", err)
	}
}

// TestExposeServicesDefaultType tests that unspecified exposure type defaults to I2P.
func TestExposeServicesDefaultType(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

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

	containerID := "test-container-default"
	networkID := "test-network"
	containerIP := net.ParseIP("172.20.0.11")

	// Port with no explicit exposure type
	ports := []ExposedPort{
		{
			ContainerPort: 80,
			Protocol:      "tcp",
			ServiceName:   "web",
			ExposureType:  "", // Empty - should default to I2P
		},
	}

	exposures, err := manager.ExposeServices(containerID, networkID, containerIP, ports)
	if err != nil {
		t.Fatalf("Failed to expose services: %v", err)
	}

	if len(exposures) != 1 {
		t.Fatalf("Expected 1 exposure, got %d", len(exposures))
	}

	// Should default to I2P exposure
	if exposures[0].Tunnel == nil {
		t.Error("Default exposure should be I2P with tunnel")
	}
	if !strings.HasSuffix(exposures[0].Destination, ".b32.i2p") {
		t.Errorf("Default exposure should have .b32.i2p destination, got %s", exposures[0].Destination)
	}

	// Clean up
	err = manager.CleanupServices(containerID)
	if err != nil {
		t.Errorf("Failed to cleanup services: %v", err)
	}
}

// TestUDPPortForwarding tests UDP port forwarding functionality.
func TestUDPPortForwarding(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	containerID := "test-container-udp"
	containerIP := net.ParseIP("172.20.0.20")

	// Test UDP port exposure
	port := ExposedPort{
		ContainerPort: 15353, // Use unprivileged port
		Protocol:      "udp",
		ServiceName:   "dns",
		ExposureType:  ExposureTypeIP,
		TargetIP:      "127.0.0.1",
	}

	exposure, err := manager.createIPServiceExposure(containerID, containerIP, port)
	if err != nil {
		t.Fatalf("Failed to create UDP exposure: %v", err)
	}
	defer exposure.Forwarder.Stop()

	// Verify exposure properties
	if exposure.Forwarder == nil {
		t.Fatal("UDP exposure should have forwarder")
	}
	if exposure.Forwarder.protocol != "udp" {
		t.Errorf("Expected protocol udp, got %s", exposure.Forwarder.protocol)
	}
	if exposure.Forwarder.packetConn == nil {
		t.Error("UDP forwarder should have packet connection")
	}
	if exposure.Forwarder.listener != nil {
		t.Error("UDP forwarder should not have TCP listener")
	}
	if exposure.Destination != "127.0.0.1:15353" {
		t.Errorf("Expected destination 127.0.0.1:15353, got %s", exposure.Destination)
	}
}

// TestTCPAndUDPMixedForwarding tests both TCP and UDP exposures in same container.
func TestTCPAndUDPMixedForwarding(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager, err := NewServiceExposureManager(tunnelMgr)
	if err != nil {
		t.Fatalf("Failed to create service exposure manager: %v", err)
	}

	containerID := "test-container-mixed-protocols"
	containerIP := net.ParseIP("172.20.0.21")

	tests := []struct {
		name     string
		port     ExposedPort
		expected string
	}{
		{
			name: "TCP port",
			port: ExposedPort{
				ContainerPort: 18888,
				Protocol:      "tcp",
				ServiceName:   "http",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "127.0.0.1",
			},
			expected: "tcp",
		},
		{
			name: "UDP port",
			port: ExposedPort{
				ContainerPort: 18889,
				Protocol:      "udp",
				ServiceName:   "dns",
				ExposureType:  ExposureTypeIP,
				TargetIP:      "127.0.0.1",
			},
			expected: "udp",
		},
	}

	var exposures []*ServiceExposure
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exposure, err := manager.createIPServiceExposure(containerID, containerIP, tt.port)
			if err != nil {
				t.Fatalf("Failed to create exposure: %v", err)
			}
			exposures = append(exposures, exposure)

			if exposure.Forwarder == nil {
				t.Fatal("Exposure should have forwarder")
			}
			if exposure.Forwarder.protocol != tt.expected {
				t.Errorf("Expected protocol %s, got %s", tt.expected, exposure.Forwarder.protocol)
			}
		})
	}

	// Cleanup
	for _, exp := range exposures {
		if exp.Forwarder != nil {
			exp.Forwarder.Stop()
		}
	}
}
