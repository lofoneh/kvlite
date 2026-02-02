// tests/integration_test.go
package tests

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/engine"
	"github.com/lofoneh/kvlite/pkg/api"
)

// TestServer is a helper for integration tests
type TestServer struct {
	server *api.Server
	engine *engine.Engine
	addr   string
}

func setupTestServer(t *testing.T) *TestServer {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Host:           "localhost",
		Port:           0, // Random port - OS will assign available port
		MaxConnections: 0,
	}

	eng, err := engine.New(engine.Options{
		WALPath:         tmpDir,
		SyncMode:        false,
		EnableAnalytics: true,
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	server := api.NewServer(cfg, eng)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for server to start and check for errors
	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-errChan:
		t.Fatalf("Server failed to start: %v", err)
	default:
	}

	return &TestServer{
		server: server,
		engine: eng,
		addr:   server.Addr(), // Use actual address from listener
	}
}

func (ts *TestServer) sendCommand(t *testing.T, cmd string) string {
	conn, err := net.Dial("tcp", ts.addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome
	_, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read welcome: %v", err)
	}

	// Send command
	fmt.Fprintf(writer, "%s\n", cmd)
	writer.Flush()

	// Read response
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	return response[:len(response)-1] // Remove trailing newline
}

func (ts *TestServer) close() {
	ts.server.Shutdown()
	ts.engine.Close()
}

// Integration Tests

func TestIntegration_BasicOperations(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	// SET
	response := ts.sendCommand(t, "SET key1 value1")
	if response != "+OK" {
		t.Errorf("SET failed: %s", response)
	}

	// GET
	response = ts.sendCommand(t, "GET key1")
	if response != "value1" {
		t.Errorf("GET failed: expected value1, got %s", response)
	}

	// DELETE
	response = ts.sendCommand(t, "DELETE key1")
	if response != "+OK" {
		t.Errorf("DELETE failed: %s", response)
	}

	// GET after DELETE
	response = ts.sendCommand(t, "GET key1")
	if response != "-ERR key not found" {
		t.Errorf("GET after DELETE should fail: %s", response)
	}
}

func TestIntegration_TTL(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	// SETEX
	response := ts.sendCommand(t, "SETEX temp 2 value")
	if response != "+OK" {
		t.Errorf("SETEX failed: %s", response)
	}

	// GET immediately
	response = ts.sendCommand(t, "GET temp")
	if response != "value" {
		t.Errorf("GET failed: %s", response)
	}

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// GET after expiration
	response = ts.sendCommand(t, "GET temp")
	if response != "-ERR key not found" {
		t.Errorf("Key should be expired: %s", response)
	}
}

func TestIntegration_ConcurrentClients(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	done := make(chan bool)
	errors := make(chan error, 10)

	// Spawn 10 concurrent clients
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("key%d_%d", id, j)
				value := fmt.Sprintf("value%d_%d", id, j)

				// SET
				response := ts.sendCommand(t, fmt.Sprintf("SET %s %s", key, value))
				if response != "+OK" {
					errors <- fmt.Errorf("SET failed for %s: %s", key, response)
					return
				}

				// GET
				response = ts.sendCommand(t, fmt.Sprintf("GET %s", key))
				if response != value {
					errors <- fmt.Errorf("GET failed for %s: expected %s, got %s", key, value, response)
					return
				}
			}
		}(i)
	}

	// Wait for all clients
	for i := 0; i < 10; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestIntegration_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first server with random port
	cfg1 := &config.Config{
		Host: "localhost",
		Port: 0, // Random port
	}

	eng1, err := engine.New(engine.Options{
		WALPath:  tmpDir,
		SyncMode: false,
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	server1 := api.NewServer(cfg1, eng1)
	go server1.Start()
	time.Sleep(100 * time.Millisecond)

	addr1 := server1.Addr()

	// Write data
	conn, err := net.Dial("tcp", addr1)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)
	reader.ReadString('\n') // Welcome

	fmt.Fprintf(writer, "SET persist test_value\n")
	writer.Flush()
	reader.ReadString('\n')

	// IMPORTANT: Close connection BEFORE shutting down server
	conn.Close()

	// Close first server
	server1.Shutdown()
	eng1.Close()

	time.Sleep(100 * time.Millisecond)

	// Create second server with same data directory
	cfg2 := &config.Config{
		Host: "localhost",
		Port: 0, // Random port
	}

	eng2, err := engine.New(engine.Options{
		WALPath:  tmpDir,
		SyncMode: false,
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	server2 := api.NewServer(cfg2, eng2)
	go server2.Start()
	time.Sleep(100 * time.Millisecond)

	addr2 := server2.Addr()

	// Verify data persisted
	conn, err = net.Dial("tcp", addr2)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	writer = bufio.NewWriter(conn)
	reader = bufio.NewReader(conn)
	reader.ReadString('\n') // Welcome

	fmt.Fprintf(writer, "GET persist\n")
	writer.Flush()
	response, _ := reader.ReadString('\n')

	// Close connection BEFORE shutting down server
	conn.Close()

	if response[:len(response)-1] != "test_value" {
		t.Errorf("Data not persisted: %s", response)
	}

	server2.Shutdown()
	eng2.Close()
}

func TestIntegration_Analytics(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	// Generate access pattern
	for i := 0; i < 50; i++ {
		ts.sendCommand(t, "SET key1 value1")
		ts.sendCommand(t, "GET key1")
	}

	// Check analytics
	response := ts.sendCommand(t, "ANALYZE key1")
	if response == "-ERR analytics not enabled or key not found" {
		t.Error("Analytics should be enabled")
	}

	// Check hot keys
	response = ts.sendCommand(t, "HOTKEYS 5")
	if response == "-ERR analytics not enabled" {
		t.Error("Analytics should be enabled")
	}
}

// Benchmark Tests

func BenchmarkIntegration_SET(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := &config.Config{Host: "localhost", Port: 0}
	eng, _ := engine.New(engine.Options{WALPath: tmpDir})
	server := api.NewServer(cfg, eng)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	addr := server.Addr()

	defer func() {
		server.Shutdown()
		eng.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, _ := net.Dial("tcp", addr)
		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)
		reader.ReadString('\n')

		fmt.Fprintf(writer, "SET bench value\n")
		writer.Flush()
		reader.ReadString('\n')

		conn.Close()
	}
}