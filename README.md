# kvlite

A fast, lightweight in-memory key-value store written in Go with persistence, TTL support, and AI-powered analytics.

## Features

- **High Performance**: Sub-microsecond GET operations, ~45M ops/sec
- **Persistence**: Write-Ahead Log (WAL) with crash recovery
- **TTL Support**: Per-key expiration with lazy and background cleanup
- **Snapshots**: Point-in-time snapshots for efficient compaction
- **Analytics**: Track access patterns, detect hot keys, suggest TTLs
- **Redis-like Protocol**: Familiar command syntax over TCP
- **Zero Dependencies**: Pure Go standard library

## Quick Start

```bash
# Build
make build

# Run server
./bin/kvlite

# Connect with netcat or telnet
nc localhost 6380
```

```
+OK kvlite ready
SET greeting "Hello, World!"
+OK
GET greeting
Hello, World!
```

## Installation

### From Source

```bash
git clone https://github.com/lofoneh/kvlite.git
cd kvlite
make build
```

### Docker

```bash
docker build -t kvlite .
docker run -p 6380:6380 kvlite
```

## Commands

### Basic Operations

| Command | Description | Example |
|---------|-------------|---------|
| `SET key value` | Store a value | `SET name Alice` |
| `GET key` | Retrieve a value | `GET name` |
| `DELETE key` | Remove a key | `DELETE name` |
| `EXISTS key` | Check if key exists | `EXISTS name` |
| `KEYS pattern` | List keys matching pattern | `KEYS user:*` |

### TTL Operations

| Command | Description | Example |
|---------|-------------|---------|
| `SETEX key seconds value` | Set with expiration | `SETEX session 3600 data` |
| `EXPIRE key seconds` | Set TTL on existing key | `EXPIRE name 60` |
| `TTL key` | Get remaining TTL | `TTL session` |
| `PERSIST key` | Remove TTL | `PERSIST session` |

### Batch Operations

| Command | Description | Example |
|---------|-------------|---------|
| `MSET k1 v1 k2 v2` | Set multiple keys | `MSET a 1 b 2 c 3` |
| `MGET k1 k2 k3` | Get multiple keys | `MGET a b c` |
| `MDEL k1 k2 k3` | Delete multiple keys | `MDEL a b c` |

### Counter Operations

| Command | Description | Example |
|---------|-------------|---------|
| `INCR key` | Increment by 1 | `INCR counter` |
| `DECR key` | Decrement by 1 | `DECR counter` |

### String Operations

| Command | Description | Example |
|---------|-------------|---------|
| `APPEND key value` | Append to string | `APPEND msg " world"` |
| `STRLEN key` | Get string length | `STRLEN msg` |

### Server Operations

| Command | Description |
|---------|-------------|
| `PING` | Health check (returns PONG) |
| `INFO` | Server information |
| `STATS` | Detailed statistics |
| `HEALTH` | Health check (JSON) |
| `SYNC` | Force WAL flush |
| `COMPACT` | Force compaction |
| `CLEAR` | Delete all keys |
| `QUIT` | Close connection |

### Analytics Operations

| Command | Description | Example |
|---------|-------------|---------|
| `ANALYZE key` | Get key statistics | `ANALYZE popular_key` |
| `HOTKEYS n` | Get top N hot keys | `HOTKEYS 10` |
| `SUGGEST-TTL key` | Get TTL suggestion | `SUGGEST-TTL cache_key` |
| `ANOMALIES` | Detect unusual patterns | `ANOMALIES` |

## Configuration

### Command Line Flags

```bash
./bin/kvlite \
  --host 0.0.0.0 \
  --port 6380 \
  --max-connections 1000 \
  --wal-path ./data \
  --sync-mode \
  --enable-analytics
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `KVLITE_HOST` | Bind address | `localhost` |
| `KVLITE_PORT` | Listen port | `6380` |
| `KVLITE_MAX_CONNECTIONS` | Connection limit (0=unlimited) | `0` |

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    TCP Server                        │
│                   (pkg/api/)                         │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│                     Engine                           │
│                (internal/engine/)                    │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌────────┐ │
│  │  Store  │  │   WAL   │  │Snapshot │  │Analytics│ │
│  └─────────┘  └─────────┘  └─────────┘  └────────┘ │
└─────────────────────────────────────────────────────┘
```

### Data Flow

1. **Writes**: Client → API → Engine → WAL → Store
2. **Reads**: Client → API → Engine → Store (with lazy expiration)
3. **Recovery**: Snapshot → WAL Replay → Store

## Examples

See the [examples/](examples/) directory for usage patterns:

- **[caching/](examples/caching/)** - Cache-aside and write-through patterns
- **[sessions/](examples/sessions/)** - Session management with TTL
- **[rate_limiting/](examples/rate_limiting/)** - Token bucket, leaky bucket
- **[locks/](examples/locks/)** - Distributed locking
- **[counters/](examples/counters/)** - Atomic counters and analytics

## Development

```bash
# Run tests
make test

# Run tests with race detector
make test-race

# Run benchmarks
make bench

# Format code
make fmt

# Run linter
make lint

# Full check (fmt + vet + test)
make check
```

## Benchmarks

```
BenchmarkStore_Set-8        50000000    25.3 ns/op    0 B/op    0 allocs/op
BenchmarkStore_Get-8        100000000   10.2 ns/op    0 B/op    0 allocs/op
BenchmarkEngine_Set-8       500000      2145 ns/op    48 B/op   1 allocs/op
BenchmarkEngine_Get-8       50000000    28.4 ns/op    0 B/op    0 allocs/op
```

## Documentation

- [Quick Start Guide](docs/QUICKSTART.md)
- [API Reference](docs/API_REFERENCE.md)
- [Testing Guide](docs/TESTING.md)
- [Changelog](docs/CHANGELOG.md)

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read the contributing guidelines and submit pull requests.
