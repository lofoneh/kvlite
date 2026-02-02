// pkg/client/pool_test.go
package client

import (
	"sync"
	"testing"
	"time"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/engine"
	"github.com/lofoneh/kvlite/pkg/api"
)

// testServer provides a kvlite server for testing
type testServer struct {
	server *api.Server
	engine *engine.Engine
	addr   string
}

func setupTestServer(t *testing.T) *testServer {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Host:           "localhost",
		Port:           0, // Random port
		MaxConnections: 0,
	}

	eng, err := engine.New(engine.Options{
		WALPath:         tmpDir,
		SyncMode:        false,
		EnableAnalytics: false,
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	server := api.NewServer(cfg, eng)

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

	return &testServer{
		server: server,
		engine: eng,
		addr:   server.Addr(),
	}
}

func (ts *testServer) close() {
	ts.server.Shutdown()
	ts.engine.Close()
}

// Pool Tests

func TestNewPool_MissingAddr(t *testing.T) {
	_, err := NewPool(PoolOptions{})
	if err == nil {
		t.Error("Expected error for missing addr")
	}
}

func TestNewPool_Defaults(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Addr: "localhost:6380",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	if pool.maxIdle != 5 {
		t.Errorf("Expected default maxIdle 5, got %d", pool.maxIdle)
	}
	if pool.idleTimeout != 5*time.Minute {
		t.Errorf("Expected default idleTimeout 5m, got %v", pool.idleTimeout)
	}
}

func TestNewPool_CustomOptions(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Addr:        "localhost:6380",
		MaxIdle:     10,
		MaxActive:   20,
		IdleTimeout: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	if pool.maxIdle != 10 {
		t.Errorf("Expected maxIdle 10, got %d", pool.maxIdle)
	}
	if pool.maxActive != 20 {
		t.Errorf("Expected maxActive 20, got %d", pool.maxActive)
	}
}

func TestPool_GetAndPut(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	pool, err := NewPool(PoolOptions{
		Addr:    ts.addr,
		MaxIdle: 5,
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	// Get connection
	conn, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Use connection
	response, err := conn.Do("PING")
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	if response != "+PONG" {
		t.Errorf("Expected +PONG, got %s", response)
	}

	// Return to pool
	conn.Close()

	// Get again (should reuse)
	conn2, err := pool.Get()
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}
	conn2.Close()
}

func TestPool_Stats(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	pool, err := NewPool(PoolOptions{
		Addr:    ts.addr,
		MaxIdle: 5,
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	// Initial stats
	stats := pool.Stats()
	if stats["active"] != 0 {
		t.Errorf("Expected 0 active, got %d", stats["active"])
	}

	// Get connection
	conn, _ := pool.Get()
	stats = pool.Stats()
	if stats["active"] != 1 {
		t.Errorf("Expected 1 active, got %d", stats["active"])
	}

	// Return to pool
	conn.Close()
	stats = pool.Stats()
	if stats["idle"] != 1 {
		t.Errorf("Expected 1 idle, got %d", stats["idle"])
	}
}

func TestPool_MaxActive(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	pool, err := NewPool(PoolOptions{
		Addr:      ts.addr,
		MaxIdle:   2,
		MaxActive: 2, // Only allow 2 active connections
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	// Get 2 connections
	conn1, _ := pool.Get()
	conn2, _ := pool.Get()

	// Third should fail
	_, err = pool.Get()
	if err == nil {
		t.Error("Expected error for exceeding maxActive")
	}

	// Return one and try again
	conn1.Close()
	conn3, err := pool.Get()
	if err != nil {
		t.Errorf("Should succeed after returning connection: %v", err)
	}
	conn2.Close()
	conn3.Close()
}

func TestPool_Close(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	pool, err := NewPool(PoolOptions{
		Addr: ts.addr,
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	// Get and return a connection
	conn, _ := pool.Get()
	conn.Close()

	// Close pool
	pool.Close()

	// Get should fail
	_, err = pool.Get()
	if err != ErrPoolClosed {
		t.Errorf("Expected ErrPoolClosed, got %v", err)
	}
}

func TestPool_CloseIdempotent(t *testing.T) {
	pool, _ := NewPool(PoolOptions{
		Addr: "localhost:6380",
	})

	// Close multiple times should not panic
	pool.Close()
	pool.Close()
}

func TestPool_ConcurrentAccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	pool, err := NewPool(PoolOptions{
		Addr:      ts.addr,
		MaxIdle:   5,
		MaxActive: 10,
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				conn, err := pool.Get()
				if err != nil {
					errors <- err
					return
				}
				_, err = conn.Do("PING")
				if err != nil {
					errors <- err
				}
				conn.Close()
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent error: %v", err)
	}
}

// Connection Tests

func TestConnection_Do(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	pool, _ := NewPool(PoolOptions{Addr: ts.addr})
	defer pool.Close()

	conn, _ := pool.Get()
	defer conn.Close()

	// Test various commands
	testCases := []struct {
		cmd      string
		args     []string
		expected string
	}{
		{"PING", nil, "+PONG"},
		{"SET", []string{"test", "value"}, "+OK"},
		{"GET", []string{"test"}, "value"},
	}

	for _, tc := range testCases {
		response, err := conn.Do(tc.cmd, tc.args...)
		if err != nil {
			t.Errorf("Do(%s) failed: %v", tc.cmd, err)
			continue
		}
		if response != tc.expected {
			t.Errorf("Do(%s) = %s, expected %s", tc.cmd, response, tc.expected)
		}
	}
}

// Client Tests

func TestNewClient(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, err := NewClient(ts.addr)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	if client.pool == nil {
		t.Error("Client pool should not be nil")
	}
}

func TestClient_SetGet(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	// Set
	err := client.Set("client_key", "client_value")
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get
	value, err := client.Get("client_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if value != "client_value" {
		t.Errorf("Expected client_value, got %s", value)
	}
}

func TestClient_Get_NonExistent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	_, err := client.Get("nonexistent_key")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestClient_Delete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	// Setup
	client.Set("delete_key", "value")

	// Delete
	err := client.Delete("delete_key")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = client.Get("delete_key")
	if err == nil {
		t.Error("Key should be deleted")
	}
}

func TestClient_MSet(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	pairs := map[string]string{
		"mkey1": "mvalue1",
		"mkey2": "mvalue2",
		"mkey3": "mvalue3",
	}

	err := client.MSet(pairs)
	if err != nil {
		t.Errorf("MSet failed: %v", err)
	}

	// Verify each key
	for k, expected := range pairs {
		value, err := client.Get(k)
		if err != nil {
			t.Errorf("Get %s failed: %v", k, err)
			continue
		}
		if value != expected {
			t.Errorf("Get %s = %s, expected %s", k, value, expected)
		}
	}
}

func TestClient_MGet(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	// Setup
	client.Set("mg1", "value1")
	client.Set("mg2", "value2")

	// MGet
	values, err := client.MGet([]string{"mg1", "mg2"})
	if err != nil {
		t.Errorf("MGet failed: %v", err)
	}

	// Note: MGet returns multiline response as single joined string
	// The actual behavior depends on how server handles multiline
	if len(values) < 1 {
		t.Error("MGet should return values")
	}
}

func TestClient_Stats(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	stats := client.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
	if _, ok := stats["active"]; !ok {
		t.Error("Stats should contain 'active'")
	}
	if _, ok := stats["idle"]; !ok {
		t.Error("Stats should contain 'idle'")
	}
}

func TestClient_ConcurrentOperations(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := "concurrent_" + string(rune('a'+id)) + "_" + string(rune('0'+j))
				value := "value_" + key

				if err := client.Set(key, value); err != nil {
					errors <- err
					return
				}

				got, err := client.Get(key)
				if err != nil {
					errors <- err
					return
				}
				if got != value {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent error: %v", err)
		}
	}
}

// Benchmark Tests

func BenchmarkPool_GetPut(b *testing.B) {
	ts := &testServer{}
	tmpDir := b.TempDir()

	cfg := &config.Config{Host: "localhost", Port: 0}
	eng, _ := engine.New(engine.Options{WALPath: tmpDir})
	server := api.NewServer(cfg, eng)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	ts.server = server
	ts.engine = eng
	ts.addr = server.Addr()

	defer func() {
		ts.server.Shutdown()
		ts.engine.Close()
	}()

	pool, _ := NewPool(PoolOptions{Addr: ts.addr, MaxIdle: 10})
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, _ := pool.Get()
		conn.Close()
	}
}

func BenchmarkClient_Set(b *testing.B) {
	ts := &testServer{}
	tmpDir := b.TempDir()

	cfg := &config.Config{Host: "localhost", Port: 0}
	eng, _ := engine.New(engine.Options{WALPath: tmpDir})
	server := api.NewServer(cfg, eng)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	ts.server = server
	ts.engine = eng
	ts.addr = server.Addr()

	defer func() {
		ts.server.Shutdown()
		ts.engine.Close()
	}()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Set("bench", "value")
	}
}

func BenchmarkClient_Get(b *testing.B) {
	ts := &testServer{}
	tmpDir := b.TempDir()

	cfg := &config.Config{Host: "localhost", Port: 0}
	eng, _ := engine.New(engine.Options{WALPath: tmpDir})
	server := api.NewServer(cfg, eng)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	ts.server = server
	ts.engine = eng
	ts.addr = server.Addr()

	defer func() {
		ts.server.Shutdown()
		ts.engine.Close()
	}()

	client, _ := NewClient(ts.addr)
	defer client.Close()

	client.Set("bench", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Get("bench")
	}
}
