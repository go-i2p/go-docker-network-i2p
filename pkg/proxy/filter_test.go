package proxy

import (
	"strings"
	"testing"
)

func TestNewTrafficFilter(t *testing.T) {
	tests := []struct {
		name   string
		config *FilterConfig
	}{
		{
			name:   "default_config",
			config: nil,
		},
		{
			name: "custom_config",
			config: &FilterConfig{
				EnableAllowlist: true,
				EnableBlocklist: false,
				LogTraffic:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewTrafficFilter(tt.config)
			if filter == nil {
				t.Fatal("Expected filter to be created")
			}

			if tt.config == nil {
				// Should use default config
				if !filter.config.EnableBlocklist {
					t.Error("Expected default config to enable blocklist")
				}
			} else {
				if filter.config.EnableAllowlist != tt.config.EnableAllowlist {
					t.Errorf("Expected EnableAllowlist=%v, got %v", tt.config.EnableAllowlist, filter.config.EnableAllowlist)
				}
			}
		})
	}
}

func TestTrafficFilter_AllowlistOperations(t *testing.T) {
	filter := NewTrafficFilter(DefaultFilterConfig())

	tests := []struct {
		name        string
		destination string
		expectError bool
	}{
		{
			name:        "valid_i2p_domain",
			destination: "example.i2p",
			expectError: false,
		},
		{
			name:        "valid_b32_address",
			destination: "3g2upl4pq6kufc4m.b32.i2p",
			expectError: false,
		},
		{
			name:        "valid_wildcard_pattern",
			destination: "*.example.i2p",
			expectError: false,
		},
		{
			name:        "empty_destination",
			destination: "",
			expectError: true,
		},
		{
			name:        "invalid_non_i2p_domain",
			destination: "example.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filter.AddToAllowlist(tt.destination)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Test removal
				filter.RemoveFromAllowlist(tt.destination)
			}
		})
	}
}

func TestTrafficFilter_BlocklistOperations(t *testing.T) {
	filter := NewTrafficFilter(DefaultFilterConfig())

	tests := []struct {
		name        string
		destination string
		expectError bool
	}{
		{
			name:        "valid_i2p_domain",
			destination: "malicious.i2p",
			expectError: false,
		},
		{
			name:        "valid_wildcard_pattern",
			destination: "*.malicious.i2p",
			expectError: false,
		},
		{
			name:        "invalid_destination",
			destination: "malicious.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filter.AddToBlocklist(tt.destination)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Test removal
				filter.RemoveFromBlocklist(tt.destination)
			}
		})
	}
}

