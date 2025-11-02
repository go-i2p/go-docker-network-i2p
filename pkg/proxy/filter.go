package proxy

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TrafficFilter provides comprehensive traffic filtering and monitoring for I2P networks.
//
// The filter implements allowlist/blocklist functionality, traffic analysis,
// and security monitoring to prevent traffic leaks and enforce access policies.
type TrafficFilter struct {
	// config holds the filter configuration
	config *FilterConfig
	// allowlist contains allowed I2P destinations
	allowlist map[string]bool
	// blocklist contains blocked I2P destinations
	blocklist map[string]bool
	// stats tracks traffic statistics
	stats *TrafficStats
	// mutex protects concurrent access to filter state
	mutex sync.RWMutex
}

// FilterConfig defines configuration for traffic filtering.
type FilterConfig struct {
	// EnableAllowlist enables allowlist-based filtering
	EnableAllowlist bool
	// EnableBlocklist enables blocklist-based filtering
	EnableBlocklist bool
	// LogTraffic enables detailed traffic logging
	LogTraffic bool
	// LogNonI2P enables logging of non-I2P traffic attempts
	LogNonI2P bool
	// MaxLogEntries limits the number of log entries to keep in memory
	MaxLogEntries int
	// StatsRetentionPeriod defines how long to keep traffic statistics
	StatsRetentionPeriod time.Duration
}

// DefaultFilterConfig returns a secure default filter configuration.
func DefaultFilterConfig() *FilterConfig {
	return &FilterConfig{
		EnableAllowlist:      false, // Disabled by default for ease of use
		EnableBlocklist:      true,  // Enable basic protection
		LogTraffic:           true,  // Enable traffic monitoring
		LogNonI2P:            true,  // Log potential leaks
		MaxLogEntries:        1000,  // Keep last 1000 entries
		StatsRetentionPeriod: 24 * time.Hour,
	}
}

// TrafficStats tracks network traffic statistics and security events.
type TrafficStats struct {
	// I2PConnectionsAllowed counts successful I2P connections
	I2PConnectionsAllowed int64
	// I2PConnectionsBlocked counts blocked I2P connections
	I2PConnectionsBlocked int64
	// NonI2PConnectionsBlocked counts blocked non-I2P connections
	NonI2PConnectionsBlocked int64
	// TotalBytesTransferred tracks total data volume
	TotalBytesTransferred int64
	// LastActivity records the timestamp of the last network activity
	LastActivity time.Time
	// LogEntries contains recent traffic log entries
	LogEntries []TrafficLogEntry
	// mutex protects concurrent access to stats
	mutex sync.RWMutex
}

// TrafficLogEntry represents a single traffic event.
type TrafficLogEntry struct {
	// Timestamp when the event occurred
	Timestamp time.Time
	// Action taken (ALLOW, BLOCK, LOG)
	Action string
	// Protocol used (TCP, UDP)
	Protocol string
	// Source address
	Source string
	// Destination address
	Destination string
	// Reason for the action
	Reason string
	// BytesTransferred in this connection
	BytesTransferred int64
}

// NewTrafficFilter creates a new traffic filter with the given configuration.
func NewTrafficFilter(config *FilterConfig) *TrafficFilter {
	if config == nil {
		config = DefaultFilterConfig()
	}

	return &TrafficFilter{
		config:    config,
		allowlist: make(map[string]bool),
		blocklist: make(map[string]bool),
		stats: &TrafficStats{
			LogEntries: make([]TrafficLogEntry, 0, config.MaxLogEntries),
		},
	}
}

// AddToAllowlist adds a destination to the allowlist.
//
// Destinations can be exact matches (example.i2p) or patterns (*.example.i2p).
// The allowlist takes precedence over the blocklist.
func (tf *TrafficFilter) AddToAllowlist(destination string) error {
	if destination == "" {
		return fmt.Errorf("destination cannot be empty")
	}

	// Validate I2P destination format
	if !tf.isValidI2PDestination(destination) {
		return fmt.Errorf("invalid I2P destination format: %s", destination)
	}

	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	tf.allowlist[strings.ToLower(destination)] = true

	if tf.config.LogTraffic {
		log.Printf("Added destination to allowlist: %s", destination)
	}

	return nil
}

// AddToBlocklist adds a destination to the blocklist.
//
// Destinations can be exact matches (example.i2p) or patterns (*.example.i2p).
// The allowlist takes precedence over the blocklist.
func (tf *TrafficFilter) AddToBlocklist(destination string) error {
	if destination == "" {
		return fmt.Errorf("destination cannot be empty")
	}

	// Validate I2P destination format
	if !tf.isValidI2PDestination(destination) {
		return fmt.Errorf("invalid I2P destination format: %s", destination)
	}

	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	tf.blocklist[strings.ToLower(destination)] = true

	if tf.config.LogTraffic {
		log.Printf("Added destination to blocklist: %s", destination)
	}

	return nil
}

