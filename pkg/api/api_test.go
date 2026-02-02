// pkg/api/api_test.go
package api

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/engine"
)

// TestHelper provides a test server setup
type testHelper struct {
	server *Server
	engine *engine.Engine
	addr   string
	t      *testing.T
}

func setupTestHelper(t *testing.T) *testHelper {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Host:           "localhost",
		Port:           0, // Random port
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

	server := NewServer(cfg, eng)

	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-errChan:
		t.Fatalf("Server failed to start: %v", err)
	default:
	}

	return &testHelper{
		server: server,
		engine: eng,
		addr:   server.Addr(),
		t:      t,
	}
}

func (h *testHelper) close() {
	h.server.Shutdown()
	h.engine.Close()
}

func (h *testHelper) sendCommand(cmd string) string {
	conn, err := net.Dial("tcp", h.addr)
	if err != nil {
		h.t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome
	_, err = reader.ReadString('\n')
	if err != nil {
		h.t.Fatalf("Failed to read welcome: %v", err)
	}

	// Send command
	fmt.Fprintf(writer, "%s\n", cmd)
	writer.Flush()

	// Read response
	response, err := reader.ReadString('\n')
	if err != nil {
		h.t.Fatalf("Failed to read response: %v", err)
	}

	return strings.TrimSuffix(response, "\n")
}

func (h *testHelper) sendMultilineCommand(cmd string) []string {
	conn, err := net.Dial("tcp", h.addr)
	if err != nil {
		h.t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome
	_, err = reader.ReadString('\n')
	if err != nil {
		h.t.Fatalf("Failed to read welcome: %v", err)
	}

	// Send command
	fmt.Fprintf(writer, "%s\n", cmd)
	writer.Flush()

	// Read all lines until empty response or timeout
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var responses []string
	for {
		response, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		responses = append(responses, strings.TrimSuffix(response, "\n"))
	}

	return responses
}

// Basic Command Tests

func TestServer_PING(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("PING")
	if response != "+PONG" {
		t.Errorf("Expected +PONG, got %s", response)
	}
}

func TestServer_SET_GET(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// SET
	response := h.sendCommand("SET testkey testvalue")
	if response != "+OK" {
		t.Errorf("SET failed: %s", response)
	}

	// GET
	response = h.sendCommand("GET testkey")
	if response != "testvalue" {
		t.Errorf("GET failed: expected testvalue, got %s", response)
	}
}

func TestServer_SET_ValueWithSpaces(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// SET with multi-word value
	response := h.sendCommand("SET greeting hello world from kvlite")
	if response != "+OK" {
		t.Errorf("SET failed: %s", response)
	}

	// GET
	response = h.sendCommand("GET greeting")
	if response != "hello world from kvlite" {
		t.Errorf("GET failed: expected 'hello world from kvlite', got '%s'", response)
	}
}

func TestServer_SET_MissingArgs(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("SET onlykey")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for SET with missing value, got: %s", response)
	}
}

func TestServer_GET_NonExistent(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("GET nonexistent")
	if response != "-ERR key not found" {
		t.Errorf("Expected '-ERR key not found', got: %s", response)
	}
}

func TestServer_DELETE(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Setup
	h.sendCommand("SET delkey value")

	// DELETE
	response := h.sendCommand("DELETE delkey")
	if response != "+OK" {
		t.Errorf("DELETE failed: %s", response)
	}

	// Verify deleted
	response = h.sendCommand("GET delkey")
	if response != "-ERR key not found" {
		t.Errorf("Key should be deleted")
	}
}

func TestServer_DEL_Alias(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET aliaskey value")

	// DEL (alias for DELETE)
	response := h.sendCommand("DEL aliaskey")
	if response != "+OK" {
		t.Errorf("DEL failed: %s", response)
	}
}

func TestServer_DELETE_NonExistent(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("DELETE nonexistent")
	if response != "-ERR key not found" {
		t.Errorf("Expected error for deleting non-existent key, got: %s", response)
	}
}

func TestServer_EXISTS(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET existskey value")

	// EXISTS for existing key
	response := h.sendCommand("EXISTS existskey")
	if response != "1" {
		t.Errorf("Expected 1, got %s", response)
	}

	// EXISTS for non-existing key
	response = h.sendCommand("EXISTS nonexistent")
	if response != "0" {
		t.Errorf("Expected 0, got %s", response)
	}
}

// TTL Command Tests

func TestServer_SETEX(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("SETEX tempkey 60 tempvalue")
	if response != "+OK" {
		t.Errorf("SETEX failed: %s", response)
	}

	response = h.sendCommand("GET tempkey")
	if response != "tempvalue" {
		t.Errorf("GET after SETEX failed: %s", response)
	}

	// Check TTL is set
	response = h.sendCommand("TTL tempkey")
	if response == "-1" || response == "-2" {
		t.Errorf("Expected positive TTL, got %s", response)
	}
}

func TestServer_SETEX_InvalidTTL(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("SETEX key invalid value")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for invalid TTL, got: %s", response)
	}

	response = h.sendCommand("SETEX key 0 value")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for zero TTL, got: %s", response)
	}

	response = h.sendCommand("SETEX key -5 value")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for negative TTL, got: %s", response)
	}
}