func TestTrafficFilter_ShouldAllowConnection(t *testing.T) {
	tests := []struct {
		name                   string
		setupFilter            func(*TrafficFilter)
		destination            string
		protocol               string
		expectedAllow          bool
		expectedReasonContains string
	}{
		{
			name: "non_i2p_destination_blocked",
			setupFilter: func(f *TrafficFilter) {
				// Default config blocks non-I2P
			},
			destination:            "example.com:80",
			protocol:               "tcp",
			expectedAllow:          false,
			expectedReasonContains: "Non-I2P destination blocked",
		},
		{
			name: "i2p_destination_allowed_by_default",
			setupFilter: func(f *TrafficFilter) {
				// Default config allows I2P when not explicitly blocked
			},
			destination:            "example.i2p:80",
			protocol:               "tcp",
			expectedAllow:          true,
			expectedReasonContains: "I2P destination allowed",
		},
		{
			name: "i2p_destination_blocked_by_blocklist",
			setupFilter: func(f *TrafficFilter) {
				f.AddToBlocklist("blocked.i2p")
			},
			destination:            "blocked.i2p:80",
			protocol:               "tcp",
			expectedAllow:          false,
			expectedReasonContains: "blocked by blocklist",
		},
		{
			name: "i2p_destination_allowed_by_allowlist",
			setupFilter: func(f *TrafficFilter) {
				f.config.EnableAllowlist = true
				f.AddToAllowlist("allowed.i2p")
			},
			destination:            "allowed.i2p:80",
			protocol:               "tcp",
			expectedAllow:          true,
			expectedReasonContains: "allowed by allowlist",
		},
		{
			name: "i2p_destination_blocked_when_not_in_allowlist",
			setupFilter: func(f *TrafficFilter) {
				f.config.EnableAllowlist = true
				f.AddToAllowlist("allowed.i2p")
			},
			destination:            "notallowed.i2p:80",
			protocol:               "tcp",
			expectedAllow:          false,
			expectedReasonContains: "not in allowlist",
		},
		{
			name: "wildcard_allowlist_matching",
			setupFilter: func(f *TrafficFilter) {
				f.config.EnableAllowlist = true
				f.AddToAllowlist("*.example.i2p")
			},
			destination:            "test.example.i2p:80",
			protocol:               "tcp",
			expectedAllow:          true,
			expectedReasonContains: "allowed by allowlist",
		},
		{
			name: "wildcard_blocklist_matching",
			setupFilter: func(f *TrafficFilter) {
				f.AddToBlocklist("*.blocked.i2p")
			},
			destination:            "test.blocked.i2p:80",
			protocol:               "tcp",
			expectedAllow:          false,
			expectedReasonContains: "blocked by blocklist",
		},
		{
			name: "b32_i2p_address_allowed",
			setupFilter: func(f *TrafficFilter) {
				// Default config allows I2P
			},
			destination:            "3g2upl4pq6kufc4m.b32.i2p:80",
			protocol:               "tcp",
			expectedAllow:          true,
			expectedReasonContains: "I2P destination allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewTrafficFilter(DefaultFilterConfig())
			tt.setupFilter(filter)

			allowed, reason := filter.ShouldAllowConnection(tt.destination, tt.protocol)

			if allowed != tt.expectedAllow {
				t.Errorf("Expected allowed=%v, got %v", tt.expectedAllow, allowed)
			}

			if !strings.Contains(reason, tt.expectedReasonContains) {
				t.Errorf("Expected reason to contain '%s', got '%s'", tt.expectedReasonContains, reason)
			}
		})
	}
}

func TestTrafficFilter_WildcardMatching(t *testing.T) {
	filter := NewTrafficFilter(DefaultFilterConfig())

	tests := []struct {
		name        string
		destination string
		pattern     string
		shouldMatch bool
	}{
		{
			name:        "exact_match",
			destination: "example.i2p",
			pattern:     "example.i2p",
			shouldMatch: true,
		},
		{
			name:        "wildcard_prefix",
			destination: "test.example.i2p",
			pattern:     "*.example.i2p",
			shouldMatch: true,
		},
		{
			name:        "wildcard_suffix",
			destination: "example.test.i2p",
			pattern:     "example.*.i2p",
			shouldMatch: true,
		},
		{
			name:        "wildcard_middle",
			destination: "test.middle.example.i2p",
			pattern:     "test.*.example.i2p",
			shouldMatch: true,
		},
		{
			name:        "no_match",
			destination: "different.i2p",
			pattern:     "example.i2p",
			shouldMatch: false,
		},
		{
			name:        "wildcard_no_match",
			destination: "test.different.i2p",
			pattern:     "*.example.i2p",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := filter.matchesWildcard(tt.destination, tt.pattern)
			if matched != tt.shouldMatch {
				t.Errorf("Expected match=%v for destination '%s' and pattern '%s', got %v",
					tt.shouldMatch, tt.destination, tt.pattern, matched)
			}
		})
	}
}

func TestTrafficFilter_LogConnection(t *testing.T) {
	config := DefaultFilterConfig()
	config.LogTraffic = true
	filter := NewTrafficFilter(config)

	// Log a connection
	filter.LogConnection("192.168.1.10", "example.i2p:80", "tcp", 1024)

	// Check stats
	stats := filter.GetStats()
	if stats.TotalBytesTransferred != 1024 {
		t.Errorf("Expected bytes transferred to be 1024, got %d", stats.TotalBytesTransferred)
	}

	if stats.LastActivity.IsZero() {
		t.Error("Expected LastActivity to be set")
	}

	// Check log entries
	logs := filter.GetRecentLogs(10)
	if len(logs) == 0 {
		t.Error("Expected log entries to be created")
	}

	if logs[len(logs)-1].BytesTransferred != 1024 {
		t.Errorf("Expected log entry bytes to be 1024, got %d", logs[len(logs)-1].BytesTransferred)
	}
}

