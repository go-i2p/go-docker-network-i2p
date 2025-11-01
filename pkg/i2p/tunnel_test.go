package i2p

import (
	"context"
	"testing"
	"time"
)

func TestDefaultTunnelOptions(t *testing.T) {
	opts := DefaultTunnelOptions()

	expected := TunnelOptions{
		InboundTunnels:  2,
		OutboundTunnels: 2,
		InboundLength:   3,
		OutboundLength:  3,
		InboundBackups:  1,
		OutboundBackups: 1,
		EncryptLeaseset: false,
		CloseIdle:       true,
		CloseIdleTime:   10,
	}

	if opts != expected {
		t.Errorf("DefaultTunnelOptions() = %+v, want %+v", opts, expected)
	}
}

func TestNewTunnelManager(t *testing.T) {
	client, _ := NewSAMClient(nil) // Error expected, but we just need the struct

	tm := NewTunnelManager(client)

	if tm == nil {
		t.Fatal("NewTunnelManager() returned nil")
	}

	if tm.tunnels == nil {
		t.Error("TunnelManager.tunnels map not initialized")
	}

	if tm.containerSessions == nil {
		t.Error("TunnelManager.containerSessions map not initialized")
	}

	if tm.containerSAMClients == nil {
		t.Error("TunnelManager.containerSAMClients map not initialized")
	}

	if len(tm.containerSessions) != 0 {
		t.Errorf("Expected empty container sessions map, got %d entries", len(tm.containerSessions))
	}
}

func TestTunnelManagerCreateTunnel(t *testing.T) {
	client, _ := NewSAMClient(nil) // Error expected for validation purposes
	tm := NewTunnelManager(client)

	tests := []struct {
		name    string
		config  *TunnelConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty tunnel name",
			config: &TunnelConfig{
				Name:        "",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalPort:   8080,
			},
			wantErr: true,
		},
		{
			name: "empty container ID",
			config: &TunnelConfig{
				Name:      "test-tunnel",
				Type:      TunnelTypeClient,
				LocalPort: 8080,
			},
			wantErr: true,
		},
		{
			name: "invalid tunnel type",
			config: &TunnelConfig{
				Name:        "test-tunnel",
				ContainerID: "container-123",
				Type:        "invalid",
				LocalPort:   8080,
			},
			wantErr: true,
		},
		{
			name: "invalid port - zero",
			config: &TunnelConfig{
				Name:        "test-tunnel",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalPort:   0,
			},
			wantErr: true,
		},
		{
			name: "invalid port - negative",
			config: &TunnelConfig{
				Name:        "test-tunnel",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalPort:   -1,
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &TunnelConfig{
				Name:        "test-tunnel",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalPort:   70000,
			},
			wantErr: true,
		},
		{
			name: "valid client tunnel config",
			config: &TunnelConfig{
				Name:        "test-client",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalPort:   8080,
				Destination: "example.b32.i2p",
			},
			wantErr: true, // Will fail because SAM client is not connected
		},
		{
			name: "valid server tunnel config",
			config: &TunnelConfig{
				Name:        "test-server",
				ContainerID: "container-123",
				Type:        TunnelTypeServer,
				LocalPort:   8080,
			},
			wantErr: true, // Will fail because SAM client is not connected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tunnel, err := tm.CreateTunnel(tt.config)

			if tt.wantErr && err == nil {
				t.Errorf("CreateTunnel() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("CreateTunnel() unexpected error: %v", err)
			}

			if !tt.wantErr && tunnel == nil {
				t.Error("CreateTunnel() returned nil tunnel without error")
			}
		})
	}
}

func TestTunnelManagerOperations(t *testing.T) {
	client, _ := NewSAMClient(nil)
	tm := NewTunnelManager(client)

	// Test with empty tunnel manager
	tunnels := tm.ListTunnels()
	if len(tunnels) != 0 {
		t.Errorf("Expected empty tunnel list, got %d tunnels", len(tunnels))
	}

	// Test getting non-existent tunnel
	tunnel, exists := tm.GetTunnel("non-existent")
	if exists {
		t.Error("GetTunnel() returned true for non-existent tunnel")
	}
	if tunnel != nil {
		t.Error("GetTunnel() returned non-nil tunnel for non-existent tunnel")
	}

	// Test destroying non-existent tunnel
	err := tm.DestroyTunnel("non-existent")
	if err == nil {
		t.Error("DestroyTunnel() should return error for non-existent tunnel")
	}

	// Test DestroyAllTunnels on empty manager
	err = tm.DestroyAllTunnels()
	if err != nil {
		t.Errorf("DestroyAllTunnels() unexpected error: %v", err)
	}
}