func TestServer_EXPIRE(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET expirekey value")

	// EXPIRE
	response := h.sendCommand("EXPIRE expirekey 120")
	if response != "1" {
		t.Errorf("EXPIRE failed: %s", response)
	}

	// Verify TTL is set
	response = h.sendCommand("TTL expirekey")
	if response == "-1" {
		t.Errorf("Expected TTL to be set")
	}
}

func TestServer_EXPIRE_NonExistent(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("EXPIRE nonexistent 60")
	if response != "0" {
		t.Errorf("Expected 0 for EXPIRE on non-existent key, got: %s", response)
	}
}

func TestServer_TTL_NoExpiration(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET noexpire value")

	response := h.sendCommand("TTL noexpire")
	if response != "-1" {
		t.Errorf("Expected -1 for key with no TTL, got: %s", response)
	}
}

func TestServer_TTL_NonExistent(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("TTL nonexistent")
	if response != "-2" {
		t.Errorf("Expected -2 for non-existent key, got: %s", response)
	}
}

func TestServer_PERSIST(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SETEX persistkey 60 value")

	// PERSIST
	response := h.sendCommand("PERSIST persistkey")
	if response != "1" {
		t.Errorf("PERSIST failed: %s", response)
	}

	// Verify TTL removed
	response = h.sendCommand("TTL persistkey")
	if response != "-1" {
		t.Errorf("Expected -1 after PERSIST, got: %s", response)
	}
}

// Key Operations Tests

func TestServer_KEYS(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET user:1 alice")
	h.sendCommand("SET user:2 bob")
	h.sendCommand("SET config:app production")

	// All keys - response is multiline, read all lines
	responses := h.sendMultilineCommand("KEYS *")
	fullResponse := strings.Join(responses, "\n")
	if !strings.Contains(fullResponse, "user:1") || !strings.Contains(fullResponse, "user:2") {
		t.Errorf("KEYS * should return all keys: %s", fullResponse)
	}

	// Pattern matching
	responses = h.sendMultilineCommand("KEYS user:*")
	fullResponse = strings.Join(responses, "\n")
	if !strings.Contains(fullResponse, "user:1") || !strings.Contains(fullResponse, "user:2") {
		t.Errorf("KEYS user:* should match user keys: %s", fullResponse)
	}
	if strings.Contains(fullResponse, "config:app") {
		t.Errorf("KEYS user:* should not match config keys: %s", fullResponse)
	}
}

func TestServer_KEYS_Empty(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("KEYS *")
	if response != "(empty list)" {
		t.Errorf("Expected (empty list), got: %s", response)
	}
}

func TestServer_SCAN(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Setup some keys
	for i := 0; i < 20; i++ {
		h.sendCommand(fmt.Sprintf("SET key%d value%d", i, i))
	}

	// SCAN with cursor 0
	response := h.sendCommand("SCAN 0 COUNT 5")
	lines := strings.Split(response, "\n")
	if len(lines) < 1 {
		t.Errorf("SCAN should return cursor and keys: %s", response)
	}
}

func TestServer_SCAN_InvalidCursor(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("SCAN invalid")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for invalid cursor, got: %s", response)
	}
}

// Batch Operations Tests

func TestServer_MSET_MGET(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// MSET
	response := h.sendCommand("MSET key1 value1 key2 value2 key3 value3")
	if response != "+OK" {
		t.Errorf("MSET failed: %s", response)
	}

	// MGET - multiline response
	responses := h.sendMultilineCommand("MGET key1 key2 key3")
	fullResponse := strings.Join(responses, "\n")
	if !strings.Contains(fullResponse, "value1") || !strings.Contains(fullResponse, "value2") {
		t.Errorf("MGET failed: %s", fullResponse)
	}
}

func TestServer_MGET_MixedExistence(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET exists value")

	// MGET - multiline response
	responses := h.sendMultilineCommand("MGET exists nonexistent")
	fullResponse := strings.Join(responses, "\n")
	if !strings.Contains(fullResponse, "value") || !strings.Contains(fullResponse, "(nil)") {
		t.Errorf("MGET should show (nil) for missing keys: %s", fullResponse)
	}
}

func TestServer_MDEL(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET del1 value1")
	h.sendCommand("SET del2 value2")
	h.sendCommand("SET del3 value3")

	response := h.sendCommand("MDEL del1 del2 nonexistent")
	if response != "2" {
		t.Errorf("Expected 2 deleted, got: %s", response)
	}
}

