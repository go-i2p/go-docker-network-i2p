package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
)

// SOCKSProxy implements a SOCKS5 proxy that routes traffic through I2P tunnels.
//
// The proxy accepts SOCKS5 connections from containers and establishes
// corresponding I2P client tunnels for outbound connectivity.
type SOCKSProxy struct {
	// listenAddr is the address where the SOCKS proxy listens
	listenAddr string
	// tunnelManager manages I2P tunnels for proxy connections
	tunnelManager *i2p.TunnelManager
	// listener is the TCP listener for SOCKS connections
	listener net.Listener
	// ctx is the context for proxy operation
	ctx context.Context
	// cancel cancels the proxy context
	cancel context.CancelFunc
}

// NewSOCKSProxy creates a new SOCKS5 proxy that routes traffic through I2P.
//
// The proxy will listen on the specified address and use the tunnel manager
// to create I2P client tunnels for outbound connections.
func NewSOCKSProxy(listenAddr string, tunnelManager *i2p.TunnelManager) *SOCKSProxy {
	ctx, cancel := context.WithCancel(context.Background())

	return &SOCKSProxy{
		listenAddr:    listenAddr,
		tunnelManager: tunnelManager,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins accepting SOCKS5 connections and processing them.
//
// This method blocks until the proxy is stopped or an error occurs.
// It should be run in a goroutine for non-blocking operation.
func (s *SOCKSProxy) Start() error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.listenAddr, err)
	}

	s.listener = listener

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		// Set accept timeout to allow checking context cancellation
		listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))

		conn, err := listener.Accept()
		if err != nil {
			// Check if this is a timeout (normal during shutdown)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		// Handle connection in goroutine
		go s.handleConnection(conn)
	}
}

// Stop gracefully shuts down the SOCKS proxy.
//
// This method closes the listener and cancels all active connections.
func (s *SOCKSProxy) Stop() error {
	s.cancel()

	if s.listener != nil {
		return s.listener.Close()
	}

	return nil
}

// handleConnection processes a single SOCKS5 connection.
//
// This method implements the SOCKS5 protocol handshake and establishes
// the I2P tunnel for the requested destination.
func (s *SOCKSProxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set connection timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// SOCKS5 handshake
	if err := s.performSOCKS5Handshake(conn); err != nil {
		return
	}

	// Parse SOCKS5 request
	target, err := s.parseSOCKS5Request(conn)
	if err != nil {
		s.sendSOCKS5Error(conn, 0x01) // General SOCKS server failure
		return
	}

	// Check if target is an I2P destination
	if !s.isI2PDestination(target) {
		s.sendSOCKS5Error(conn, 0x02) // Connection not allowed by ruleset
		return
	}

	// Establish I2P connection
	i2pConn, err := s.connectToI2P(target)
	if err != nil {
		s.sendSOCKS5Error(conn, 0x04) // Host unreachable
		return
	}
	defer i2pConn.Close()

	// Send success response
	if err := s.sendSOCKS5Success(conn); err != nil {
		return
	}

	// Relay traffic between SOCKS client and I2P connection
	s.relayTraffic(conn, i2pConn)
}