func TestTunnelConfigValidation(t *testing.T) {
	client, _ := NewSAMClient(nil)
	tm := NewTunnelManager(client)

	tests := []struct {
		name    string
		config  *TunnelConfig
		wantErr bool
	}{
		{
			name: "valid config with default host",
			config: &TunnelConfig{
				Name:        "test",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalPort:   8080,
			},
			wantErr: false,
		},
		{
			name: "valid config with custom host",
			config: &TunnelConfig{
				Name:        "test",
				ContainerID: "container-123",
				Type:        TunnelTypeClient,
				LocalHost:   "0.0.0.0",
				LocalPort:   8080,
			},
			wantErr: false,
		},
		{
			name: "config with zero options (should get defaults)",
			config: &TunnelConfig{
				Name:        "test",
				ContainerID: "container-123",
				Type:        TunnelTypeServer,
				LocalPort:   8080,
				Options:     TunnelOptions{}, // Zero values
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tm.validateTunnelConfig(tt.config)

			if tt.wantErr && err == nil {
				t.Errorf("validateTunnelConfig() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validateTunnelConfig() unexpected error: %v", err)
			}

			// Test that defaults are applied
			if !tt.wantErr {
				if tt.config.LocalHost == "" {
					t.Error("LocalHost should be set to default")
				}
				if tt.config.Options.InboundTunnels == 0 {
					t.Error("Options should be set to defaults when zero")
				}
			}
		})
	}
}

func TestTunnelMethods(t *testing.T) {
	config := &TunnelConfig{
		Name:        "test-tunnel",
		ContainerID: "container-123",
		Type:        TunnelTypeClient,
		LocalHost:   "127.0.0.1",
		LocalPort:   8080,
		Destination: "example.b32.i2p",
		Options:     DefaultTunnelOptions(),
	}

	tunnel := &Tunnel{
		config: config,
		active: true,
	}

	// Test IsActive
	if !tunnel.IsActive() {
		t.Error("Expected tunnel to be active")
	}

	// Test GetConfig
	if tunnel.GetConfig() != config {
		t.Error("GetConfig() returned wrong config")
	}

	// Test GetDestination
	if tunnel.GetDestination() != "example.b32.i2p" {
		t.Errorf("GetDestination() = %s, want %s", tunnel.GetDestination(), "example.b32.i2p")
	}

	// Test GetLocalEndpoint
	expected := "127.0.0.1:8080"
	if tunnel.GetLocalEndpoint() != expected {
		t.Errorf("GetLocalEndpoint() = %s, want %s", tunnel.GetLocalEndpoint(), expected)
	}
}

func TestTunnelManagerContainerSessions(t *testing.T) {
	client, _ := NewSAMClient(nil)
	tm := NewTunnelManager(client)

	// Test listing empty container sessions
	sessions := tm.ListContainerSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected empty container sessions list, got %d sessions", len(sessions))
	}

	// Test getting or creating container session (will fail if SAM not connected)
	_, err := tm.GetOrCreateContainerSession("container-123")
	if err == nil {
		t.Error("GetOrCreateContainerSession() should return error when SAM client not connected")
	}

	// Test destroying non-existent container session (should succeed)
	err = tm.DestroyContainerSession("non-existent")
	if err != nil {
		t.Errorf("DestroyContainerSession() unexpected error: %v", err)
	}
}

// TestTunnelManagerSessionManagement tests session creation and reuse when SAM is available
func TestTunnelManagerSessionManagement(t *testing.T) {
	// Try to create a SAM client with default config
	client, err := NewSAMClient(nil)
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	// Try to connect to SAM
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Give more time for session creation
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to SAM bridge: %v", err)
	}
	defer client.Disconnect()

	tm := NewTunnelManager(client)

	// Test creating a primary session for a container
	containerID := "test-container-123"
	session1, err := tm.GetOrCreateContainerSession(containerID)
	if err != nil {
		// Session creation may fail if I2P router is not fully configured
		// This is expected in unit test environment
		t.Fatalf("Failed to create container session - I2P router may not be fully configured: %v", err)
	}

	if session1 == nil {
		t.Fatal("Created session is nil")
	}

	// Test that subsequent calls return the same session (reuse)
	session2, err := tm.GetOrCreateContainerSession(containerID)
	if err != nil {
		t.Fatalf("Failed to get existing container session: %v", err)
	}

	if session1 != session2 {
		t.Error("Expected same session object on second call (session reuse)")
	}

	// Verify the session is listed
	sessions := tm.ListContainerSessions()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 container session, got %d", len(sessions))
	}

	if sessions[0] != containerID {
		t.Errorf("Expected container ID %s in session list, got %s", containerID, sessions[0])
	}

	// Test creating a different container's session
	containerID2 := "test-container-456"
	session3, err := tm.GetOrCreateContainerSession(containerID2)
	if err != nil {
		t.Fatalf("Failed to create second container session: %v", err)
	}

	if session1 == session3 {
		t.Error("Expected different session objects for different containers")
	}

	// Now we should have 2 sessions
	sessions = tm.ListContainerSessions()
	if len(sessions) != 2 {
		t.Errorf("Expected 2 container sessions, got %d", len(sessions))
	}

	// Test destroying a container session
	err = tm.DestroyContainerSession(containerID)
	if err != nil {
		t.Errorf("Failed to destroy container session: %v", err)
	}

	// Should now have 1 session
	sessions = tm.ListContainerSessions()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 container session after destroy, got %d", len(sessions))
	}

	// Clean up remaining session
	err = tm.DestroyContainerSession(containerID2)
	if err != nil {
		t.Errorf("Failed to destroy second container session: %v", err)
	}

	// Should now have 0 sessions
	sessions = tm.ListContainerSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 container sessions after cleanup, got %d", len(sessions))
	}
}
