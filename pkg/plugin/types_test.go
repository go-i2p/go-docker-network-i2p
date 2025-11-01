package plugin

import (
	"encoding/json"
	"testing"
)

func TestTypesSerialization(t *testing.T) {
	tests := []struct {
		name   string
		data   interface{}
		golden string
	}{
		{
			name: "ActivateResponse",
			data: ActivateResponse{
				Implements: []string{"NetworkDriver"},
			},
			golden: `{"Implements":["NetworkDriver"]}`,
		},
		{
			name: "CapabilitiesResponse",
			data: CapabilitiesResponse{
				Scope:             "local",
				ConnectivityScope: "local",
				ErrorResponse:     ErrorResponse{Err: ""},
			},
			golden: `{"Scope":"local","ConnectivityScope":"local","Err":""}`,
		},
		{
			name: "CreateEndpointResponse",
			data: CreateEndpointResponse{
				Interface: &EndpointInterface{
					Address:    "192.168.1.10/24",
					MacAddress: "02:42:ac:14:00:02",
				},
				ErrorResponse: ErrorResponse{Err: ""},
			},
			golden: `{"Interface":{"Address":"192.168.1.10/24","MacAddress":"02:42:ac:14:00:02"},"Err":""}`,
		},
		{
			name: "JoinResponse",
			data: JoinResponse{
				InterfaceName: &InterfaceName{
					SrcName:   "veth",
					DstPrefix: "eth",
				},
				Gateway:       "172.20.0.1",
				ErrorResponse: ErrorResponse{Err: ""},
			},
			golden: `{"InterfaceName":{"SrcName":"veth","DstPrefix":"eth"},"Gateway":"172.20.0.1","Err":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.data)
			if err != nil {
				t.Errorf("Failed to marshal %s: %v", tt.name, err)
				return
			}

			// Compare with expected JSON (allowing for different ordering)
			var expected, actual map[string]interface{}
			if err := json.Unmarshal([]byte(tt.golden), &expected); err != nil {
				t.Errorf("Failed to unmarshal golden data: %v", err)
				return
			}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Errorf("Failed to unmarshal actual data: %v", err)
				return
			}

			// Check that all expected fields are present and correct
			if !mapsEqual(expected, actual) {
				t.Errorf("Marshaled data doesn't match expected.\nExpected: %s\nActual: %s", tt.golden, string(data))
			}
		})
	}
}

func TestTypesDeserialization(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		target   interface{}
		validate func(interface{}) error
	}{
		{
			name:     "CreateNetworkRequest",
			jsonData: `{"NetworkID":"test-net","Options":{"driver":"i2p"}}`,
			target:   &CreateNetworkRequest{},
			validate: func(v interface{}) error {
				req := v.(*CreateNetworkRequest)
				if req.NetworkID != "test-net" {
					t.Errorf("Expected NetworkID 'test-net', got '%s'", req.NetworkID)
				}
				if req.Options["driver"] != "i2p" {
					t.Errorf("Expected driver option 'i2p', got '%v'", req.Options["driver"])
				}
				return nil
			},
		},
		{
			name:     "CreateEndpointRequest",
			jsonData: `{"NetworkID":"test-net","EndpointID":"test-endpoint","Interface":{"Address":"192.168.1.10/24"}}`,
			target:   &CreateEndpointRequest{},
			validate: func(v interface{}) error {
				req := v.(*CreateEndpointRequest)
				if req.NetworkID != "test-net" {
					t.Errorf("Expected NetworkID 'test-net', got '%s'", req.NetworkID)
				}
				if req.EndpointID != "test-endpoint" {
					t.Errorf("Expected EndpointID 'test-endpoint', got '%s'", req.EndpointID)
				}
				if req.Interface == nil || req.Interface.Address != "192.168.1.10/24" {
					t.Errorf("Expected Interface.Address '192.168.1.10/24', got '%v'", req.Interface)
				}
				return nil
			},
		},
		{
			name:     "JoinRequest",
			jsonData: `{"NetworkID":"test-net","EndpointID":"test-endpoint","SandboxKey":"/var/run/docker/netns/container"}`,
			target:   &JoinRequest{},
			validate: func(v interface{}) error {
				req := v.(*JoinRequest)
				if req.NetworkID != "test-net" {
					t.Errorf("Expected NetworkID 'test-net', got '%s'", req.NetworkID)
				}
				if req.EndpointID != "test-endpoint" {
					t.Errorf("Expected EndpointID 'test-endpoint', got '%s'", req.EndpointID)
				}
				if req.SandboxKey != "/var/run/docker/netns/container" {
					t.Errorf("Expected SandboxKey '/var/run/docker/netns/container', got '%s'", req.SandboxKey)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.jsonData), tt.target); err != nil {
				t.Errorf("Failed to unmarshal %s: %v", tt.name, err)
				return
			}

			if err := tt.validate(tt.target); err != nil {
				t.Errorf("Validation failed for %s: %v", tt.name, err)
			}
		})
	}
}

// mapsEqual checks if two maps contain the same key-value pairs
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valueA := range a {
		valueB, exists := b[key]
		if !exists {
			return false
		}

		// Handle nested maps
		if mapA, ok := valueA.(map[string]interface{}); ok {
			if mapB, ok := valueB.(map[string]interface{}); ok {
				if !mapsEqual(mapA, mapB) {
					return false
				}
				continue
			}
			return false
		}

		// Handle slices
		if sliceA, ok := valueA.([]interface{}); ok {
			if sliceB, ok := valueB.([]interface{}); ok {
				if len(sliceA) != len(sliceB) {
					return false
				}
				for i, elemA := range sliceA {
					if elemA != sliceB[i] {
						return false
					}
				}
				continue
			}
			return false
		}

		// Direct comparison for simple types
		if valueA != valueB {
			return false
		}
	}

	return true
}
