package proxy

import (
	"net"
	"testing"

	"github.com/go-i2p/go-docker-network-i2p/pkg/i2p"
	"github.com/miekg/dns"
)

func TestNewTrafficInterceptor(t *testing.T) {
	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	interceptor := NewTrafficInterceptor(subnet, 1080, 53)

	if interceptor.containerSubnet.String() != subnet.String() {
		t.Errorf("Expected subnet %s, got %s", subnet.String(), interceptor.containerSubnet.String())
	}

	if interceptor.proxyPort != 1080 {
		t.Errorf("Expected proxy port 1080, got %d", interceptor.proxyPort)
	}

	if interceptor.dnsPort != 53 {
		t.Errorf("Expected DNS port 53, got %d", interceptor.dnsPort)
	}
}

func TestTrafficInterceptor_generateIptablesRules(t *testing.T) {
	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	interceptor := NewTrafficInterceptor(subnet, 1080, 53)
	rules := interceptor.generateIptablesRules()

	// Check that we have the expected number of rules
	if len(rules) == 0 {
		t.Error("Expected iptables rules to be generated")
	}

	// Check for custom chain creation
	foundRedirectChain := false
	foundFilterChain := false
	for _, rule := range rules {
		if rule == "-t nat -N I2P_REDIRECT" {
			foundRedirectChain = true
		}
		if rule == "-t filter -N I2P_FILTER" {
			foundFilterChain = true
		}
	}

	if !foundRedirectChain {
		t.Error("Expected I2P_REDIRECT chain creation rule")
	}

	if !foundFilterChain {
		t.Error("Expected I2P_FILTER chain creation rule")
	}
}

func TestNewSOCKSProxy(t *testing.T) {
	// Create a mock tunnel manager (simplified for testing)
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	proxy := NewSOCKSProxy("127.0.0.1:1080", tunnelMgr)

	if proxy.listenAddr != "127.0.0.1:1080" {
		t.Errorf("Expected listen address 127.0.0.1:1080, got %s", proxy.listenAddr)
	}

	if proxy.tunnelManager != tunnelMgr {
		t.Error("Expected tunnel manager to be set correctly")
	}
}

func TestSOCKSProxy_isI2PDestination(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	proxy := NewSOCKSProxy("127.0.0.1:1080", tunnelMgr)

	tests := []struct {
		name     string
		target   string
		expected bool
	}{
		{
			name:     "valid I2P domain",
			target:   "example.i2p:80",
			expected: true,
		},
		{
			name:     "valid base32 I2P address",
			target:   "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqr.b32.i2p:80",
			expected: true,
		},
		{
			name:     "invalid regular domain",
			target:   "example.com:80",
			expected: false,
		},
		{
			name:     "invalid IP address",
			target:   "192.168.1.1:80",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proxy.isI2PDestination(tt.target)
			if result != tt.expected {
				t.Errorf("isI2PDestination(%s) = %v, expected %v", tt.target, result, tt.expected)
			}
		})
	}
}

func TestNewI2PDNSResolver(t *testing.T) {
	resolver := NewI2PDNSResolver("127.0.0.1:5353")

	if resolver.listenAddr != "127.0.0.1:5353" {
		t.Errorf("Expected listen address 127.0.0.1:5353, got %s", resolver.listenAddr)
	}
}

func TestI2PDNSResolver_isI2PDomain(t *testing.T) {
	resolver := NewI2PDNSResolver("127.0.0.1:5353")

	tests := []struct {
		name     string
		domain   string
		expected bool
	}{
		{
			name:     "valid I2P domain",
			domain:   "example.i2p",
			expected: true,
		},
		{
			name:     "valid base32 I2P domain",
			domain:   "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqr.b32.i2p",
			expected: true,
		},
		{
			name:     "invalid regular domain",
			domain:   "example.com",
			expected: false,
		},
		{
			name:     "invalid subdomain",
			domain:   "sub.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.isI2PDomain(tt.domain)
			if result != tt.expected {
				t.Errorf("isI2PDomain(%s) = %v, expected %v", tt.domain, result, tt.expected)
			}
		})
	}
}

func TestI2PDNSResolver_generateI2PIP(t *testing.T) {
	resolver := NewI2PDNSResolver("127.0.0.1:5353")

	// Test that the same domain always generates the same IP
	domain := "example.i2p"
	ip1 := resolver.generateI2PIP(domain)
	ip2 := resolver.generateI2PIP(domain)

	if !ip1.Equal(ip2) {
		t.Errorf("Expected same IP for same domain, got %v and %v", ip1, ip2)
	}

	// Test that different domains generate different IPs
	ip3 := resolver.generateI2PIP("different.i2p")
	if ip1.Equal(ip3) {
		t.Errorf("Expected different IPs for different domains, got %v for both", ip1)
	}

	// Test that generated IP is in the expected range (198.18.0.0/15)
	// 198.18.0.0/15 covers 198.18.0.0 to 198.19.255.255
	if ip1.To4() == nil {
		t.Errorf("Generated IP %v is not IPv4", ip1)
		return
	}
	ipBytes := ip1.To4()
	if ipBytes[0] != 198 || ipBytes[1] < 18 || ipBytes[1] > 19 {
		t.Errorf("Generated IP %v is not in expected range 198.18.0.0/15", ip1)
	}
}