func TestTrafficFilter_StatsTracking(t *testing.T) {
	filter := NewTrafficFilter(DefaultFilterConfig())

	// Test I2P connection allowed
	filter.ShouldAllowConnection("example.i2p:80", "tcp")

	// Test I2P connection blocked
	filter.AddToBlocklist("blocked.i2p")
	filter.ShouldAllowConnection("blocked.i2p:80", "tcp")

	// Test non-I2P connection blocked
	filter.ShouldAllowConnection("example.com:80", "tcp")

	stats := filter.GetStats()

	if stats.I2PConnectionsAllowed != 1 {
		t.Errorf("Expected 1 I2P connection allowed, got %d", stats.I2PConnectionsAllowed)
	}

	if stats.I2PConnectionsBlocked != 1 {
		t.Errorf("Expected 1 I2P connection blocked, got %d", stats.I2PConnectionsBlocked)
	}

	if stats.NonI2PConnectionsBlocked != 1 {
		t.Errorf("Expected 1 non-I2P connection blocked, got %d", stats.NonI2PConnectionsBlocked)
	}
}

func TestTrafficFilter_ClearStats(t *testing.T) {
	filter := NewTrafficFilter(DefaultFilterConfig())

	// Generate some stats
	filter.ShouldAllowConnection("example.i2p:80", "tcp")
	filter.LogConnection("192.168.1.10", "example.i2p:80", "tcp", 1024)

	// Verify stats exist
	stats := filter.GetStats()
	if stats.I2PConnectionsAllowed == 0 || stats.TotalBytesTransferred == 0 {
		t.Error("Expected stats to be non-zero before clearing")
	}

	// Clear stats
	filter.ClearStats()

	// Verify stats are cleared
	clearedStats := filter.GetStats()
	if clearedStats.I2PConnectionsAllowed != 0 {
		t.Errorf("Expected I2P connections allowed to be 0 after clearing, got %d", clearedStats.I2PConnectionsAllowed)
	}

	if clearedStats.TotalBytesTransferred != 0 {
		t.Errorf("Expected total bytes transferred to be 0 after clearing, got %d", clearedStats.TotalBytesTransferred)
	}

	if len(clearedStats.LogEntries) != 1 {
		t.Errorf("Expected 1 log entry after clearing (for the clear action), got %d entries", len(clearedStats.LogEntries))
	} else if clearedStats.LogEntries[0].Action != "ADMIN" {
		t.Errorf("Expected first log entry to be an ADMIN action, got %s", clearedStats.LogEntries[0].Action)
	}
}

func TestTrafficFilter_LogRetention(t *testing.T) {
	config := DefaultFilterConfig()
	config.MaxLogEntries = 3 // Small limit for testing
	filter := NewTrafficFilter(config)

	// Add more log entries than the limit
	for i := 0; i < 5; i++ {
		filter.ShouldAllowConnection("example.i2p:80", "tcp")
	}

	logs := filter.GetRecentLogs(10)
	if len(logs) > config.MaxLogEntries {
		t.Errorf("Expected log entries to be limited to %d, got %d", config.MaxLogEntries, len(logs))
	}
}

func TestTrafficFilter_IsValidI2PDestination(t *testing.T) {
	filter := NewTrafficFilter(DefaultFilterConfig())

	tests := []struct {
		name        string
		destination string
		isValid     bool
	}{
		{
			name:        "valid_i2p_domain",
			destination: "example.i2p",
			isValid:     true,
		},
		{
			name:        "valid_b32_address",
			destination: "3g2upl4pq6kufc4m.b32.i2p",
			isValid:     true,
		},
		{
			name:        "valid_wildcard_pattern",
			destination: "*.example.i2p",
			isValid:     true,
		},
		{
			name:        "invalid_regular_domain",
			destination: "example.com",
			isValid:     false,
		},
		{
			name:        "invalid_wildcard_regular_domain",
			destination: "*.example.com",
			isValid:     false,
		},
		{
			name:        "empty_destination",
			destination: "",
			isValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := filter.isValidI2PDestination(tt.destination)
			if isValid != tt.isValid {
				t.Errorf("Expected isValidI2PDestination('%s') = %v, got %v",
					tt.destination, tt.isValid, isValid)
			}
		})
	}
}
