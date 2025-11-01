// Package plugin defines the types used in Docker Plugin API communication.
//
// These types represent the request and response structures defined in
// Docker's Plugin API v2 specification for network drivers.
package plugin

// ActivateResponse represents the response to Plugin.Activate.
type ActivateResponse struct {
	Implements []string `json:"Implements"`
}

// ErrorResponse represents a standard error response.
type ErrorResponse struct {
	Err string `json:"Err"`
}

// CapabilitiesResponse represents the response to NetworkDriver.GetCapabilities.
type CapabilitiesResponse struct {
	Scope             string `json:"Scope"`
	ConnectivityScope string `json:"ConnectivityScope"`
	ErrorResponse
}

// CreateNetworkRequest represents a request to create a network.
type CreateNetworkRequest struct {
	NetworkID string                 `json:"NetworkID"`
	Options   map[string]interface{} `json:"Options"`
	IPv4Data  []IPAMData             `json:"IPv4Data,omitempty"`
	IPv6Data  []IPAMData             `json:"IPv6Data,omitempty"`
}

// IPAMData represents IP address management data.
type IPAMData struct {
	AddressSpace string            `json:"AddressSpace,omitempty"`
	Pool         string            `json:"Pool,omitempty"`
	Gateway      string            `json:"Gateway,omitempty"`
	AuxAddresses map[string]string `json:"AuxAddresses,omitempty"`
}

// DeleteNetworkRequest represents a request to delete a network.
type DeleteNetworkRequest struct {
	NetworkID string `json:"NetworkID"`
}

// CreateEndpointRequest represents a request to create an endpoint.
type CreateEndpointRequest struct {
	NetworkID  string                 `json:"NetworkID"`
	EndpointID string                 `json:"EndpointID"`
	Interface  *EndpointInterface     `json:"Interface,omitempty"`
	Options    map[string]interface{} `json:"Options,omitempty"`
}

// EndpointInterface represents network interface configuration.
type EndpointInterface struct {
	Address     string `json:"Address,omitempty"`
	AddressIPv6 string `json:"AddressIPv6,omitempty"`
	MacAddress  string `json:"MacAddress,omitempty"`
}

// CreateEndpointResponse represents the response to creating an endpoint.
type CreateEndpointResponse struct {
	Interface *EndpointInterface `json:"Interface,omitempty"`
	ErrorResponse
}

// DeleteEndpointRequest represents a request to delete an endpoint.
type DeleteEndpointRequest struct {
	NetworkID  string `json:"NetworkID"`
	EndpointID string `json:"EndpointID"`
}

// EndpointInfoRequest represents a request for endpoint information.
type EndpointInfoRequest struct {
	NetworkID  string `json:"NetworkID"`
	EndpointID string `json:"EndpointID"`
}

// EndpointInfoResponse represents the response with endpoint information.
type EndpointInfoResponse struct {
	Value map[string]interface{} `json:"Value"`
	ErrorResponse
}

// JoinRequest represents a request to join a container to a network.
type JoinRequest struct {
	NetworkID  string                 `json:"NetworkID"`
	EndpointID string                 `json:"EndpointID"`
	SandboxKey string                 `json:"SandboxKey"`
	Options    map[string]interface{} `json:"Options,omitempty"`
}

// InterfaceName represents the interface naming configuration.
type InterfaceName struct {
	SrcName   string `json:"SrcName"`
	DstPrefix string `json:"DstPrefix"`
}

// JoinResponse represents the response to a join request.
type JoinResponse struct {
	InterfaceName         *InterfaceName         `json:"InterfaceName,omitempty"`
	Gateway               string                 `json:"Gateway,omitempty"`
	GatewayIPv6           string                 `json:"GatewayIPv6,omitempty"`
	StaticRoutes          []StaticRoute          `json:"StaticRoutes,omitempty"`
	DisableGatewayService bool                   `json:"DisableGatewayService,omitempty"`
	GWName                string                 `json:"GWName,omitempty"`
	ResolvConf            string                 `json:"ResolvConf,omitempty"`
	DNS                   []string               `json:"DNS,omitempty"`
	Options               map[string]interface{} `json:"Options,omitempty"`
	ErrorResponse
}

// StaticRoute represents a static route configuration.
type StaticRoute struct {
	Destination string `json:"Destination"`
	RouteType   int    `json:"RouteType"`
	NextHop     string `json:"NextHop"`
}

// LeaveRequest represents a request to leave a network.
type LeaveRequest struct {
	NetworkID  string `json:"NetworkID"`
	EndpointID string `json:"EndpointID"`
}

// DiscoveryNotification represents discovery notifications.
type DiscoveryNotification struct {
	DiscoveryType int                    `json:"DiscoveryType"`
	DiscoveryData map[string]interface{} `json:"DiscoveryData"`
}

// ExternalConnectivityRequest represents external connectivity operations.
type ExternalConnectivityRequest struct {
	NetworkID  string                 `json:"NetworkID"`
	EndpointID string                 `json:"EndpointID"`
	Options    map[string]interface{} `json:"Options"`
}