func TestDefaultProxyConfig(t *testing.T) {
	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	config := DefaultProxyConfig(subnet)

	if config.ContainerSubnet.String() != subnet.String() {
		t.Errorf("Expected subnet %s, got %s", subnet.String(), config.ContainerSubnet.String())
	}

	if config.SOCKSPort != 1080 {
		t.Errorf("Expected SOCKS port 1080, got %d", config.SOCKSPort)
	}

	if config.DNSPort != 53 {
		t.Errorf("Expected DNS port 53, got %d", config.DNSPort)
	}

	if config.SOCKSBindAddr != "127.0.0.1:1080" {
		t.Errorf("Expected SOCKS bind address 127.0.0.1:1080, got %s", config.SOCKSBindAddr)
	}

	if config.DNSBindAddr != "127.0.0.1:53" {
		t.Errorf("Expected DNS bind address 127.0.0.1:53, got %s", config.DNSBindAddr)
	}
}

func TestNewProxyManager(t *testing.T) {
	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	config := DefaultProxyConfig(subnet)

	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager := NewProxyManager(config, tunnelMgr)

	if manager.config != config {
		t.Error("Expected config to be set correctly")
	}

	if manager.tunnelManager != tunnelMgr {
		t.Error("Expected tunnel manager to be set correctly")
	}

	if !manager.IsRunning() {
		t.Error("Expected new proxy manager to be in running state")
	}
}

/*func TestProxyManager_Lifecycle(t *testing.T) {
	// This test requires root privileges for iptables, so we'll test the basic lifecycle
	// and expect iptables operations to fail gracefully

	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	config := DefaultProxyConfig(subnet)
	config.SOCKSBindAddr = "127.0.0.1:10801" // Use non-privileged port
	config.DNSBindAddr = "127.0.0.1:5353"    // Use non-privileged port

	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager := NewProxyManager(config, tunnelMgr)

	// Test start (will fail due to iptables, but that's expected)
	err = manager.Start()
	if err == nil {
		t.Error("Expected start to fail without root privileges")
	}

	// Test stop
	err = manager.Stop()
	if err != nil {
		t.Errorf("Expected stop to succeed, got error: %v", err)
	}

	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)

	if manager.IsRunning() {
		t.Error("Expected proxy manager to be stopped")
	}
}*/

