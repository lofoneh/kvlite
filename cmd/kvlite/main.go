// cmd/kvlite/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/engine"
	"github.com/lofoneh/kvlite/pkg/api"
)

var (
	host             = flag.String("host", "", "Host to bind to (default: localhost)")
	port             = flag.Int("port", 0, "Port to listen on (default: 6380)")
	maxConnections   = flag.Int("max-connections", 0, "Maximum concurrent connections (0 = unlimited)")
	walPath          = flag.String("wal-path", "./data", "Path for WAL files")
	syncMode         = flag.Bool("sync-mode", false, "Sync to disk after every write (slower but safer)")
	maxWALEntries    = flag.Int64("max-wal-entries", 10000, "Trigger compaction after this many entries")
	maxWALSize       = flag.Int64("max-wal-size", 10*1024*1024, "Trigger compaction after this size (bytes)")
	compactInterval  = flag.Duration("compact-interval", 1*time.Minute, "How often to check for compaction")
	ttlCheckInterval = flag.Duration("ttl-check-interval", 1*time.Second, "How often to check for expired keys")
	enableAnalytics  = flag.Bool("enable-analytics", true, "Enable AI-powered analytics and smart scheduling")
	version          = flag.Bool("version", false, "Print version and exit")
)

const (
	appVersion = "0.5.0"
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

	// Initialize engine with persistence
	log.Printf("Initializing engine (WAL path: %s, sync mode: %v, analytics: %v)...", *walPath, *syncMode, *enableAnalytics)
	eng, err := engine.New(engine.Options{
		WALPath:            *walPath,
		SyncMode:           *syncMode,
		MaxWALEntries:      *maxWALEntries,
		MaxWALSize:         *maxWALSize,
		CompactionInterval: *compactInterval,
		TTLCheckInterval:   *ttlCheckInterval,
		EnableAnalytics:    *enableAnalytics,
	})
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer eng.Close()

	log.Printf("Engine initialized with %d keys", eng.Len())

	// Create and start server
	server := api.NewServer(cfg, eng)

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
║           kvlite v%s                ║
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