package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-i2p/go-docker-network-i2p/internal/config"
	"github.com/go-i2p/go-docker-network-i2p/pkg/plugin"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Command-line flags
	sockPath := flag.String("sock", "", "Unix socket path (overrides config)")
	configFile := flag.String("config", "", "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		log.Printf("i2p-network-plugin %s (built %s, commit %s)", version, buildTime, gitCommit)
		os.Exit(0)
	}

	// Load configuration: defaults -> file -> environment -> flags
	cfg := config.DefaultConfig()

	if *configFile != "" {
		if err := cfg.LoadFromFile(*configFile); err != nil {
			log.Fatalf("Failed to load configuration file: %v", err)
		}
	}

	if err := cfg.LoadFromEnvironment(); err != nil {
		log.Fatalf("Failed to load environment configuration: %v", err)
	}

	// Apply command-line flag overrides (highest priority)
	if *sockPath != "" {
		cfg.Plugin.SocketPath = *sockPath
	}
	if *debug {
		cfg.Plugin.Debug = true
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting i2p-network-plugin %s", version)

	// Create and start the plugin
	p, err := plugin.New(cfg.Plugin.SocketPath, cfg.GetSAMConfig())
	if err != nil {
		log.Fatalf("Failed to create plugin: %v", err)
	}

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	if err := p.Start(ctx); err != nil {
		log.Fatalf("Plugin error: %v", err)
	}
}
