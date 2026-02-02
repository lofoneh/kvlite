# API Reference

Complete reference for all kvlite commands.

## Connection

kvlite uses a simple text-based protocol over TCP. Connect using any TCP client:

```bash
nc localhost 6380
telnet localhost 6380
```

### Protocol Format

- Commands are sent as single lines terminated by `\n`
- Responses are also line-terminated
- Success responses start with `+` (e.g., `+OK`)
- Error responses start with `-ERR`
- Integer responses are plain numbers (e.g., `1`, `0`, `-1`)

---

## String Commands

### SET

Store a key-value pair.

```
SET key value
```

**Arguments:**
- `key` - The key name
- `value` - The value to store (can contain spaces)

**Returns:** `+OK` on success

**Example:**
```
SET greeting Hello World
+OK
```

---

### GET

Retrieve a value by key.

```
GET key
```

**Arguments:**
- `key` - The key to retrieve

**Returns:**
- The value if key exists
- `-ERR key not found` if key doesn't exist

**Example:**
```
GET greeting
Hello World

GET missing
-ERR key not found
```

---

### DELETE / DEL

Remove a key.

```
DELETE key
DEL key
```

**Arguments:**
- `key` - The key to delete

**Returns:**
- `+OK` if key was deleted
- `-ERR key not found` if key doesn't exist

**Example:**
```
DELETE greeting
+OK
```

---

### EXISTS

Check if a key exists.

```
EXISTS key
```

**Arguments:**
- `key` - The key to check

**Returns:**
- `1` if key exists
- `0` if key doesn't exist

**Example:**
```
EXISTS greeting
1

EXISTS missing
0
```

---

### APPEND

Append a value to an existing string.

```
APPEND key value
```

**Arguments:**
- `key` - The key to append to
- `value` - The value to append

**Returns:** The length of the string after appending

**Example:**
```
SET msg Hello
+OK

APPEND msg " World"
11

GET msg
Hello World
```

---

### STRLEN

Get the length of a string value.

```
STRLEN key
```

**Arguments:**
- `key` - The key to check

**Returns:**
- The string length
- `0` if key doesn't exist

**Example:**
```
SET name Alice
+OK

STRLEN name
5

STRLEN missing
0
```

---

## Counter Commands

### INCR

Increment a counter by 1.

```
INCR key
```