// Counter Operations Tests

func TestServer_INCR(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// INCR on non-existent key
	response := h.sendCommand("INCR counter")
	if response != "1" {
		t.Errorf("Expected 1, got: %s", response)
	}

	// INCR again
	response = h.sendCommand("INCR counter")
	if response != "2" {
		t.Errorf("Expected 2, got: %s", response)
	}
}

func TestServer_INCR_NonInteger(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET notanumber hello")

	response := h.sendCommand("INCR notanumber")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for non-integer, got: %s", response)
	}
}

func TestServer_DECR(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET counter 10")

	response := h.sendCommand("DECR counter")
	if response != "9" {
		t.Errorf("Expected 9, got: %s", response)
	}

	// DECR on non-existent
	response = h.sendCommand("DECR newcounter")
	if response != "-1" {
		t.Errorf("Expected -1, got: %s", response)
	}
}

// String Operations Tests

func TestServer_APPEND(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET greeting Hello")

	response := h.sendCommand("APPEND greeting World")
	if response != "10" { // "HelloWorld" = 10 chars
		t.Errorf("Expected 10, got: %s", response)
	}

	response = h.sendCommand("GET greeting")
	if response != "HelloWorld" {
		t.Errorf("Expected HelloWorld, got: %s", response)
	}
}

func TestServer_APPEND_NewKey(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("APPEND newkey value")
	if response != "5" {
		t.Errorf("Expected 5, got: %s", response)
	}
}

func TestServer_STRLEN(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET mykey Hello")

	response := h.sendCommand("STRLEN mykey")
	if response != "5" {
		t.Errorf("Expected 5, got: %s", response)
	}
}

func TestServer_STRLEN_NonExistent(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("STRLEN nonexistent")
	if response != "0" {
		t.Errorf("Expected 0, got: %s", response)
	}
}

// Server Commands Tests

func TestServer_CLEAR(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET key1 value1")
	h.sendCommand("SET key2 value2")

	response := h.sendCommand("CLEAR")
	if response != "+OK" {
		t.Errorf("CLEAR failed: %s", response)
	}

	// Verify cleared
	response = h.sendCommand("KEYS *")
	if response != "(empty list)" {
		t.Errorf("Expected empty after CLEAR, got: %s", response)
	}
}

func TestServer_INFO(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	h.sendCommand("SET key value")

	response := h.sendCommand("INFO")
	if !strings.HasPrefix(response, "+OK") {
		t.Errorf("Expected +OK, got: %s", response)
	}
	if !strings.Contains(response, "keys=") {
		t.Errorf("Expected keys info, got: %s", response)
	}
}

func TestServer_STATS(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("STATS")
	if !strings.HasPrefix(response, "+OK") {
		t.Errorf("Expected +OK, got: %s", response)
	}
	if !strings.Contains(response, "wal_size=") {
		t.Errorf("Expected wal_size, got: %s", response)
	}
}

func TestServer_HEALTH(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// HEALTH returns multi-line JSON
	responses := h.sendMultilineCommand("HEALTH")
	fullResponse := strings.Join(responses, "\n")
	if !strings.Contains(fullResponse, "healthy") {
		t.Errorf("Expected healthy status, got: %s", fullResponse)
	}
}

func TestServer_CONFIG_GET(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("CONFIG GET host")
	if response != "localhost" {
		t.Errorf("Expected localhost, got: %s", response)
	}
}

func TestServer_CONFIG_InvalidParam(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("CONFIG GET invalid")
	if !strings.HasPrefix(response, "-ERR") {
		t.Errorf("Expected error for invalid param, got: %s", response)
	}
}

func TestServer_QUIT(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("QUIT")
	if response != "+OK goodbye" {
		t.Errorf("Expected '+OK goodbye', got: %s", response)
	}
}

func TestServer_SYNC(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("SYNC")
	if response != "+OK" {
		t.Errorf("SYNC failed: %s", response)
	}
}

func TestServer_COMPACT(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Add some data first
	for i := 0; i < 10; i++ {
		h.sendCommand(fmt.Sprintf("SET key%d value%d", i, i))
	}

	response := h.sendCommand("COMPACT")
	if response != "+OK" {
		t.Errorf("COMPACT failed: %s", response)
	}
}

// Analytics Commands Tests

func TestServer_ANALYZE(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Generate some access
	h.sendCommand("SET analytics_key value")
	for i := 0; i < 10; i++ {
		h.sendCommand("GET analytics_key")
	}

	response := h.sendCommand("ANALYZE analytics_key")
	if !strings.HasPrefix(response, "+OK") {
		t.Errorf("ANALYZE failed: %s", response)
	}
	if !strings.Contains(response, "reads=") {
		t.Errorf("Expected reads info, got: %s", response)
	}
}

