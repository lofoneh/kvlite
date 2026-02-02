# Quick Start Guide

Get up and running with kvlite in under 5 minutes.

## Prerequisites

- Go 1.21 or later
- Make (optional, for build commands)

## Installation

### Option 1: Build from Source

```bash
# Clone the repository
git clone https://github.com/lofoneh/kvlite.git
cd kvlite

# Build the server
make build
# or: go build -o bin/kvlite ./cmd/kvlite

# Run the server
./bin/kvlite
```

### Option 2: Docker

```bash
# Build image
docker build -t kvlite .

# Run container
docker run -p 6380:6380 kvlite
```

## Connect to the Server

### Using netcat

```bash
nc localhost 6380
```

### Using telnet

```bash
telnet localhost 6380
```

### Using the Go client

```go
package main

import (
    "fmt"
    "github.com/lofoneh/kvlite/pkg/client"
)

func main() {
    c, _ := client.NewClient("localhost:6380")
    defer c.Close()

    c.Set("greeting", "Hello, World!")
    value, _ := c.Get("greeting")
    fmt.Println(value) // Hello, World!
}
```

## Basic Operations

Once connected, you'll see:
```
+OK kvlite ready
```

### Store and Retrieve Data

```
SET name Alice
+OK

GET name
Alice
```

### Delete Data

```
DELETE name
+OK

GET name
-ERR key not found
```

### Check if Key Exists

```
SET active true
+OK

EXISTS active
1

EXISTS missing
0
```

## Working with TTL (Time-To-Live)

### Set Key with Expiration

```
SETEX session 60 user123
+OK
```

This key will automatically expire after 60 seconds.

### Check Remaining TTL

```
TTL session
58
```

### Add TTL to Existing Key

```
SET permanent value
+OK

EXPIRE permanent 120
1

TTL permanent
119
```

### Remove TTL

```
PERSIST permanent
1

TTL permanent
-1
```

## Batch Operations

### Set Multiple Keys

```
MSET user:1 Alice user:2 Bob user:3 Charlie
+OK
```

### Get Multiple Keys

```
MGET user:1 user:2 user:3
Alice
Bob
Charlie
```

### Delete Multiple Keys

```
MDEL user:1 user:2 user:3
3
```

## Counters

### Increment

```
INCR pageviews
1

INCR pageviews
2

INCR pageviews
3
```

### Decrement

```
DECR stock
-1

SET stock 100
+OK

DECR stock
99
```

## Pattern Matching

### List All Keys

```
KEYS *
user:1
user:2
config:app
```

### Match Pattern

```
KEYS user:*
user:1
user:2
```

## Server Commands

### Health Check

```
PING
+PONG
```

### Server Info

```
INFO
+OK keys=5 connections=1 wal_size=1024
```

### Detailed Statistics

```
STATS
+OK keys=5 connections=1 wal_size=1024 wal_entries=10 ...
```

## Analytics (if enabled)

### Analyze Key Access

```
ANALYZE popular_key
+OK reads=150 writes=10 last_access=2024-01-15T10:30:00Z
```

### Find Hot Keys

```
HOTKEYS 5
popular_key (reads=150 writes=10)
another_key (reads=100 writes=5)
```

## Configuration Options

### Start with Custom Port

```bash
./bin/kvlite --port 7000
```

### Enable Analytics

```bash
./bin/kvlite --enable-analytics
```

### Custom Data Directory

```bash
./bin/kvlite --wal-path /var/data/kvlite
```

### All Options

```bash
./bin/kvlite --help
```

## Next Steps

- Read the [API Reference](API_REFERENCE.md) for all commands
- Check out [examples/](../examples/) for usage patterns
- See [TESTING.md](TESTING.md) for running tests

## Troubleshooting

### Connection Refused

Make sure the server is running:
```bash
./bin/kvlite
```

### Port Already in Use

Use a different port:
```bash
./bin/kvlite --port 6381
```

### Data Not Persisting

Check if the WAL directory is writable:
```bash
ls -la ./data/
```