// performSOCKS5Handshake handles the SOCKS5 authentication handshake.
//
// This implementation only supports "no authentication" method.
func (s *SOCKSProxy) performSOCKS5Handshake(conn net.Conn) error {
	// Read client greeting
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n < 3 {
		return fmt.Errorf("failed to read SOCKS5 greeting")
	}

	// Check SOCKS version
	if buf[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	// Check for "no authentication" method
	nMethods := int(buf[1])
	if nMethods == 0 || n < 2+nMethods {
		return fmt.Errorf("invalid SOCKS5 greeting")
	}

	supportsNoAuth := false
	for i := 0; i < nMethods; i++ {
		if buf[2+i] == 0x00 { // No authentication
			supportsNoAuth = true
			break
		}
	}

	if !supportsNoAuth {
		// Send "no acceptable methods"
		conn.Write([]byte{0x05, 0xFF})
		return fmt.Errorf("client does not support no-auth method")
	}

	// Send "no authentication" response
	_, err = conn.Write([]byte{0x05, 0x00})
	return err
}

// parseSOCKS5Request parses the SOCKS5 connection request.
//
// Returns the target address in "host:port" format.
func (s *SOCKSProxy) parseSOCKS5Request(conn net.Conn) (string, error) {
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n < 7 {
		return "", fmt.Errorf("failed to read SOCKS5 request")
	}

	// Check SOCKS version and command
	if buf[0] != 0x05 {
		return "", fmt.Errorf("invalid SOCKS version")
	}
	if buf[1] != 0x01 { // CONNECT command
		return "", fmt.Errorf("unsupported SOCKS command: %d", buf[1])
	}

	// Parse address
	addrType := buf[3]
	var host string
	var port uint16

	switch addrType {
	case 0x01: // IPv4
		if n < 10 {
			return "", fmt.Errorf("invalid IPv4 address length")
		}
		host = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
		port = uint16(buf[8])<<8 | uint16(buf[9])

	case 0x03: // Domain name
		if n < 5 {
			return "", fmt.Errorf("invalid domain name length")
		}
		domainLen := int(buf[4])
		if n < 7+domainLen {
			return "", fmt.Errorf("incomplete domain name")
		}
		host = string(buf[5 : 5+domainLen])
		port = uint16(buf[5+domainLen])<<8 | uint16(buf[6+domainLen])

	case 0x04: // IPv6
		return "", fmt.Errorf("IPv6 not supported")

	default:
		return "", fmt.Errorf("unsupported address type: %d", addrType)
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}

// isI2PDestination checks if the target address is an I2P destination.
//
// I2P destinations end with .i2p or are base32/base64 encoded addresses.
func (s *SOCKSProxy) isI2PDestination(target string) bool {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return false
	}

	// Check for .i2p domain
	if strings.HasSuffix(host, ".i2p") {
		return true
	}

	// Check for base32 I2P address (52 characters ending in .b32.i2p)
	if strings.HasSuffix(host, ".b32.i2p") && len(host) == 60 {
		return true
	}

	// Could add more I2P destination format checks here
	return false
}

// connectToI2P establishes a connection to an I2P destination.
//
// This method creates an I2P client tunnel and connects to the target.
func (s *SOCKSProxy) connectToI2P(target string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target format: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// Create I2P client tunnel configuration
	tunnelConfig := &i2p.TunnelConfig{
		Name:        fmt.Sprintf("client-%s-%d", host, port),
		ContainerID: "proxy-session",
		Type:        i2p.TunnelTypeClient,
		LocalHost:   "127.0.0.1",
		LocalPort:   0, // Let system assign port
		Destination: host,
		Options:     i2p.DefaultTunnelOptions(),
	}

	// Create tunnel
	tunnel, err := s.tunnelManager.CreateTunnel(tunnelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create I2P tunnel: %w", err)
	}

	// Connect through the tunnel
	tunnelAddr := tunnel.GetLocalEndpoint()
	conn, err := net.DialTimeout("tcp", tunnelAddr, 30*time.Second)
	if err != nil {
		s.tunnelManager.DestroyTunnel(tunnel.GetConfig().Name)
		return nil, fmt.Errorf("failed to connect through I2P tunnel: %w", err)
	}

	return conn, nil
}

// sendSOCKS5Error sends a SOCKS5 error response.
func (s *SOCKSProxy) sendSOCKS5Error(conn net.Conn, errorCode byte) {
	response := []byte{0x05, errorCode, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	conn.Write(response)
}

// sendSOCKS5Success sends a SOCKS5 success response.
func (s *SOCKSProxy) sendSOCKS5Success(conn net.Conn) error {
	response := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := conn.Write(response)
	return err
}

// relayTraffic copies data between the SOCKS client and I2P connection.
func (s *SOCKSProxy) relayTraffic(client, i2p net.Conn) {
	done := make(chan struct{}, 2)

	// Copy from client to I2P
	go func() {
		defer func() { done <- struct{}{} }()
		io.Copy(i2p, client)
	}()

	// Copy from I2P to client
	go func() {
		defer func() { done <- struct{}{} }()
		io.Copy(client, i2p)
	}()

	// Wait for one direction to complete
	<-done
}