**Arguments:**
- `key` - The key to increment (creates with value 1 if doesn't exist)

**Returns:** The new value after incrementing

**Errors:** `-ERR value is not an integer` if value isn't numeric

**Example:**
```
INCR counter
1

INCR counter
2

SET text hello
+OK

INCR text
-ERR value is not an integer
```

---

### DECR

Decrement a counter by 1.

```
DECR key
```

**Arguments:**
- `key` - The key to decrement (creates with value -1 if doesn't exist)

**Returns:** The new value after decrementing

**Errors:** `-ERR value is not an integer` if value isn't numeric

**Example:**
```
SET stock 100
+OK

DECR stock
99
```

---

## TTL Commands

### SETEX

Set a key with an expiration time.

```
SETEX key seconds value
```

**Arguments:**
- `key` - The key name
- `seconds` - TTL in seconds (must be positive)
- `value` - The value to store

**Returns:** `+OK` on success

**Errors:** `-ERR invalid TTL` if seconds is not a positive integer

**Example:**
```
SETEX session 3600 user123
+OK
```

---

### EXPIRE

Set a TTL on an existing key.

```
EXPIRE key seconds
```

**Arguments:**
- `key` - The key to set TTL on
- `seconds` - TTL in seconds (must be positive)

**Returns:**
- `1` if TTL was set
- `0` if key doesn't exist

**Example:**
```
SET name Alice
+OK

EXPIRE name 60
1

EXPIRE missing 60
0
```

---

### TTL

Get the remaining time-to-live for a key.

```
TTL key
```

**Arguments:**
- `key` - The key to check

**Returns:**
- Remaining seconds if TTL is set
- `-1` if key exists but has no TTL
- `-2` if key doesn't exist

**Example:**
```
SETEX temp 60 value
+OK

TTL temp
58

SET permanent value
+OK

TTL permanent
-1

TTL missing
-2
```

---

### PERSIST

Remove the TTL from a key.

```
PERSIST key
```

**Arguments:**
- `key` - The key to persist

**Returns:**
- `1` if TTL was removed
- `0` if key doesn't exist or has no TTL

**Example:**
```
SETEX temp 60 value
+OK

PERSIST temp
1

TTL temp
-1
```

---

## Batch Commands

### MSET

Set multiple key-value pairs atomically.

```
MSET key1 value1 key2 value2 ...
```

**Arguments:**
- Key-value pairs (must be even number of arguments)

**Returns:** `+OK` on success

**Errors:** `-ERR MSET requires key value pairs`

**Example:**
```
MSET a 1 b 2 c 3
+OK
```

---

### MGET

Get multiple values.

```
MGET key1 key2 key3 ...
```

**Arguments:**
- One or more keys to retrieve

**Returns:** Values separated by newlines, `(nil)` for missing keys

**Example:**
```
MSET a 1 b 2
+OK

MGET a b c
1
2
(nil)
```

---

### MDEL

Delete multiple keys.

```
MDEL key1 key2 key3 ...
```

**Arguments:**
- One or more keys to delete

**Returns:** Number of keys deleted

**Example:**
```
MSET a 1 b 2 c 3
+OK

MDEL a b d
2
```

---

## Key Commands

### KEYS

List keys matching a pattern.

```
KEYS pattern
```

**Arguments:**
- `pattern` - Glob pattern (`*` matches any, `?` matches one char)

**Returns:**
- Keys separated by newlines
- `(empty list)` if no matches

**Example:**
```
MSET user:1 a user:2 b config:app c
+OK

KEYS *
user:1
user:2
config:app

KEYS user:*
user:1
user:2

KEYS config:*
config:app
```

---

### SCAN

Iterate keys with cursor-based pagination.

```
SCAN cursor [MATCH pattern] [COUNT count]
```

**Arguments:**
- `cursor` - Start position (use 0 for first call)
- `MATCH pattern` - Optional glob pattern filter
- `COUNT count` - Optional hint for number of results

**Returns:**
- First line: next cursor (0 means iteration complete)
- Following lines: matching keys

**Example:**
```
SCAN 0 MATCH user:* COUNT 10
5
user:1
user:2

SCAN 5 MATCH user:* COUNT 10
0
user:3
```

---

### CLEAR

Delete all keys.

```
CLEAR
```

**Returns:** `+OK` on success

**Example:**
```
MSET a 1 b 2 c 3
+OK

CLEAR
+OK

KEYS *
(empty list)
```

---

## Server Commands

### PING

Health check.

```
PING
```

**Returns:** `+PONG`

---

### INFO

Get server information.

```
INFO
```

**Returns:** Server stats in format: `+OK keys=N connections=N wal_size=N`

**Example:**
```
INFO
+OK keys=5 connections=1 wal_size=2048
```

---

### STATS

Get detailed statistics.

```
STATS
```

**Returns:** Extended stats including WAL entries, TTL info, analytics

**Example:**
```
STATS
+OK keys=5 connections=1 wal_size=2048 wal_entries=15 needs_compaction=false ttl_expired=3 ttl_checks=100
```

---

### HEALTH

Get health status as JSON.

```
HEALTH
```

**Returns:** JSON health object

**Example:**
```
HEALTH
{
  "status": "healthy",
  "keys": 5,
  "connections": 1,
  "wal_size": 2048,
  "wal_healthy": true
}
```

---

### SYNC

Force WAL flush to disk.

```
SYNC
```

**Returns:** `+OK` on success

---

### COMPACT

Force WAL compaction (creates snapshot).

```
COMPACT
```

**Returns:** `+OK` on success

---

### CONFIG GET

Get configuration value.

```
CONFIG GET parameter
```

**Arguments:**
- `parameter` - One of: `host`, `port`, `max_connections`

**Returns:** The configuration value

**Example:**
```
CONFIG GET port
6380

CONFIG GET host
localhost
```

---

### QUIT

Close the connection.

```
QUIT
```

**Returns:** `+OK goodbye`

---

## Analytics Commands

*Requires `--enable-analytics` flag*

### ANALYZE

Get access statistics for a key.

```
ANALYZE key
```

**Arguments:**
- `key` - The key to analyze

**Returns:** Stats including reads, writes, timestamps

**Example:**
```
ANALYZE popular_key
+OK reads=150 writes=10 last_access=2024-01-15T10:30:00Z created=2024-01-01T00:00:00Z
```

---

### HOTKEYS

Get the most accessed keys.

```
HOTKEYS [count]
```

**Arguments:**
- `count` - Number of keys to return (default: 10)

**Returns:** Hot keys with access counts

**Example:**
```
HOTKEYS 5
popular_key (reads=150 writes=10)
another_key (reads=100 writes=5)
third_key (reads=50 writes=2)
```

---

### SUGGEST-TTL

Get a suggested TTL based on access patterns.

```
SUGGEST-TTL key
```

**Arguments:**
- `key` - The key to analyze

**Returns:** Suggested TTL in seconds

**Example:**
```
SUGGEST-TTL cache_key
3600
```

---

### ANOMALIES

Detect keys with unusual access patterns.

```
ANOMALIES
```

**Returns:**
- Keys with anomalous access
- `(no anomalies detected)` if none found

**Example:**
```
ANOMALIES
suspicious_key
another_outlier
```

---

## Error Responses

| Error | Description |
|-------|-------------|
| `-ERR key not found` | Key doesn't exist |
| `-ERR empty command` | No command provided |
| `-ERR unknown command 'X'` | Unrecognized command |
| `-ERR SET requires key and value` | Missing arguments |
| `-ERR invalid TTL` | TTL is not a positive integer |
| `-ERR value is not an integer` | INCR/DECR on non-numeric value |
| `-ERR connection limit reached` | Max connections exceeded |
| `-ERR analytics not enabled` | Analytics commands require flag |

---

## Response Codes

| Prefix | Meaning |
|--------|---------|
| `+OK` | Success |
| `+PONG` | Ping response |
| `-ERR` | Error |
| `1` | True/success (for EXISTS, EXPIRE, etc.) |
| `0` | False/not found |
| `-1` | No TTL set |
| `-2` | Key doesn't exist (for TTL) |