func TestI2PDNSResolver_resolveQuestion(t *testing.T) {
	resolver := NewI2PDNSResolver("127.0.0.1:5353")

	// Test I2P domain resolution
	question := dns.Question{
		Name:   "example.i2p.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}

	answer := resolver.resolveQuestion(question)
	if answer == nil {
		t.Error("Expected answer for I2P domain, got nil")
	}

	if aRecord, ok := answer.(*dns.A); ok {
		if aRecord.A == nil {
			t.Error("Expected A record to have an IP address")
		}
	} else {
		t.Error("Expected answer to be an A record")
	}
}

func TestI2PDNSResolver_resolveQuestion_NonI2P(t *testing.T) {
	resolver := NewI2PDNSResolver("127.0.0.1:5353")

	// Test non-I2P domain resolution
	question := dns.Question{
		Name:   "example.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}

	answer := resolver.resolveQuestion(question)
	if answer != nil {
		t.Error("Expected nil answer for non-I2P domain")
	}
}

func TestTrafficInterceptor_iptablesIntegration(t *testing.T) {
	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	interceptor := NewTrafficInterceptor(subnet, 1080, 53)

	// Test that rules are generated correctly
	rules := interceptor.generateIptablesRules()
	if len(rules) == 0 {
		t.Error("Expected iptables rules to be generated")
	}

	// Test validation of private subnet
	if !subnet.IP.IsPrivate() {
		t.Error("Test subnet should be private")
	}
}

// Benchmark tests for performance
func BenchmarkI2PDNSResolver_generateI2PIP(b *testing.B) {
	resolver := NewI2PDNSResolver("127.0.0.1:5353")
	domain := "example.i2p"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.generateI2PIP(domain)
	}
}

func TestSOCKSProxy_TrafficFilterIntegration(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	proxy := NewSOCKSProxy("127.0.0.1:1080", tunnelMgr)

	// Test traffic filter initialization
	filter := proxy.GetTrafficFilter()
	if filter == nil {
		t.Error("Expected traffic filter to be initialized")
	}

	// Test allowlist functionality
	err = filter.AddToAllowlist("example.i2p")
	if err != nil {
		t.Errorf("Failed to add to allowlist: %v", err)
	}

	allowed, _ := filter.ShouldAllowConnection("example.i2p:80", "tcp")
	if !allowed {
		t.Error("Expected example.i2p to be allowed")
	}

	// Test blocklist functionality
	err = filter.AddToBlocklist("blocked.i2p")
	if err != nil {
		t.Errorf("Failed to add to blocklist: %v", err)
	}

	blocked, _ := filter.ShouldAllowConnection("blocked.i2p:80", "tcp")
	if blocked {
		t.Error("Expected blocked.i2p to be blocked")
	}

	// Test statistics
	stats := filter.GetStats()
	if stats.I2PConnectionsAllowed < 0 {
		t.Error("Expected valid traffic stats for new filter")
	}
}

func TestProxyManager_TrafficFilterIntegration(t *testing.T) {
	_, subnet, err := net.ParseCIDR("172.20.0.0/16")
	if err != nil {
		t.Fatalf("Failed to parse test subnet: %v", err)
	}

	config := DefaultProxyConfig(subnet)

	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	manager := NewProxyManager(config, tunnelMgr)

	// Test traffic filter initialization
	filter := manager.GetTrafficFilter()
	if filter == nil {
		t.Error("Expected traffic filter to be initialized")
	}

	// Test allowlist management
	err = manager.AddToAllowlist("test.i2p")
	if err != nil {
		t.Errorf("Failed to add to allowlist via manager: %v", err)
	}

	allowlist := manager.GetAllowlist()
	found := false
	for _, domain := range allowlist {
		if domain == "test.i2p" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test.i2p to be in allowlist")
	}

	// Test blocklist management
	err = manager.AddToBlocklist("bad.i2p")
	if err != nil {
		t.Errorf("Failed to add to blocklist via manager: %v", err)
	}

	blocklist := manager.GetBlocklist()
	found = false
	for _, domain := range blocklist {
		if domain == "bad.i2p" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected bad.i2p to be in blocklist")
	}

	// Test statistics access
	stats := manager.GetTrafficStats()
	if stats.I2PConnectionsAllowed < 0 {
		t.Error("Expected valid traffic stats via manager")
	}

	// Test removal functionality
	manager.RemoveFromAllowlist("test.i2p")
	manager.RemoveFromBlocklist("bad.i2p")

	// Verify removal
	allowlist = manager.GetAllowlist()
	for _, domain := range allowlist {
		if domain == "test.i2p" {
			t.Error("Expected test.i2p to be removed from allowlist")
		}
	}
}

func TestSOCKSProxy_TrafficFilterValidation(t *testing.T) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		t.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	proxy := NewSOCKSProxy("127.0.0.1:1080", tunnelMgr)
	filter := proxy.GetTrafficFilter()

	// Test wildcard patterns
	err = filter.AddToAllowlist("*.example.i2p")
	if err != nil {
		t.Errorf("Failed to add wildcard to allowlist: %v", err)
	}

	// Should allow subdomains
	allowed, _ := filter.ShouldAllowConnection("sub.example.i2p:80", "tcp")
	if !allowed {
		t.Error("Expected wildcard to allow subdomains")
	}

	// Test case sensitivity
	err = filter.AddToBlocklist("CASE.I2P")
	if err != nil {
		t.Errorf("Failed to add uppercase domain to blocklist: %v", err)
	}

	// Should block regardless of case
	blocked, _ := filter.ShouldAllowConnection("case.i2p:80", "tcp")
	if blocked {
		t.Error("Expected case-insensitive blocking")
	}

	// Test filter priority (blocklist overrides allowlist)
	err = filter.AddToAllowlist("conflict.i2p")
	if err != nil {
		t.Errorf("Failed to add conflict domain to allowlist: %v", err)
	}

	err = filter.AddToBlocklist("conflict.i2p")
	if err != nil {
		t.Errorf("Failed to add conflict domain to blocklist: %v", err)
	}

	// Blocklist should take precedence
	conflictAllowed, _ := filter.ShouldAllowConnection("conflict.i2p:80", "tcp")
	if conflictAllowed {
		t.Error("Expected blocklist to override allowlist")
	}
}

func BenchmarkSOCKSProxy_isI2PDestination(b *testing.B) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		b.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	proxy := NewSOCKSProxy("127.0.0.1:1080", tunnelMgr)
	target := "example.i2p:80"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proxy.isI2PDestination(target)
	}
}

func BenchmarkTrafficFilter_ShouldAllowConnection(b *testing.B) {
	samClient, err := i2p.NewSAMClient(i2p.DefaultSAMConfig())
	if err != nil {
		b.Fatalf("Failed to create SAM client: %v", err)
	}

	tunnelMgr := i2p.NewTunnelManager(samClient)
	proxy := NewSOCKSProxy("127.0.0.1:1080", tunnelMgr)
	filter := proxy.GetTrafficFilter()

	// Add some test patterns
	filter.AddToAllowlist("*.allowed.i2p")
	filter.AddToBlocklist("blocked.i2p")

	target := "test.allowed.i2p:80"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.ShouldAllowConnection(target, "tcp")
	}
}
