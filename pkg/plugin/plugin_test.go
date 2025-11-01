package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		sockPath string
		wantErr  bool
	}{
		{
			name:     "valid socket path",
			sockPath: "/tmp/test.sock",
			wantErr:  false,
		},
		{
			name:     "empty socket path",
			sockPath: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := New(tt.sockPath)

			if tt.wantErr && err == nil {
				t.Errorf("New() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("New() unexpected error: %v", err)
			}

			if !tt.wantErr && plugin == nil {
				t.Errorf("New() returned nil plugin")
			}
		})
	}
}

func TestPluginStart(t *testing.T) {
	// Create temporary directory for test socket
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	plugin, err := New(sockPath)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Create a context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start the plugin in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- plugin.Start(ctx)
	}()

	// Give the plugin time to start
	time.Sleep(50 * time.Millisecond)

	// Check that socket file was created
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Errorf("Socket file was not created at %s", sockPath)
	}

	// Wait for context timeout and plugin shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Plugin.Start() returned error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Plugin.Start() did not return within timeout")
	}
}

func TestJSONResponseHandling(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedFields []string
	}{
		{
			name:           "activate response",
			handler:        plugin.handleActivate,
			expectedFields: []string{"Implements"},
		},
		{
			name:           "capabilities response",
			handler:        plugin.handleGetCapabilities,
			expectedFields: []string{"Scope", "ConnectivityScope", "Err"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status OK, got %d", w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			// Parse response to ensure it's valid JSON
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Response is not valid JSON: %v", err)
			}

			// Check that expected fields are present
			for _, field := range tt.expectedFields {
				if _, exists := response[field]; !exists {
					t.Errorf("Expected field %s not found in response", field)
				}
			}
		})
	}
}

func TestRequestParsing(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		requestBody    string
		expectValidErr bool
	}{
		{
			name:           "create network with valid JSON",
			handler:        plugin.handleCreateNetwork,
			requestBody:    `{"NetworkID":"test-network","Options":{}}`,
			expectValidErr: false,
		},
		{
			name:           "create network with invalid JSON",
			handler:        plugin.handleCreateNetwork,
			requestBody:    `{"NetworkID":"test-network",invalid}`,
			expectValidErr: true,
		},
		{
			name:           "create endpoint with valid JSON",
			handler:        plugin.handleCreateEndpoint,
			requestBody:    `{"NetworkID":"test-network","EndpointID":"test-endpoint"}`,
			expectValidErr: false,
		},
		{
			name:           "join with valid JSON",
			handler:        plugin.handleJoin,
			requestBody:    `{"NetworkID":"test-network","EndpointID":"test-endpoint","SandboxKey":"sandbox"}`,
			expectValidErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.requestBody))
			w := httptest.NewRecorder()

			tt.handler(w, req)

			// Parse response to check for errors
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Response is not valid JSON: %v", err)
			}

			errField, hasErr := response["Err"]
			if !hasErr {
				t.Errorf("Response missing Err field")
				return
			}

			errString, ok := errField.(string)
			if !ok {
				t.Errorf("Err field is not a string")
				return
			}

			hasError := errString != ""
			if tt.expectValidErr && !hasError {
				t.Errorf("Expected error in response, but got none")
			}
			if !tt.expectValidErr && hasError {
				t.Errorf("Unexpected error in response: %s", errString)
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Test with empty body
	req := httptest.NewRequest("POST", "/", nil)
	w := httptest.NewRecorder()

	plugin.handleCreateNetwork(w, req)

	var response ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	// Empty body should result in an error for missing network ID
	if response.Err == "" {
		t.Error("Expected error for empty body (missing network ID), but got success")
	}
}

