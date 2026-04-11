package i2p

import (
	"context"
	"testing"
	"time"
)

func TestDefaultSAMConfig(t *testing.T) {
	config := DefaultSAMConfig()

	if config.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", config.Host)
	}

	if config.Port != 7656 {
		t.Errorf("Expected port 7656, got %d", config.Port)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}
}

func TestNewSAMClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *SAMConfig
		wantErr bool
	}{
		{
			name:    "default config (uses default SAM address)",
			config:  nil,   // Will use DefaultSAMConfig()
			wantErr: false, // If a SAM API is listening this should succeed
		},
		{
			name: "unreachable port on localhost",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    65433, // Use an unlikely port
				Timeout: 1 * time.Second,
			},
			wantErr: false, // Config fields are valid; connection failure deferred to Connect()
		},
		{
			name: "unreachable port on 127.0.0.1",
			config: &SAMConfig{
				Host:    "127.0.0.1",
				Port:    65432, // Use an unlikely port
				Timeout: 1 * time.Second,
			},
			wantErr: false, // Config fields are valid; connection failure deferred to Connect()
		},
		{
			name: "invalid port",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    -1,
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "empty host",
			config: &SAMConfig{
				Host:    "",
				Port:    7656,
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    7656,
				Timeout: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSAMClient(tt.config)

			if tt.wantErr && err == nil {
				t.Errorf("NewSAMClient() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("NewSAMClient() unexpected error: %v", err)
			}

			// For successful creation, test basic properties
			if !tt.wantErr && client != nil {
				if !client.IsConnected() {
					// This is expected since we haven't called Connect()
				}

				if client.GetSAMVersion() != "" {
					t.Errorf("Expected empty version before connection, got '%s'", client.GetSAMVersion())
				}
			}
		})
	}
}

func TestSAMClientConnectionLifecycle(t *testing.T) {
	// Test with a port that immediately refuses connections
	config := &SAMConfig{
		Host:    "127.0.0.1",
		Port:    65434, // Unlikely to have a listener
		Timeout: 2 * time.Second,
	}

	client, err := NewSAMClient(config)
	if err != nil {
		t.Fatalf("Unexpected error creating client: %v", err)
	}

	// Connection should fail because nothing is listening on this port
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err == nil {
		t.Error("Expected connection error for unreachable port")
	}

	// Test disconnect (should not panic even if not connected)
	err = client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() returned unexpected error: %v", err)
	}
}

func TestValidateSAMConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *SAMConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &SAMConfig{
				Host:    "127.0.0.1",
				Port:    7656,
				Timeout: 10 * time.Second,
			},
			wantErr: false, // Should succeed with working I2P SAM API
		},
		{
			name: "empty host",
			config: &SAMConfig{
				Host:    "",
				Port:    7656,
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid port - negative",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    -1,
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    70000,
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    7656,
				Timeout: 0,
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: &SAMConfig{
				Host:    "localhost",
				Port:    7656,
				Timeout: -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSAMConfig(tt.config)

			if tt.wantErr && err == nil {
				t.Errorf("validateSAMConfig() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validateSAMConfig() unexpected error: %v", err)
			}
		})
	}
}
