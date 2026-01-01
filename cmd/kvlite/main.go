// cmd/kvlite/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/store"
	"github.com/lofoneh/kvlite/pkg/api"
)

var (
	host           = flag.String("host", "", "Host to bind to (default: localhost)")
	port           = flag.Int("port", 0, "Port to listen on (default: 6380)")
	maxConnections = flag.Int("max-connections", 0, "Maximum concurrent connections (0 = unlimited)")
	version        = flag.Bool("version", false, "Print version and exit")
)

const (
	appVersion = "0.1.0"
	appName    = "kvlite"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	// Load configuration
	cfg := config.LoadFromEnv()

	// Override with command-line flags
	if *host != "" {
		cfg.Host = *host
	}
	if *port != 0 {
		cfg.Port = *port
	}
	if *maxConnections != 0 {
		cfg.MaxConnections = *maxConnections
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Print startup banner
	printBanner(cfg)

	// Initialize store
	st := store.New()

	// Create and start server
	server := api.NewServer(cfg, st)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v, shutting down...", sig)
		if err := server.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
			os.Exit(1)
		}
	case err := <-errChan:
		if err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}

	log.Println("kvlite stopped")
}

func printBanner(cfg *config.Config) {
	banner := `
╔═══════════════════════════════════════╗
║           kvlite v%s                  ║
║   Fast In-Memory Key-Value Store      ║
╚═══════════════════════════════════════╝
`
	fmt.Printf(banner, appVersion)
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Address:         %s\n", cfg.Address())
	fmt.Printf("  Max Connections: ")
	if cfg.MaxConnections == 0 {
		fmt.Println("unlimited")
	} else {
		fmt.Printf("%d\n", cfg.MaxConnections)
	}
	fmt.Println()
}