// RemoveFromAllowlist removes a destination from the allowlist.
func (tf *TrafficFilter) RemoveFromAllowlist(destination string) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	delete(tf.allowlist, strings.ToLower(destination))

	if tf.config.LogTraffic {
		log.Printf("Removed destination from allowlist: %s", destination)
	}
}

// RemoveFromBlocklist removes a destination from the blocklist.
func (tf *TrafficFilter) RemoveFromBlocklist(destination string) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	delete(tf.blocklist, strings.ToLower(destination))

	if tf.config.LogTraffic {
		log.Printf("Removed destination from blocklist: %s", destination)
	}
}

// ShouldAllowConnection determines if a connection should be allowed based on filtering rules.
//
// This method checks the destination against allowlist/blocklist rules and
// returns the decision along with a reason for logging.
func (tf *TrafficFilter) ShouldAllowConnection(destination string, protocol string) (bool, string) {
	tf.mutex.RLock()
	defer tf.mutex.RUnlock()

	// Normalize destination for consistent matching
	dest := strings.ToLower(destination)

	// Extract hostname from destination (remove port if present)
	host, _, err := net.SplitHostPort(dest)
	if err != nil {
		// Destination might not have a port, use as-is
		host = dest
	}

	// Check if this is an I2P destination
	if !tf.isI2PDestination(host) {
		// Non-I2P traffic is always blocked
		reason := fmt.Sprintf("Non-I2P destination blocked: %s", host)
		tf.logTrafficEvent("BLOCK", protocol, "", dest, reason, 0)
		tf.stats.NonI2PConnectionsBlocked++
		return false, reason
	}

	// Check allowlist first (takes precedence)
	if tf.config.EnableAllowlist {
		if allowed := tf.matchesPattern(host, tf.allowlist); allowed {
			reason := fmt.Sprintf("I2P destination allowed by allowlist: %s", host)
			tf.logTrafficEvent("ALLOW", protocol, "", dest, reason, 0)
			tf.stats.I2PConnectionsAllowed++
			return true, reason
		}
		// If allowlist is enabled but destination not found, block it
		reason := fmt.Sprintf("I2P destination not in allowlist: %s", host)
		tf.logTrafficEvent("BLOCK", protocol, "", dest, reason, 0)
		tf.stats.I2PConnectionsBlocked++
		return false, reason
	}

	// Check blocklist
	if tf.config.EnableBlocklist {
		if blocked := tf.matchesPattern(host, tf.blocklist); blocked {
			reason := fmt.Sprintf("I2P destination blocked by blocklist: %s", host)
			tf.logTrafficEvent("BLOCK", protocol, "", dest, reason, 0)
			tf.stats.I2PConnectionsBlocked++
			return false, reason
		}
	}

	// Default: allow I2P traffic if not explicitly blocked
	reason := fmt.Sprintf("I2P destination allowed: %s", host)
	tf.logTrafficEvent("ALLOW", protocol, "", dest, reason, 0)
	tf.stats.I2PConnectionsAllowed++
	return true, reason
}

// LogConnection records a completed connection for traffic analysis.
//
// This method should be called when a connection completes to track
// bandwidth usage and connection patterns.
func (tf *TrafficFilter) LogConnection(source, destination string, protocol string, bytesTransferred int64) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	tf.stats.TotalBytesTransferred += bytesTransferred
	tf.stats.LastActivity = time.Now()

	reason := fmt.Sprintf("Connection completed, %d bytes transferred", bytesTransferred)
	tf.logTrafficEvent("LOG", protocol, source, destination, reason, bytesTransferred)
}

// GetStats returns a copy of current traffic statistics.
func (tf *TrafficFilter) GetStats() TrafficStats {
	tf.stats.mutex.RLock()
	defer tf.stats.mutex.RUnlock()

	// Create a deep copy of stats
	statsCopy := TrafficStats{
		I2PConnectionsAllowed:    tf.stats.I2PConnectionsAllowed,
		I2PConnectionsBlocked:    tf.stats.I2PConnectionsBlocked,
		NonI2PConnectionsBlocked: tf.stats.NonI2PConnectionsBlocked,
		TotalBytesTransferred:    tf.stats.TotalBytesTransferred,
		LastActivity:             tf.stats.LastActivity,
		LogEntries:               make([]TrafficLogEntry, len(tf.stats.LogEntries)),
	}

	copy(statsCopy.LogEntries, tf.stats.LogEntries)
	return statsCopy
}

// GetRecentLogs returns recent traffic log entries.
func (tf *TrafficFilter) GetRecentLogs(limit int) []TrafficLogEntry {
	tf.stats.mutex.RLock()
	defer tf.stats.mutex.RUnlock()

	if limit <= 0 || limit > len(tf.stats.LogEntries) {
		limit = len(tf.stats.LogEntries)
	}

	// Return the most recent entries
	start := len(tf.stats.LogEntries) - limit
	if start < 0 {
		start = 0
	}

	logs := make([]TrafficLogEntry, limit)
	copy(logs, tf.stats.LogEntries[start:])
	return logs
}

