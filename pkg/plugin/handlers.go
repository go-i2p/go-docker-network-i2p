package plugin

import (
	"log"
	"net/http"
	"strings"
)

// handleGetCapabilities returns the capabilities of the network driver.
//
// This tells Docker what features this network plugin supports.
// For I2P networks, we support local scope networking.
func (p *Plugin) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.GetCapabilities request")

	response := CapabilitiesResponse{
		Scope:             "local",
		ConnectivityScope: "local",
		ErrorResponse:     ErrorResponse{Err: ""},
	}

	p.writeJSONResponse(w, response)
}

// handleCreateNetwork creates a new I2P network.
//
// This is called when 'docker network create' is used with our driver.
// We'll set up the I2P network infrastructure here.
func (p *Plugin) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.CreateNetwork request")

	var req CreateNetworkRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing CreateNetwork request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Creating network %s", req.NetworkID)

	// Use the network manager to create the network
	if err := p.networkMgr.CreateNetwork(req.NetworkID, req.Options, req.IPv4Data); err != nil {
		log.Printf("Error creating network %s: %v", req.NetworkID, err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Successfully created network %s", req.NetworkID)
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleDeleteNetwork removes an I2P network.
//
// This cleans up I2P tunnels and network resources when the network is deleted.
func (p *Plugin) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.DeleteNetwork request")

	var req DeleteNetworkRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing DeleteNetwork request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Deleting network %s", req.NetworkID)

	// Use the network manager to delete the network
	if err := p.networkMgr.DeleteNetwork(req.NetworkID); err != nil {
		log.Printf("Error deleting network %s: %v", req.NetworkID, err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Successfully deleted network %s", req.NetworkID)
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleCreateEndpoint creates a new endpoint for a container.
//
// This sets up I2P connectivity for a specific container on the network.
func (p *Plugin) handleCreateEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.CreateEndpoint request")

	var req CreateEndpointRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing CreateEndpoint request: %v", err)
		p.writeJSONResponse(w, CreateEndpointResponse{
			ErrorResponse: ErrorResponse{Err: err.Error()},
		})
		return
	}

	log.Printf("Creating endpoint %s on network %s", req.EndpointID, req.NetworkID)

	// Use the network manager to create the endpoint
	endpoint, err := p.networkMgr.CreateEndpoint(req.NetworkID, req.EndpointID, req.Options)
	if err != nil {
		log.Printf("Error creating endpoint %s: %v", req.EndpointID, err)
		p.writeJSONResponse(w, CreateEndpointResponse{
			ErrorResponse: ErrorResponse{Err: err.Error()},
		})
		return
	}

	// Prepare the response with endpoint interface information
	response := CreateEndpointResponse{
		Interface: &EndpointInterface{
			MacAddress: endpoint.MacAddress,
			Address:    endpoint.IPAddress.String() + "/" + "24", // Include CIDR notation
		},
		ErrorResponse: ErrorResponse{Err: ""},
	}

	log.Printf("Successfully created endpoint %s on network %s", req.EndpointID, req.NetworkID)
	p.writeJSONResponse(w, response)
}

// handleDeleteEndpoint removes a container endpoint.
//
// This cleans up I2P resources for a specific container.
func (p *Plugin) handleDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.DeleteEndpoint request")

	var req DeleteEndpointRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing DeleteEndpoint request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Deleting endpoint %s on network %s", req.EndpointID, req.NetworkID)

	// Use the network manager to delete the endpoint
	if err := p.networkMgr.DeleteEndpoint(req.NetworkID, req.EndpointID); err != nil {
		log.Printf("Error deleting endpoint %s: %v", req.EndpointID, err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Successfully deleted endpoint %s on network %s", req.EndpointID, req.NetworkID)
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleEndpointInfo returns information about an endpoint.
//
// This provides Docker with endpoint-specific information.
func (p *Plugin) handleEndpointInfo(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.EndpointOperInfo request")

	var req EndpointInfoRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing EndpointInfo request: %v", err)
		p.writeJSONResponse(w, EndpointInfoResponse{
			ErrorResponse: ErrorResponse{Err: err.Error()},
		})
		return
	}

	log.Printf("Getting info for endpoint %s on network %s", req.EndpointID, req.NetworkID)

	response := EndpointInfoResponse{
		Value:         map[string]interface{}{},
		ErrorResponse: ErrorResponse{Err: ""},
	}

	p.writeJSONResponse(w, response)
}

// handleJoin connects a container to the I2P network.
//
// This is called when a container is started and needs to join the network.
func (p *Plugin) handleJoin(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.Join request")

	var req JoinRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing Join request: %v", err)
		p.writeJSONResponse(w, JoinResponse{
			ErrorResponse: ErrorResponse{Err: err.Error()},
		})
		return
	}

	log.Printf("Joining endpoint %s to network %s (sandbox: %s)", req.EndpointID, req.NetworkID, req.SandboxKey)

	// Extract container ID from sandbox key (Docker format: /var/run/docker/netns/<containerID>)
	containerID := extractContainerID(req.SandboxKey)
	if containerID == "" {
		containerID = req.SandboxKey // Fallback to using sandbox key directly
	}

	// Use the network manager to join the endpoint
	endpoint, err := p.networkMgr.JoinEndpoint(req.NetworkID, req.EndpointID, containerID, req.SandboxKey, req.Options)
	if err != nil {
		log.Printf("Error joining endpoint %s: %v", req.EndpointID, err)
		p.writeJSONResponse(w, JoinResponse{
			ErrorResponse: ErrorResponse{Err: err.Error()},
		})
		return
	}

	// Get the network to retrieve gateway information
	network := p.networkMgr.GetNetwork(req.NetworkID)
	if network == nil {
		log.Printf("Network %s not found during join", req.NetworkID)
		p.writeJSONResponse(w, JoinResponse{
			ErrorResponse: ErrorResponse{Err: "network not found"},
		})
		return
	}

	// Prepare the response with network configuration
	response := JoinResponse{
		InterfaceName: &InterfaceName{
			SrcName:   "veth", // Standard Docker veth interface
			DstPrefix: "eth",  // Standard container interface prefix
		},
		Gateway: network.Gateway.String(),
		// ResolvConf and DNS can be configured for I2P-specific resolution
		ErrorResponse: ErrorResponse{Err: ""},
	}

	log.Printf("Successfully joined endpoint %s to network %s with IP %s",
		req.EndpointID, req.NetworkID, endpoint.IPAddress.String())
	p.writeJSONResponse(w, response)
}

// handleLeave disconnects a container from the I2P network.
//
// This is called when a container is stopped and needs to leave the network.
func (p *Plugin) handleLeave(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.Leave request")

	var req LeaveRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing Leave request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Leaving endpoint %s from network %s", req.EndpointID, req.NetworkID)

	// Use the network manager to leave the endpoint
	err := p.networkMgr.LeaveEndpoint(req.NetworkID, req.EndpointID)
	if err != nil {
		log.Printf("Error leaving endpoint %s: %v", req.EndpointID, err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Successfully left endpoint %s from network %s", req.EndpointID, req.NetworkID)
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleDiscoverNew handles discovery of new nodes.
//
// This is used for multi-host networking, which we don't support for I2P.
func (p *Plugin) handleDiscoverNew(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.DiscoverNew request")

	var req DiscoveryNotification
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing DiscoverNew request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	// I2P networks are local scope, so discovery is not needed
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleDiscoverDelete handles removal of discovered nodes.
//
// This is used for multi-host networking, which we don't support for I2P.
func (p *Plugin) handleDiscoverDelete(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.DiscoverDelete request")

	var req DiscoveryNotification
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing DiscoverDelete request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	// I2P networks are local scope, so discovery is not needed
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleProgramExternalConnectivity sets up external connectivity.
//
// This would typically handle port mapping, but for I2P we handle
// this through I2P tunnels instead.
func (p *Plugin) handleProgramExternalConnectivity(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.ProgramExternalConnectivity request")

	var req ExternalConnectivityRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing ProgramExternalConnectivity request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Programming external connectivity for endpoint %s on network %s", req.EndpointID, req.NetworkID)

	// TODO: Set up I2P server tunnels for exposed ports
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// handleRevokeExternalConnectivity removes external connectivity.
//
// This cleans up I2P server tunnels when ports are no longer exposed.
func (p *Plugin) handleRevokeExternalConnectivity(w http.ResponseWriter, r *http.Request) {
	log.Println("Received NetworkDriver.RevokeExternalConnectivity request")

	var req ExternalConnectivityRequest
	if err := p.readJSONRequest(r, &req); err != nil {
		log.Printf("Error parsing RevokeExternalConnectivity request: %v", err)
		p.writeJSONResponse(w, ErrorResponse{Err: err.Error()})
		return
	}

	log.Printf("Revoking external connectivity for endpoint %s on network %s", req.EndpointID, req.NetworkID)

	// TODO: Clean up I2P server tunnels
	p.writeJSONResponse(w, ErrorResponse{Err: ""})
}

// extractContainerID extracts the container ID from a Docker sandbox key.
//
// Docker sandbox keys are typically in the format:
// /var/run/docker/netns/<containerID>
func extractContainerID(sandboxKey string) string {
	if sandboxKey == "" {
		return ""
	}

	// Split by '/' and get the last segment
	parts := strings.Split(sandboxKey, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return sandboxKey
}