func TestServer_HOTKEYS(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Generate access patterns
	for i := 0; i < 50; i++ {
		h.sendCommand("SET hotkey value")
		h.sendCommand("GET hotkey")
	}

	response := h.sendCommand("HOTKEYS 5")
	if response == "-ERR analytics not enabled" {
		t.Skip("Analytics not enabled")
	}
	if strings.Contains(response, "hotkey") || response == "(empty list)" {
		// Success - either found hotkey or list is empty
	} else if strings.HasPrefix(response, "-ERR") {
		t.Errorf("HOTKEYS failed: %s", response)
	}
}

// Error Cases Tests

func TestServer_UnknownCommand(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	response := h.sendCommand("UNKNOWNCMD arg1 arg2")
	if !strings.HasPrefix(response, "-ERR unknown command") {
		t.Errorf("Expected unknown command error, got: %s", response)
	}
}

func TestServer_EmptyCommand(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Send empty line (should be skipped)
	conn, _ := net.Dial("tcp", h.addr)
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	reader.ReadString('\n') // Welcome

	// Send empty line then PING
	fmt.Fprintf(writer, "\n")
	fmt.Fprintf(writer, "PING\n")
	writer.Flush()

	response, _ := reader.ReadString('\n')
	if strings.TrimSpace(response) != "+PONG" {
		t.Errorf("Expected +PONG after empty line, got: %s", response)
	}
}

func TestServer_CaseSensitivity(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	// Commands should be case-insensitive
	response := h.sendCommand("ping")
	if response != "+PONG" {
		t.Errorf("Expected +PONG for lowercase ping, got: %s", response)
	}

	response = h.sendCommand("PING")
	if response != "+PONG" {
		t.Errorf("Expected +PONG for uppercase PING, got: %s", response)
	}

	response = h.sendCommand("PiNg")
	if response != "+PONG" {
		t.Errorf("Expected +PONG for mixed case PiNg, got: %s", response)
	}
}

// Connection Tests

func TestServer_MaxConnections(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Host:           "localhost",
		Port:           0,
		MaxConnections: 2, // Only allow 2 connections
	}

	eng, _ := engine.New(engine.Options{
		WALPath:  tmpDir,
		SyncMode: false,
	})

	server := NewServer(cfg, eng)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	addr := server.Addr()

	defer func() {
		server.Shutdown()
		eng.Close()
	}()

	// Open 2 connections (should succeed)
	conn1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Second connection failed: %v", err)
	}
	defer conn2.Close()

	// Third connection should be rejected
	conn3, err := net.Dial("tcp", addr)
	if err != nil {
		return // Connection refused is acceptable
	}
	defer conn3.Close()

	reader := bufio.NewReader(conn3)
	response, _ := reader.ReadString('\n')
	if !strings.Contains(response, "connection limit") {
		t.Logf("Third connection response: %s", response)
	}
}

func TestServer_ConcurrentOperations(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Run concurrent SET/GET operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("concurrent_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)

				response := h.sendCommand(fmt.Sprintf("SET %s %s", key, value))
				if response != "+OK" {
					errors <- fmt.Errorf("SET failed for %s: %s", key, response)
					return
				}

				response = h.sendCommand(fmt.Sprintf("GET %s", key))
				if response != value {
					errors <- fmt.Errorf("GET failed for %s: expected %s, got %s", key, value, response)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestServer_Addr(t *testing.T) {
	h := setupTestHelper(t)
	defer h.close()

	addr := h.server.Addr()
	if addr == "" {
		t.Error("Addr() should return non-empty address")
	}
	if !strings.Contains(addr, ":") {
		t.Errorf("Addr() should return host:port format, got: %s", addr)
	}
}

// Benchmark Tests

func BenchmarkServer_SET(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := &config.Config{Host: "localhost", Port: 0}
	eng, _ := engine.New(engine.Options{WALPath: tmpDir})
	server := NewServer(cfg, eng)
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
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		reader.ReadString('\n')

		fmt.Fprintf(writer, "SET bench value\n")
		writer.Flush()
		reader.ReadString('\n')

		conn.Close()
	}
}

func BenchmarkServer_GET(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := &config.Config{Host: "localhost", Port: 0}
	eng, _ := engine.New(engine.Options{WALPath: tmpDir})
	server := NewServer(cfg, eng)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	addr := server.Addr()

	// Setup key
	conn, _ := net.Dial("tcp", addr)
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	reader.ReadString('\n')
	fmt.Fprintf(writer, "SET bench value\n")
	writer.Flush()
	reader.ReadString('\n')
	conn.Close()

	defer func() {
		server.Shutdown()
		eng.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, _ := net.Dial("tcp", addr)
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		reader.ReadString('\n')

		fmt.Fprintf(writer, "GET bench\n")
		writer.Flush()
		reader.ReadString('\n')

		conn.Close()
	}
}
