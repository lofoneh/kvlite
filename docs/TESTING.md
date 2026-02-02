# Testing Guide

Guide to running and writing tests for kvlite.

## Running Tests

### All Tests

```bash
make test
# or
go test ./...
```

### With Verbose Output

```bash
go test ./... -v
```

### With Race Detector

```bash
make test-race
# or
go test ./... -race
```

### With Coverage

```bash
make test-cover
# or
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Specific Package

```bash
# Test store package
go test ./internal/store/...

# Test API package
go test ./pkg/api/...

# Test with verbose
go test ./internal/engine/... -v
```

### Specific Test

```bash
# Run single test
go test ./internal/store/... -run TestStore_SetGet

# Run tests matching pattern
go test ./... -run ".*TTL.*"
```

## Test Structure

### Unit Tests

Located alongside source files with `_test.go` suffix:

```
internal/
├── store/
│   ├── store.go
│   └── store_test.go      # Unit tests
├── engine/
│   ├── engine.go
│   └── engine_test.go
├── analytics/
│   ├── analytics.go
│   └── analytics_test.go
└── config/
    ├── config.go
    └── config_test.go
```

### Integration Tests

Located in `tests/` directory:

```
tests/
└── integration_test.go    # End-to-end tests
```

### Package Tests

```
pkg/
├── api/
│   ├── api.go
│   └── api_test.go        # Server tests
└── client/
    ├── pool.go
    └── pool_test.go       # Client tests
```

## Running Benchmarks

### All Benchmarks

```bash
make bench
# or
go test ./... -bench=.
```

### Specific Package

```bash
go test ./internal/store/... -bench=.
```

### With Memory Allocation Stats

```bash
go test ./internal/store/... -bench=. -benchmem
```

### Run N Times

```bash
go test ./internal/store/... -bench=. -count=5
```

## Test Packages

### internal/store

Tests for the in-memory store:

- `TestStore_SetGet` - Basic set/get operations
- `TestStore_Delete` - Key deletion
- `TestStore_Len` - Key counting
- `TestStore_Clear` - Clearing all keys
- `TestStore_Concurrent` - Concurrent access
- `TestStore_TTL` - Expiration handling

### internal/engine

Tests for the engine (store + WAL):

- `TestEngine_SetAndGet` - Basic operations through engine
- `TestEngine_Delete` - Deletion with WAL
- `TestEngine_Clear` - Clear with WAL
- `TestEngine_Recovery` - Crash recovery
- `TestEngine_RecoveryWithClear` - Recovery with CLEAR operations
- `TestEngine_MultipleCycles` - Multiple restart cycles

### internal/wal

Tests for Write-Ahead Log:

- `TestRecord_EncodeDecodeValidate` - Record serialization
- `TestRecord_CorruptedChecksum` - Corruption detection
- `TestWAL_WriteAndReplay` - Write and replay operations
- `TestWAL_Truncate` - WAL truncation
- `TestWAL_ReadAll` - Reading all records
- `TestWAL_EmptyReplay` - Empty WAL handling

### internal/snapshot

Tests for snapshots:

- `TestSnapshot_CreateAndLoad` - Snapshot creation/loading
- `TestSnapshot_Atomicity` - Atomic writes
- `TestSnapshot_Exists` - Existence checks
- `TestSnapshot_LargeDataset` - 10k+ keys

### internal/ttl

Tests for TTL manager:

- `TestManager_StartStop` - Lifecycle management
- `TestManager_AutoExpiration` - Automatic expiration
- `TestManager_ForceCheck` - Forced expiration check
- `TestManager_Stats` - Statistics tracking

### internal/analytics

Tests for analytics tracker:

- `TestNewTracker` - Tracker initialization
- `TestRecordRead` / `TestRecordWrite` - Access recording
- `TestGetStats` - Stats retrieval
- `TestCircularBuffer` - Buffer overflow handling
- `TestGetHotKeys` / `TestGetColdKeys` - Key classification
- `TestSuggestTTL` - TTL suggestions
- `TestDetectAnomalies` - Anomaly detection
- `TestConcurrentAccess` - Thread safety

### internal/config

Tests for configuration:

- `TestDefault` - Default values
- `TestLoadFromEnv` - Environment variable loading
- `TestAddress` - Address formatting
- `TestValidate` - Validation rules

### pkg/api

Tests for TCP server:

- `TestServer_PING` - Health check
- `TestServer_SET_GET` - Basic operations
- `TestServer_DELETE` - Deletion
- `TestServer_EXISTS` - Existence check
- `TestServer_SETEX` / `TestServer_EXPIRE` - TTL commands
- `TestServer_KEYS` / `TestServer_SCAN` - Key listing
- `TestServer_MSET_MGET` - Batch operations
- `TestServer_INCR_DECR` - Counters
- `TestServer_MaxConnections` - Connection limits
- `TestServer_ConcurrentOperations` - Concurrency

### pkg/client

Tests for connection pool:

- `TestNewPool` - Pool initialization
- `TestPool_GetAndPut` - Connection lifecycle
- `TestPool_Stats` - Pool statistics
- `TestPool_MaxActive` - Connection limits
- `TestPool_Close` - Pool shutdown
- `TestClient_SetGet` - High-level operations
- `TestClient_ConcurrentOperations` - Concurrency

### tests (integration)

End-to-end tests:

- `TestIntegration_BasicOperations` - SET/GET/DELETE
- `TestIntegration_TTL` - Expiration flow
- `TestIntegration_ConcurrentClients` - Multiple clients
- `TestIntegration_Persistence` - Restart recovery
- `TestIntegration_Analytics` - Analytics commands

## Writing Tests

### Test File Template

```go
package mypackage

