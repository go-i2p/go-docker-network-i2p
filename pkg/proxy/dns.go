package proxy

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// I2PDNSResolver provides DNS resolution for I2P destinations.
//
// The resolver handles .i2p domain queries and provides appropriate responses
// while blocking non-I2P domains to prevent DNS leaks.
type I2PDNSResolver struct {
	// listenAddr is the address where the DNS resolver listens
	listenAddr string
	// server is the DNS server instance
	server *dns.Server
	// ctx is the context for resolver operation
	ctx context.Context
	// cancel cancels the resolver context
	cancel context.CancelFunc
}

// NewI2PDNSResolver creates a new DNS resolver for I2P destinations.
//
// The resolver will listen on the specified address and provide DNS responses
// for I2P destinations while blocking all other queries.
func NewI2PDNSResolver(listenAddr string) *I2PDNSResolver {
	ctx, cancel := context.WithCancel(context.Background())

	return &I2PDNSResolver{
		listenAddr: listenAddr,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the DNS resolver service.
//
// This method blocks until the resolver is stopped or an error occurs.
// It should be run in a goroutine for non-blocking operation.
func (r *I2PDNSResolver) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", r.handleDNSQuery)

	r.server = &dns.Server{
		Addr:    r.listenAddr,
		Net:     "udp",
		Handler: mux,
		// Enable timeout to allow graceful shutdown
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Also handle TCP queries
	tcpServer := &dns.Server{
		Addr:         r.listenAddr,
		Net:          "tcp",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Start TCP server in background
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			// Log error but don't fail UDP server
		}
	}()

	return r.server.ListenAndServe()
}

// Stop gracefully shuts down the DNS resolver.
func (r *I2PDNSResolver) Stop() error {
	r.cancel()

	if r.server != nil {
		return r.server.Shutdown()
	}

	return nil
}

// handleDNSQuery processes DNS queries and provides I2P-specific responses.
//
// This method implements the core DNS resolution logic for I2P domains.
func (r *I2PDNSResolver) handleDNSQuery(w dns.ResponseWriter, req *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = true

	// Process each question in the query
	for _, question := range req.Question {
		if answer := r.resolveQuestion(question); answer != nil {
			msg.Answer = append(msg.Answer, answer)
		} else {
			// Return NXDOMAIN for non-I2P queries
			msg.Rcode = dns.RcodeNameError
		}
	}

	w.WriteMsg(msg)
}

// resolveQuestion resolves a single DNS question.
//
// Returns a DNS resource record if the question can be answered, nil otherwise.
func (r *I2PDNSResolver) resolveQuestion(question dns.Question) dns.RR {
	name := strings.ToLower(question.Name)

	// Remove trailing dot if present
	if strings.HasSuffix(name, ".") {
		name = name[:len(name)-1]
	}

	// Only handle I2P domains
	if !r.isI2PDomain(name) {
		return nil
	}

	switch question.Qtype {
	case dns.TypeA:
		return r.resolveA(name, question.Name)
	case dns.TypeAAAA:
		// Return empty response for IPv6 (I2P doesn't use IPv6 addresses)
		return nil
	case dns.TypeCNAME:
		return r.resolveCNAME(name, question.Name)
	default:
		// Unsupported query type
		return nil
	}
}

// isI2PDomain checks if a domain is an I2P domain.
//
// I2P domains include .i2p domains and base32 addresses.
func (r *I2PDNSResolver) isI2PDomain(domain string) bool {
	// Check for .i2p domain
	if strings.HasSuffix(domain, ".i2p") {
		return true
	}

	// Check for .b32.i2p domain (base32 encoded address)
	if strings.HasSuffix(domain, ".b32.i2p") {
		return true
	}

	return false
}

// resolveA creates an A record response for I2P domains.
//
// I2P domains are resolved to a special IP address that will be intercepted
// by the traffic interception rules and routed through the SOCKS proxy.
func (r *I2PDNSResolver) resolveA(domain, originalName string) dns.RR {
	// Use a special IP range for I2P domains that will be intercepted
	// We use 198.18.0.0/15 which is reserved for benchmarking (RFC 2544)
	// and is unlikely to conflict with real networks

	// Generate a consistent IP based on domain hash to ensure
	// the same domain always gets the same IP
	ip := r.generateI2PIP(domain)

	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   originalName,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300, // 5 minutes TTL
		},
		A: ip,
	}
}

// resolveCNAME handles CNAME queries for I2P domains.
//
// This is mainly for handling subdomain redirects within I2P.
func (r *I2PDNSResolver) resolveCNAME(domain, originalName string) dns.RR {
	// For now, don't handle CNAME records
	// Could be extended to handle I2P domain aliases
	return nil
}

// generateI2PIP generates a consistent IP address for an I2P domain.
//
// This ensures the same I2P domain always resolves to the same IP address,
// which is important for application caching and connection reuse.
func (r *I2PDNSResolver) generateI2PIP(domain string) net.IP {
	// Use a simple hash-based approach to generate consistent IPs
	// in the 198.18.0.0/15 range

	hash := r.simpleHash(domain)

	// Map hash to 198.18.0.0/15 range (32,768 addresses)
	// 198.18.0.0 = 0xC6120000
	baseIP := uint32(0xC6120000)
	offset := hash % 32768 // 32,768 addresses in /15

	ip := baseIP + offset

	return net.IPv4(
		byte((ip>>24)&0xFF),
		byte((ip>>16)&0xFF),
		byte((ip>>8)&0xFF),
		byte(ip&0xFF),
	)
}

// simpleHash computes a simple hash of a string.
//
// This is used to generate consistent IP addresses for I2P domains.
func (r *I2PDNSResolver) simpleHash(s string) uint32 {
	var hash uint32 = 5381

	for _, c := range s {
		hash = ((hash << 5) + hash) + uint32(c)
	}

	return hash
}
