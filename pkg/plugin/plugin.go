// Package plugin implements the Docker network plugin interface for I2P connectivity.package plugin

// This package provides the core implementation of Docker's Container Network Model (CNM)
// interfaces, enabling containers to communicate over the I2P network transparently.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

// Plugin represents the I2P Docker network plugin.
type Plugin struct {
	sockPath   string
	listener   net.Listener
	server     *http.Server
	networkMgr *NetworkManager
}

// New creates a new instance of the I2P network plugin.
//
// The sockPath parameter specifies the Unix socket path where the plugin
// will listen for Docker daemon requests. This follows Docker's plugin
// discovery mechanism.
func New(sockPath string) (*Plugin, error) {
	if sockPath == "" {
		return nil, fmt.Errorf("socket path cannot be empty")
	}

	// Create SAM client for I2P connectivity
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create SAM client: %w", err)
	}

	// Create tunnel manager for I2P network operations
	tunnelMgr := i2p.NewTunnelManager(samClient)

	// Create network manager with I2P integration
	networkMgr, err := NewNetworkManager(tunnelMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager: %w", err)
	}

	return &Plugin{
		sockPath:   sockPath,
		networkMgr: networkMgr,
	}, nil
}

// Start begins the plugin operation, listening for Docker daemon requests.
//
// This method sets up the Unix socket listener and HTTP server to handle
// Docker's plugin API calls. It blocks until the context is cancelled.
func (p *Plugin) Start(ctx context.Context) error {
	// Clean up any existing socket file
	if err := os.RemoveAll(p.sockPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", p.sockPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket listener: %w", err)
	}
	p.listener = listener

	// Set socket permissions to allow Docker daemon access
	if err := os.Chmod(p.sockPath, 0600); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Create HTTP server with plugin handlers
	mux := http.NewServeMux()
	p.setupHandlers(mux)

	p.server = &http.Server{
		Handler: mux,
	}

	log.Printf("Plugin listening on %s", p.sockPath)

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("Shutting down plugin server...")
		return p.server.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// setupHandlers configures the HTTP handlers for Docker plugin API endpoints.
//
// This implements the Docker Plugin API v2 specification for network plugins.
// The handlers provide the required endpoints for plugin activation and
// network operations.
func (p *Plugin) setupHandlers(mux *http.ServeMux) {
	// Plugin activation endpoint
	mux.HandleFunc("/Plugin.Activate", p.handleActivate)

	// Network driver endpoints (stub implementations for now)
	mux.HandleFunc("/NetworkDriver.GetCapabilities", p.handleGetCapabilities)
	mux.HandleFunc("/NetworkDriver.CreateNetwork", p.handleCreateNetwork)
	mux.HandleFunc("/NetworkDriver.DeleteNetwork", p.handleDeleteNetwork)
	mux.HandleFunc("/NetworkDriver.CreateEndpoint", p.handleCreateEndpoint)
	mux.HandleFunc("/NetworkDriver.DeleteEndpoint", p.handleDeleteEndpoint)
	mux.HandleFunc("/NetworkDriver.EndpointOperInfo", p.handleEndpointInfo)
	mux.HandleFunc("/NetworkDriver.Join", p.handleJoin)
	mux.HandleFunc("/NetworkDriver.Leave", p.handleLeave)
	mux.HandleFunc("/NetworkDriver.DiscoverNew", p.handleDiscoverNew)
	mux.HandleFunc("/NetworkDriver.DiscoverDelete", p.handleDiscoverDelete)
	mux.HandleFunc("/NetworkDriver.ProgramExternalConnectivity", p.handleProgramExternalConnectivity)
	mux.HandleFunc("/NetworkDriver.RevokeExternalConnectivity", p.handleRevokeExternalConnectivity)
}

// handleActivate responds to Docker's plugin activation request.
//
// This tells Docker that this plugin implements the NetworkDriver interface.
func (p *Plugin) handleActivate(w http.ResponseWriter, r *http.Request) {
	log.Println("Received Plugin.Activate request")

	response := ActivateResponse{
		Implements: []string{"NetworkDriver"},
	}

	p.writeJSONResponse(w, response)
}

// writeJSONResponse is a helper to write JSON responses.
//
// This properly marshals the response data to JSON and handles errors.
// All Docker plugin API responses must be valid JSON.
func (p *Plugin) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		// Fall back to a basic error response
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"Err": "Internal server error"}`))
	}
}

// readJSONRequest is a helper to read and parse JSON requests.
//
// This reads the request body and unmarshals it into the provided structure.
// Returns an error if the request cannot be parsed.
func (p *Plugin) readJSONRequest(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

	if len(body) == 0 {
		// Empty body is acceptable for some requests
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}
