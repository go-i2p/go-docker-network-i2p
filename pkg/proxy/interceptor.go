// Package proxy implements transparent I2P traffic proxying for Docker containers.package proxy

// This package provides traffic interception, SOCKS proxying, and DNS resolution
// to route container traffic through I2P while maintaining transparency to applications.
package proxy

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// TrafficInterceptor manages iptables rules for transparent traffic interception.
//
// The interceptor sets up iptables rules to redirect container traffic to the
// I2P proxy, ensuring all traffic flows through I2P tunnels.
type TrafficInterceptor struct {
	// containerSubnet is the subnet used by I2P containers
	containerSubnet *net.IPNet
	// proxyPort is the port where the SOCKS proxy listens
	proxyPort int
	// dnsPort is the port where the DNS resolver listens
	dnsPort int
}

// NewTrafficInterceptor creates a new traffic interceptor for the given subnet.
//
// The interceptor will set up iptables rules to redirect traffic from containers
// in the specified subnet to the I2P proxy services.
func NewTrafficInterceptor(subnet *net.IPNet, proxyPort, dnsPort int) *TrafficInterceptor {
	return &TrafficInterceptor{
		containerSubnet: subnet,
		proxyPort:       proxyPort,
		dnsPort:         dnsPort,
	}
}

// SetupInterception configures iptables rules for transparent traffic proxying.
//
// This method sets up the necessary iptables rules to:
// 1. Redirect TCP traffic to the SOCKS proxy
// 2. Redirect DNS traffic to the I2P DNS resolver
// 3. Drop non-I2P traffic to prevent leaks
func (t *TrafficInterceptor) SetupInterception() error {
	rules := t.generateIptablesRules()

	for _, rule := range rules {
		if err := t.executeIptablesRule(rule); err != nil {
			// If rule fails, clean up any previously added rules
			t.CleanupInterception()
			return fmt.Errorf("failed to add iptables rule '%s': %w", rule, err)
		}
	}

	return nil
}

// CleanupInterception removes all iptables rules created for I2P traffic interception.
//
// This method should be called when the network is being torn down to ensure
// no stale iptables rules remain.
func (t *TrafficInterceptor) CleanupInterception() error {
	rules := t.generateIptablesRules()

	var errors []string

	// Remove rules in reverse order (LIFO)
	for i := len(rules) - 1; i >= 0; i-- {
		deleteRule := strings.Replace(rules[i], "-A", "-D", 1)
		if err := t.executeIptablesRule(deleteRule); err != nil {
			// Log error but continue with cleanup
			errors = append(errors, fmt.Sprintf("failed to remove rule '%s': %v", deleteRule, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// generateIptablesRules creates the iptables rules needed for I2P traffic interception.
//
// The rules implement:
// 1. DNS redirection to I2P DNS resolver
// 2. TCP traffic redirection to SOCKS proxy
// 3. Traffic dropping for non-I2P destinations
func (t *TrafficInterceptor) generateIptablesRules() []string {
	subnet := t.containerSubnet.String()

	return []string{
		// Create custom chain for I2P traffic processing
		"-t nat -N I2P_REDIRECT",

		// Redirect DNS traffic (port 53) to I2P DNS resolver
		fmt.Sprintf("-t nat -A I2P_REDIRECT -s %s -p udp --dport 53 -j REDIRECT --to-port %d",
			subnet, t.dnsPort),
		fmt.Sprintf("-t nat -A I2P_REDIRECT -s %s -p tcp --dport 53 -j REDIRECT --to-port %d",
			subnet, t.dnsPort),

		// Redirect TCP traffic to SOCKS proxy (excluding proxy port itself)
		fmt.Sprintf("-t nat -A I2P_REDIRECT -s %s -p tcp ! --dport %d -j REDIRECT --to-port %d",
			subnet, t.proxyPort, t.proxyPort),

		// Apply I2P_REDIRECT chain to OUTPUT traffic from containers
		fmt.Sprintf("-t nat -A OUTPUT -s %s -j I2P_REDIRECT", subnet),

		// Create custom chain for traffic filtering
		"-t filter -N I2P_FILTER",

		// Allow traffic to I2P proxy and DNS resolver
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -p tcp --dport %d -j ACCEPT",
			subnet, t.proxyPort),
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -p udp --dport %d -j ACCEPT",
			subnet, t.dnsPort),
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -p tcp --dport %d -j ACCEPT",
			subnet, t.dnsPort),

		// Allow loopback traffic
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -d 127.0.0.0/8 -j ACCEPT", subnet),

		// Allow traffic within container subnet
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -d %s -j ACCEPT", subnet, subnet),

		// Log and drop all other traffic to prevent leaks
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -j LOG --log-prefix \"I2P-DROP: \"", subnet),
		fmt.Sprintf("-t filter -A I2P_FILTER -s %s -j DROP", subnet),

		// Apply I2P_FILTER chain to OUTPUT traffic from containers
		fmt.Sprintf("-t filter -A OUTPUT -s %s -j I2P_FILTER", subnet),
	}
}

// executeIptablesRule executes a single iptables rule using the iptables command.
//
// This method handles the actual execution of iptables commands and provides
// error handling for common iptables failures.
func (t *TrafficInterceptor) executeIptablesRule(rule string) error {
	args := strings.Fields(rule)
	cmd := exec.Command("iptables", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables command failed: %s (output: %s)", err, string(output))
	}

	return nil
}

// IsAvailable checks if iptables is available and the current process has
// sufficient privileges to modify iptables rules.
//
// This method should be called before attempting to set up traffic interception
// to ensure the system is properly configured.
func (t *TrafficInterceptor) IsAvailable() error {
	// Check if iptables command exists
	if _, err := exec.LookPath("iptables"); err != nil {
		return fmt.Errorf("iptables command not found: %w", err)
	}

	// Test if we can run iptables (requires root privileges)
	cmd := exec.Command("iptables", "-L", "-n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot execute iptables (insufficient privileges?): %w", err)
	}

	return nil
}