// TestEndpointLifecycle tests the complete endpoint lifecycle from creation to deletion.
func TestEndpointLifecycle(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// First create a network
	networkID := "test-endpoint-lifecycle-network"
	createNetworkReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"Options": {}
	}`))
	w := httptest.NewRecorder()
	plugin.handleCreateNetwork(w, createNetworkReq)

	var createNetworkResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createNetworkResp); err != nil {
		t.Fatalf("Failed to parse CreateNetwork response: %v", err)
	}
	if createNetworkResp.Err != "" {
		t.Fatalf("Failed to create network: %s", createNetworkResp.Err)
	}

	// Test endpoint creation
	endpointID := "test-endpoint-lifecycle-endpoint"
	createEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleCreateEndpoint(w, createEndpointReq)

	var createEndpointResp CreateEndpointResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createEndpointResp); err != nil {
		t.Fatalf("Failed to parse CreateEndpoint response: %v", err)
	}
	if createEndpointResp.Err != "" {
		t.Fatalf("Failed to create endpoint: %s", createEndpointResp.Err)
	}

	// Verify endpoint has an interface
	if createEndpointResp.Interface == nil {
		t.Error("CreateEndpoint response missing Interface")
	} else {
		if createEndpointResp.Interface.Address == "" {
			t.Error("CreateEndpoint response Interface missing Address")
		}
		if createEndpointResp.Interface.MacAddress == "" {
			t.Error("CreateEndpoint response Interface missing MacAddress")
		}
	}

	// Test endpoint join
	sandboxKey := "/var/run/docker/netns/test-container-123"
	joinReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`",
		"SandboxKey": "`+sandboxKey+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleJoin(w, joinReq)

	var joinResp JoinResponse
	if err := json.Unmarshal(w.Body.Bytes(), &joinResp); err != nil {
		t.Fatalf("Failed to parse Join response: %v", err)
	}
	if joinResp.Err != "" {
		t.Fatalf("Failed to join endpoint: %s", joinResp.Err)
	}

	// Verify join response has required fields
	if joinResp.InterfaceName == nil {
		t.Error("Join response missing InterfaceName")
	} else {
		if joinResp.InterfaceName.SrcName == "" {
			t.Error("Join response InterfaceName missing SrcName")
		}
		if joinResp.InterfaceName.DstPrefix == "" {
			t.Error("Join response InterfaceName missing DstPrefix")
		}
	}
	if joinResp.Gateway == "" {
		t.Error("Join response missing Gateway")
	}

	// Test endpoint leave
	leaveReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleLeave(w, leaveReq)

	var leaveResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &leaveResp); err != nil {
		t.Fatalf("Failed to parse Leave response: %v", err)
	}
	if leaveResp.Err != "" {
		t.Fatalf("Failed to leave endpoint: %s", leaveResp.Err)
	}

	// Test endpoint deletion
	deleteEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleDeleteEndpoint(w, deleteEndpointReq)

	var deleteEndpointResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &deleteEndpointResp); err != nil {
		t.Fatalf("Failed to parse DeleteEndpoint response: %v", err)
	}
	if deleteEndpointResp.Err != "" {
		t.Fatalf("Failed to delete endpoint: %s", deleteEndpointResp.Err)
	}

	// Clean up network
	deleteNetworkReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleDeleteNetwork(w, deleteNetworkReq)

	var deleteNetworkResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &deleteNetworkResp); err != nil {
		t.Fatalf("Failed to parse DeleteNetwork response: %v", err)
	}
	if deleteNetworkResp.Err != "" {
		t.Fatalf("Failed to delete network: %s", deleteNetworkResp.Err)
	}
}

// TestEndpointErrorCases tests various error conditions in endpoint management.
func TestEndpointErrorCases(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		requestBody string
		expectError bool
	}{
		{
			name:        "create endpoint without network ID",
			handler:     plugin.handleCreateEndpoint,
			requestBody: `{"EndpointID":"test-endpoint"}`,
			expectError: true,
		},
		{
			name:        "create endpoint without endpoint ID",
			handler:     plugin.handleCreateEndpoint,
			requestBody: `{"NetworkID":"test-network"}`,
			expectError: true,
		},
		{
			name:        "create endpoint on non-existent network",
			handler:     plugin.handleCreateEndpoint,
			requestBody: `{"NetworkID":"non-existent","EndpointID":"test-endpoint"}`,
			expectError: true,
		},
		{
			name:        "join with empty network ID",
			handler:     plugin.handleJoin,
			requestBody: `{"EndpointID":"test-endpoint","SandboxKey":"sandbox"}`,
			expectError: true,
		},
		{
			name:        "join with empty endpoint ID",
			handler:     plugin.handleJoin,
			requestBody: `{"NetworkID":"test-network","SandboxKey":"sandbox"}`,
			expectError: true,
		},
		{
			name:        "join non-existent endpoint",
			handler:     plugin.handleJoin,
			requestBody: `{"NetworkID":"test-network","EndpointID":"non-existent","SandboxKey":"sandbox"}`,
			expectError: true,
		},
		{
			name:        "leave with empty network ID",
			handler:     plugin.handleLeave,
			requestBody: `{"EndpointID":"test-endpoint"}`,
			expectError: true,
		},
		{
			name:        "leave with empty endpoint ID",
			handler:     plugin.handleLeave,
			requestBody: `{"NetworkID":"test-network"}`,
			expectError: true,
		},
		{
			name:        "delete endpoint with empty network ID",
			handler:     plugin.handleDeleteEndpoint,
			requestBody: `{"EndpointID":"test-endpoint"}`,
			expectError: true,
		},
		{
			name:        "delete endpoint with empty endpoint ID",
			handler:     plugin.handleDeleteEndpoint,
			requestBody: `{"NetworkID":"test-network"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.requestBody))
			w := httptest.NewRecorder()

			tt.handler(w, req)

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Response is not valid JSON: %v", err)
			}

			errField, hasErr := response["Err"]
			if !hasErr {
				t.Errorf("Response missing Err field")
				return
			}

			errString, ok := errField.(string)
			if !ok {
				t.Errorf("Err field is not a string")
				return
			}

			hasError := errString != ""
			if tt.expectError && !hasError {
				t.Errorf("Expected error in response, but got none")
			}
			if !tt.expectError && hasError {
				t.Errorf("Unexpected error in response: %s", errString)
			}
		})
	}
}

// TestEndpointDuplicateCreation tests handling of duplicate endpoint creation.
func TestEndpointDuplicateCreation(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Create a network first
	networkID := "test-duplicate-network"
	createNetworkReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"Options": {}
	}`))
	w := httptest.NewRecorder()
	plugin.handleCreateNetwork(w, createNetworkReq)

	var createNetworkResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createNetworkResp); err != nil {
		t.Fatalf("Failed to parse CreateNetwork response: %v", err)
	}
	if createNetworkResp.Err != "" {
		t.Fatalf("Failed to create network: %s", createNetworkResp.Err)
	}

	// Create an endpoint
	endpointID := "test-duplicate-endpoint"
	createEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleCreateEndpoint(w, createEndpointReq)

	var createEndpointResp CreateEndpointResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createEndpointResp); err != nil {
		t.Fatalf("Failed to parse CreateEndpoint response: %v", err)
	}
	if createEndpointResp.Err != "" {
		t.Fatalf("Failed to create endpoint: %s", createEndpointResp.Err)
	}

	// Try to create the same endpoint again - should fail
	duplicateEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleCreateEndpoint(w, duplicateEndpointReq)

	var duplicateResp CreateEndpointResponse
	if err := json.Unmarshal(w.Body.Bytes(), &duplicateResp); err != nil {
		t.Fatalf("Failed to parse duplicate CreateEndpoint response: %v", err)
	}
	if duplicateResp.Err == "" {
		t.Error("Expected error when creating duplicate endpoint, but got success")
	}

	// Clean up
	deleteEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"EndpointID": "`+endpointID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleDeleteEndpoint(w, deleteEndpointReq)

	deleteNetworkReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleDeleteNetwork(w, deleteNetworkReq)
}