// ClearStats resets all traffic statistics and logs
func (tf *TrafficFilter) ClearStats() {
	tf.stats.mutex.Lock()
	defer tf.stats.mutex.Unlock()

	tf.stats.I2PConnectionsAllowed = 0
	tf.stats.I2PConnectionsBlocked = 0
	tf.stats.NonI2PConnectionsBlocked = 0
	tf.stats.TotalBytesTransferred = 0
	tf.stats.LastActivity = time.Time{}
	tf.stats.LogEntries = make([]TrafficLogEntry, 0)

	// Create log entry directly without using logTrafficEvent to avoid deadlock
	logEntry := TrafficLogEntry{
		Timestamp:        time.Now(),
		Action:           "ADMIN",
		Protocol:         "SYSTEM",
		Source:           "localhost",
		Destination:      "*",
		Reason:           "Statistics cleared",
		BytesTransferred: 0,
	}
	tf.stats.LogEntries = append(tf.stats.LogEntries, logEntry)
} // GetAllowlist returns a copy of the current allowlist
func (tf *TrafficFilter) GetAllowlist() []string {
	tf.mutex.RLock()
	defer tf.mutex.RUnlock()

	result := make([]string, 0, len(tf.allowlist))
	for destination := range tf.allowlist {
		result = append(result, destination)
	}
	return result
}

// GetBlocklist returns a copy of the current blocklist
func (tf *TrafficFilter) GetBlocklist() []string {
	tf.mutex.RLock()
	defer tf.mutex.RUnlock()

	result := make([]string, 0, len(tf.blocklist))
	for destination := range tf.blocklist {
		result = append(result, destination)
	}
	return result
} // isValidI2PDestination validates that a destination follows I2P naming conventions.
func (tf *TrafficFilter) isValidI2PDestination(destination string) bool {
	// Allow wildcard patterns for allowlist/blocklist
	if strings.Contains(destination, "*") {
		return tf.isValidI2PPattern(destination)
	}

	return tf.isI2PDestination(destination)
}

// isValidI2PPattern validates wildcard patterns for I2P destinations.
func (tf *TrafficFilter) isValidI2PPattern(pattern string) bool {
	// Convert to lowercase for case-insensitive comparison
	pat := strings.ToLower(pattern)

	// Simple validation: must end with .i2p and contain valid characters
	if !strings.HasSuffix(pat, ".i2p") {
		return false
	}

	// Check for valid characters (alphanumeric, dash, dot, asterisk)
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9\-.*]+\.i2p$`)
	return validPattern.MatchString(pat)
}

// isI2PDestination checks if a destination is a valid I2P address.
func (tf *TrafficFilter) isI2PDestination(destination string) bool {
	// Convert to lowercase for case-insensitive comparison
	dest := strings.ToLower(destination)

	// Check for .i2p domain
	if strings.HasSuffix(dest, ".i2p") {
		return true
	}

	// Check for base32 I2P address (52 characters ending in .b32.i2p)
	if strings.HasSuffix(dest, ".b32.i2p") && len(dest) == 60 {
		return true
	}

	return false
}

// matchesPattern checks if a destination matches any pattern in the given map.
func (tf *TrafficFilter) matchesPattern(destination string, patterns map[string]bool) bool {
	// Check for exact match first
	if patterns[destination] {
		return true
	}

	// Check wildcard patterns
	for pattern := range patterns {
		if tf.matchesWildcard(destination, pattern) {
			return true
		}
	}

	return false
}

// matchesWildcard checks if a destination matches a wildcard pattern.
func (tf *TrafficFilter) matchesWildcard(destination, pattern string) bool {
	// Simple wildcard matching: * matches any sequence of characters
	if !strings.Contains(pattern, "*") {
		return destination == pattern
	}

	// Convert wildcard pattern to regex
	regexPattern := strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*")
	regexPattern = "^" + regexPattern + "$"

	matched, err := regexp.MatchString(regexPattern, destination)
	if err != nil {
		return false
	}

	return matched
}

// logTrafficEvent adds a traffic event to the log.
func (tf *TrafficFilter) logTrafficEvent(action, protocol, source, destination, reason string, bytes int64) {
	if !tf.config.LogTraffic {
		return
	}

	entry := TrafficLogEntry{
		Timestamp:        time.Now(),
		Action:           action,
		Protocol:         protocol,
		Source:           source,
		Destination:      destination,
		Reason:           reason,
		BytesTransferred: bytes,
	}

	// Add to stats log entries
	tf.stats.mutex.Lock()
	tf.stats.LogEntries = append(tf.stats.LogEntries, entry)

	// Limit log entries to prevent memory growth
	if len(tf.stats.LogEntries) > tf.config.MaxLogEntries {
		// Remove oldest entries
		copy(tf.stats.LogEntries, tf.stats.LogEntries[1:])
		tf.stats.LogEntries = tf.stats.LogEntries[:tf.config.MaxLogEntries]
	}
	tf.stats.mutex.Unlock()

	// Log to system logger
	log.Printf("TRAFFIC %s: %s %s -> %s (%s)", action, protocol, source, destination, reason)
}
