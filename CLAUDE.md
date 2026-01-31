# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build and Run
- `make build` - Build the kvlite server binary to bin/kvlite.exe
- `make run` - Build and start the kvlite server
- `make build-client` - Build the test client to bin/test_client
- `make client` - Build and run the test client for testing

### Testing
- `make test` - Run all tests
- `make test-race` - Run tests with race detector
- `make test-cover` - Run tests with coverage report (generates coverage.html)
- `make bench` - Run benchmarks for internal/store/
- `make bench-all` - Run all benchmarks including network tests

### Code Quality
- `make fmt` - Format all Go code
- `make vet` - Run go vet
- `make lint` - Run golangci-lint (requires installation)
- `make check` - Run fmt, vet, and test together

### Maintenance
- `make clean` - Remove build artifacts and coverage files
- `make mod-tidy` - Tidy go modules
- `make deps` - Download dependencies

## Architecture Overview

kvlite is a high-performance in-memory key-value store written in Go with the following core architecture:

### Core Components

1. **Engine** (`internal/engine/`) - The central coordinator that orchestrates:
   - In-memory store operations
   - WAL (Write-Ahead Log) for durability
   - Snapshot creation for compaction
   - TTL management for key expiration
   - AI-powered analytics and smart scheduling

2. **Store** (`internal/store/`) - Thread-safe in-memory storage with:
   - Concurrent read/write operations using sync.RWMutex
   - TTL support with lazy expiration
   - Pattern matching for key operations (glob-style: *, ?)
   - Pagination support via SCAN operations

3. **API Server** (`pkg/api/`) - TCP server that:
   - Handles Redis-like protocol commands
   - Manages client connections with configurable limits
   - Processes commands like SET, GET, DELETE, EXPIRE, etc.
   - Supports advanced commands like MSET, MGET, HOTKEYS, ANALYZE

4. **Persistence Layer**:
   - **WAL** (`internal/wal/`) - Write-ahead logging for durability
   - **Snapshot** (`internal/snapshot/`) - Point-in-time snapshots for compaction
   - **TTL Manager** (`internal/ttl/`) - Background expiration of keys

5. **Analytics** (`internal/analytics/`) - AI-powered features:
   - Key access pattern tracking
   - Smart compaction scheduling
   - TTL suggestions based on usage patterns
   - Anomaly detection

### Data Flow

1. **Writes**: Command → Engine → WAL (durability) → Store (in-memory)
2. **Reads**: Command → Engine → Store (with lazy expiration)
3. **Recovery**: Snapshot load → WAL replay → Store reconstruction
4. **Compaction**: Store snapshot → WAL truncation (background or triggered)

### Key Design Patterns

- **WAL-first writes** for durability - all modifications go to WAL before in-memory store
- **Lazy expiration** - TTL keys are expired on access rather than background scanning
- **Smart compaction** - Uses AI analytics to optimize compaction timing based on load patterns
- **Thread-safe operations** - All store operations use appropriate locking mechanisms
- **Graceful shutdown** - Proper resource cleanup and connection handling

## Configuration

The server accepts configuration via:
- Command-line flags (see `cmd/kvlite/main.go` for available options)
- Environment variables (loaded via `internal/config/`)

Key configuration options:
- `--host`, `--port` - Server binding
- `--max-connections` - Connection limits
- `--wal-path` - WAL storage directory
- `--sync-mode` - Sync after every write (slower but safer)
- `--enable-analytics` - Enable AI-powered features

## Protocol

kvlite implements a Redis-compatible TCP protocol with commands like:
- Basic: SET, GET, DELETE, EXISTS
- TTL: SETEX, EXPIRE, TTL, PERSIST
- Pattern: KEYS, SCAN
- Batch: MSET, MGET, MDEL
- Analytics: ANALYZE, HOTKEYS, SUGGEST-TTL, ANOMALIES
- Server: INFO, STATS, HEALTH, COMPACT, SYNC

## Testing

The codebase uses standard Go testing:
- Unit tests are co-located with source files (`*_test.go`)
- Benchmarks test performance of critical paths
- Integration tests use the test client in `scripts/`

## Version

Current version: 0.5.0 (see `cmd/kvlite/main.go`)