// TestMultipleEndpointsOnNetwork tests creating multiple endpoints on the same network.
func TestMultipleEndpointsOnNetwork(t *testing.T) {
	plugin, err := New("/tmp/test.sock")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Create a network
	networkID := "test-multiple-endpoints-network"
	createNetworkReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`",
		"Options": {}
	}`))
	w := httptest.NewRecorder()
	plugin.handleCreateNetwork(w, createNetworkReq)

	var createNetworkResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createNetworkResp); err != nil {
		t.Fatalf("Failed to parse CreateNetwork response: %v", err)
	}
	if createNetworkResp.Err != "" {
		t.Fatalf("Failed to create network: %s", createNetworkResp.Err)
	}

	// Create multiple endpoints
	endpointIDs := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	endpointIPs := make([]string, len(endpointIDs))

	for i, endpointID := range endpointIDs {
		createEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
			"NetworkID": "`+networkID+`",
			"EndpointID": "`+endpointID+`"
		}`))
		w := httptest.NewRecorder()
		plugin.handleCreateEndpoint(w, createEndpointReq)

		var createEndpointResp CreateEndpointResponse
		if err := json.Unmarshal(w.Body.Bytes(), &createEndpointResp); err != nil {
			t.Fatalf("Failed to parse CreateEndpoint response for %s: %v", endpointID, err)
		}
		if createEndpointResp.Err != "" {
			t.Fatalf("Failed to create endpoint %s: %s", endpointID, createEndpointResp.Err)
		}

		// Store IP address for uniqueness check
		if createEndpointResp.Interface != nil {
			endpointIPs[i] = createEndpointResp.Interface.Address
		}
	}

	// Verify all endpoints got unique IP addresses
	for i := 0; i < len(endpointIPs); i++ {
		if endpointIPs[i] == "" {
			t.Errorf("Endpoint %s did not receive an IP address", endpointIDs[i])
			continue
		}
		for j := i + 1; j < len(endpointIPs); j++ {
			if endpointIPs[i] == endpointIPs[j] {
				t.Errorf("Endpoints %s and %s have the same IP address: %s",
					endpointIDs[i], endpointIDs[j], endpointIPs[i])
			}
		}
	}

	// Clean up all endpoints
	for _, endpointID := range endpointIDs {
		deleteEndpointReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
			"NetworkID": "`+networkID+`",
			"EndpointID": "`+endpointID+`"
		}`))
		w = httptest.NewRecorder()
		plugin.handleDeleteEndpoint(w, deleteEndpointReq)

		var deleteEndpointResp ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &deleteEndpointResp); err != nil {
			t.Errorf("Failed to parse DeleteEndpoint response for %s: %v", endpointID, err)
		}
		if deleteEndpointResp.Err != "" {
			t.Errorf("Failed to delete endpoint %s: %s", endpointID, deleteEndpointResp.Err)
		}
	}

	// Clean up network
	deleteNetworkReq := httptest.NewRequest("POST", "/", strings.NewReader(`{
		"NetworkID": "`+networkID+`"
	}`))
	w = httptest.NewRecorder()
	plugin.handleDeleteNetwork(w, deleteNetworkReq)
}
