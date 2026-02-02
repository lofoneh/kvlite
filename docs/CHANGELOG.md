# Changelog

All notable changes to kvlite will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.6.0] - Unreleased

### Added
- Comprehensive test suite for all packages
- Example use cases (caching, sessions, rate limiting, locks, counters)
- Complete documentation (README, QUICKSTART, API Reference, Testing Guide)

### Fixed
- Integration test port validation (allow port 0 for random assignment)
- Test timeout issues with server shutdown
- Example build error (redundant newline)

---

## [0.5.0] - Analytics & Smart Scheduling

### Added
- **AI-Powered Analytics** (`internal/analytics/`)
  - Key access pattern tracking (reads, writes, timestamps)
  - Hot key detection with configurable thresholds
  - Cold key identification for cleanup optimization
  - TTL suggestions based on access patterns
  - Anomaly detection for unusual access behavior
  - Circular buffer for efficient history storage

- **Smart Compaction Scheduling**
  - Load-aware compaction timing
  - Avoids compaction during peak usage
  - Configurable scheduling parameters

- **New Commands**
  - `ANALYZE key` - Get detailed key statistics
  - `HOTKEYS n` - List top N most accessed keys
  - `SUGGEST-TTL key` - Get TTL recommendation
  - `ANOMALIES` - Detect unusual access patterns

- **Configuration**
  - `--enable-analytics` flag to activate analytics
  - Analytics data included in `STATS` output

---

## [0.4.0] - TTL Support

### Added
- **TTL Manager** (`internal/ttl/`)
  - Per-key expiration times
  - Background expiration worker
  - Lazy expiration on access
  - Configurable check intervals
  - Expiration statistics tracking

- **New Commands**
  - `SETEX key seconds value` - Set with expiration
  - `EXPIRE key seconds` - Add TTL to existing key
  - `TTL key` - Get remaining time-to-live
  - `PERSIST key` - Remove TTL from key

- **TTL Persistence**
  - TTL values stored in WAL
  - Survives server restarts
  - Automatic cleanup of expired keys on recovery

### Changed
- Store entries now include optional expiration timestamp
- Engine coordinates TTL manager lifecycle

---

## [0.3.0] - Snapshots & Compaction

### Added
- **Snapshot System** (`internal/snapshot/`)
  - Point-in-time snapshots of entire store
  - JSON-based snapshot format
  - Atomic writes with temp file rename
  - Automatic snapshot on compaction

- **WAL Compaction**
  - Automatic compaction when thresholds exceeded
  - Manual compaction via `COMPACT` command
  - Configurable size and entry limits
  - Background compaction worker

- **New Commands**
  - `COMPACT` - Force WAL compaction
  - `STATS` - Detailed server statistics

- **Recovery Improvements**
  - Load from snapshot first
  - Replay WAL entries after snapshot
  - Faster recovery for large datasets

### Changed
- Recovery now uses snapshot + WAL replay strategy
- Added compaction statistics to server info

---

## [0.2.0] - Persistence Layer

### Added
- **Write-Ahead Log (WAL)** (`internal/wal/`)
  - Append-only log format
  - CRC32 checksums for data integrity
  - Automatic corruption detection
  - Configurable file location

- **Engine Layer** (`internal/engine/`)
  - Coordinates in-memory store and WAL
  - WAL-first writes for durability
  - Automatic crash recovery on startup
  - Replays WAL to restore state

- **New Commands**
  - `SYNC` - Force WAL flush to disk
  - `INFO` - Now includes WAL size

- **Configuration**
  - `--wal-path` flag for WAL directory
  - `--sync-mode` flag for synchronous writes
  - `KVLITE_WAL_PATH` environment variable

### Changed
- API server now uses Engine instead of Store directly
- DELETE and CLEAR commands return errors on failure

### Performance
- SET: ~2µs/op (includes WAL write, async mode)
- GET: ~25ns/op (unchanged, memory-only)
- Recovery: ~10,000 ops/sec

---

## [0.1.0] - Initial Release

### Added
- **Thread-Safe In-Memory Store** (`internal/store/`)
  - `sync.RWMutex` for concurrent access
  - Zero-allocation operations
  - GET, SET, DELETE, CLEAR operations

- **TCP Server** (`pkg/api/`)
  - Text-based protocol (Redis-like)
  - Concurrent client handling
  - Graceful shutdown support
  - Connection limiting

- **Commands**
  - `SET key value` - Store a key-value pair
  - `GET key` - Retrieve a value
  - `DELETE key` - Remove a key
  - `EXISTS key` - Check if key exists
  - `CLEAR` - Remove all keys
  - `KEYS pattern` - List keys matching pattern
  - `INFO` - Server statistics
  - `PING` - Health check
  - `QUIT` - Close connection

- **Configuration** (`internal/config/`)
  - Command-line flags: `--host`, `--port`, `--max-connections`
  - Environment variables support
  - Validation and defaults

- **Build & Deployment**
  - Makefile with common tasks
  - Dockerfile for containerization
  - docker-compose.yml for deployment

- **Testing**
  - Comprehensive unit tests
  - Concurrent access tests
  - Benchmark suite

### Performance
- SET: ~40ns/op (25M ops/sec)
- GET: ~22ns/op (45M ops/sec)
- Concurrent: ~85ns/op (12M ops/sec)
- Zero allocations per operation

---

## Roadmap

### Completed
- [x] Core in-memory store with TCP server
- [x] Persistence with Write-Ahead Log (WAL)
- [x] WAL compaction and snapshots
- [x] TTL (time-to-live) support
- [x] Analytics and smart scheduling
- [x] Comprehensive tests and documentation

### Future
- [ ] Authentication (AUTH command)
- [ ] TLS/SSL support
- [ ] Pub/Sub messaging
- [ ] Lua scripting (EVAL)
- [ ] Cluster mode / replication
- [ ] Data structures (lists, sets, hashes)

---

## Contributors
- Jeffery Asamani (@lofoneh)

## License
MIT License - See LICENSE file for details
