// Package i2p provides I2P connectivity and tunnel management for the Docker network plugin.package i2p

// This package encapsulates all I2P-related functionality, including SAM bridge
// connectivity, tunnel creation and management, and destination key handling.
package i2p

import (
	"context"
	"fmt"
	"log"
	"time"

	sam3 "github.com/go-i2p/go-sam-go"
)

// SAMConfig represents the configuration for connecting to an I2P SAM bridge.
type SAMConfig struct {
	Host     string        `json:"host"`     // SAM bridge host (default: localhost)
	Port     int           `json:"port"`     // SAM bridge port (default: 7656)
	Timeout  time.Duration `json:"timeout"`  // Connection timeout (default: 30s)
	Username string        `json:"username"` // SAM username (optional)
	Password string        `json:"password"` // SAM password (optional)
}

// DefaultSAMConfig returns a default SAM configuration.
func DefaultSAMConfig() *SAMConfig {
	return &SAMConfig{
		Host:    "localhost",
		Port:    7656,
		Timeout: 30 * time.Second,
	}
}

// SAMClient wraps the go-sam-go client with additional functionality for the plugin.
type SAMClient struct {
	config *SAMConfig
	sam    *sam3.SAM
}

// NewSAMClient creates a new SAM client with the given configuration.
//
// This establishes a connection to the I2P SAM bridge and validates that
// the I2P router is accessible and functional.
func NewSAMClient(config *SAMConfig) (*SAMClient, error) {
	if config == nil {
		config = DefaultSAMConfig()
	}

	// Validate configuration
	if err := validateSAMConfig(config); err != nil {
		return nil, fmt.Errorf("invalid SAM configuration: %w", err)
	}

	return &SAMClient{
		config: config,
	}, nil
}

// Connect establishes a connection to the I2P SAM bridge.
//
// This method creates the underlying SAM connection and performs
// initial connectivity verification. The connection attempt is bounded
// by the configured timeout (SAMConfig.Timeout) via the provided context.
func (c *SAMClient) Connect(ctx context.Context) error {
	log.Printf("Connecting to I2P SAM bridge at %s:%d", c.config.Host, c.config.Port)

	// Create SAM connection address
	address := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	// Derive a timeout context from the configured timeout
	connectCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Establish connection with timeout via goroutine + select
	type samResult struct {
		sam *sam3.SAM
		err error
	}
	ch := make(chan samResult, 1)
	go func() {
		sam, err := sam3.NewSAM(address)
		ch <- samResult{sam: sam, err: err}
	}()

	select {
	case <-connectCtx.Done():
		return fmt.Errorf("SAM bridge connection timed out after %v: %w", c.config.Timeout, connectCtx.Err())
	case result := <-ch:
		if result.err != nil {
			return fmt.Errorf("failed to connect to SAM bridge: %w", result.err)
		}
		c.sam = result.sam
	}

	// Verify connectivity by creating a basic resolver
	if err := c.verifyConnectivity(connectCtx); err != nil {
		c.sam.Close()
		c.sam = nil
		return fmt.Errorf("SAM bridge connectivity verification failed: %w", err)
	}

	log.Printf("Successfully connected to I2P SAM bridge")
	return nil
}

// IsConnected returns true if the client is connected to the SAM bridge.
func (c *SAMClient) IsConnected() bool {
	return c.sam != nil
}

// Disconnect closes the connection to the SAM bridge.
func (c *SAMClient) Disconnect() error {
	if c.sam != nil {
		log.Println("Disconnecting from I2P SAM bridge")
		err := c.sam.Close()
		c.sam = nil
		return err
	}
	return nil
}

// GetSAMVersion returns the version of the connected SAM bridge.
func (c *SAMClient) GetSAMVersion() string {
	// Note: go-sam-go doesn't expose version info directly,
	// but we can assume SAM 3.x compatibility
	if c.sam != nil {
		return "3.x"
	}
	return ""
}

// verifyConnectivity performs basic connectivity checks against the SAM bridge.
func (c *SAMClient) verifyConnectivity(ctx context.Context) error {
	if c.sam == nil {
		return fmt.Errorf("no SAM connection")
	}

	// Test basic SAM functionality by creating a resolver
	// This verifies that the SAM bridge is functioning correctly
	resolver, err := sam3.NewSAMResolver(c.sam)
	if err != nil {
		return fmt.Errorf("failed to create SAM resolver: %w", err)
	}

	// The resolver creation itself validates connectivity
	_ = resolver // We don't need to do anything with it for this test

	log.Println("I2P connectivity verified")
	return nil
}

// validateSAMConfig validates the SAM configuration parameters.
func validateSAMConfig(config *SAMConfig) error {
	if config.Host == "" {
		return fmt.Errorf("SAM host cannot be empty")
	}

	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("SAM port must be between 1 and 65535, got %d", config.Port)
	}

	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", config.Timeout)
	}

	return nil
}