import (
    "testing"
)

func TestMyFunction(t *testing.T) {
    // Setup
    // ...

    // Test
    result := MyFunction()

    // Assert
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Table-Driven Tests

```go
func TestValidate(t *testing.T) {
    testCases := []struct {
        name    string
        input   int
        wantErr bool
    }{
        {"valid", 100, false},
        {"zero", 0, true},
        {"negative", -1, true},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := Validate(tc.input)
            if (err != nil) != tc.wantErr {
                t.Errorf("Validate(%d) error = %v, wantErr %v",
                    tc.input, err, tc.wantErr)
            }
        })
    }
}
```

### Benchmark Template

```go
func BenchmarkMyFunction(b *testing.B) {
    // Setup (not timed)
    data := setupTestData()

    b.ResetTimer() // Start timing

    for i := 0; i < b.N; i++ {
        MyFunction(data)
    }
}
```

### Test with Temp Directory

```go
func TestWithTempDir(t *testing.T) {
    tmpDir := t.TempDir() // Automatically cleaned up

    // Use tmpDir for WAL, snapshots, etc.
    engine, _ := engine.New(engine.Options{
        WALPath: tmpDir,
    })
    defer engine.Close()

    // Test...
}
```

### Concurrent Test

```go
func TestConcurrent(t *testing.T) {
    var wg sync.WaitGroup
    errors := make(chan error, 100)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            // Concurrent operations...
        }(i)
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        t.Error(err)
    }
}
```

## Test Coverage Goals

| Package | Target Coverage |
|---------|-----------------|
| internal/store | 90%+ |
| internal/engine | 80%+ |
| internal/wal | 85%+ |
| internal/snapshot | 80%+ |
| internal/ttl | 80%+ |
| internal/analytics | 85%+ |
| internal/config | 90%+ |
| pkg/api | 75%+ |
| pkg/client | 75%+ |

## CI Integration

Tests run automatically on:
- Pull requests
- Push to main/development
- Scheduled (nightly)

```yaml
# Example GitHub Actions
- name: Test
  run: go test ./... -race -coverprofile=coverage.out

- name: Upload coverage
  uses: codecov/codecov-action@v3
```

## Troubleshooting Tests

### Port Already in Use

Tests use random ports (port 0) to avoid conflicts. If you see port errors, ensure no kvlite server is running.

### Flaky Tests

Some tests involve timing (TTL expiration). If tests flake:
- Increase timeouts
- Use shorter TTL values in tests
- Add retry logic for timing-sensitive assertions

### Race Conditions

Run with `-race` flag to detect races:
```bash
go test ./... -race
```

### Memory Issues

For memory profiling:
```bash
go test ./... -memprofile=mem.out
go tool pprof mem.out
```
