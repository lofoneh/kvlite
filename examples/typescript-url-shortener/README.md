# KVLite URL Shortener

A URL shortening service built with TypeScript, Express.js, and kvlite as the backend storage. This example demonstrates using kvlite as a Redis alternative for real-world applications.

## Features

- Create short URLs with auto-generated or custom codes
- Automatic expiration (TTL) support
- Click tracking and statistics
- Rate limiting on URL creation
- Health check endpoint

## Prerequisites

- Node.js 18+
- kvlite server running on localhost:6380

## Quick Start

### 1. Start kvlite server

```bash
# From the kvlite root directory
make run
```

### 2. Install dependencies

```bash
cd examples/typescript-url-shortener
npm install
```

### 3. Configure environment (optional)

```bash
cp .env.example .env
# Edit .env if needed
```

### 4. Run the server

```bash
# Development
npm run dev

# Production
npm run build && npm start
```

## API Endpoints

### Create Short URL

```bash
POST /shorten
Content-Type: application/json

{
  "url": "https://example.com/very/long/url",
  "customCode": "mylink",    # Optional custom code
  "expiresIn": 86400         # Optional TTL in seconds
}
```

Response:
```json
{
  "shortCode": "mylink",
  "shortUrl": "http://localhost:3000/mylink",
  "longUrl": "https://example.com/very/long/url",
  "expiresIn": 86400
}
```

### Redirect to Long URL

```bash
GET /:code
# Returns 302 redirect to the original URL
```

### Get Statistics

```bash
GET /stats/:code
```

Response:
```json
{
  "shortCode": "mylink",
  "longUrl": "https://example.com/very/long/url",
  "clicks": 42,
  "createdAt": "2024-01-15T10:30:00.000Z",
  "ttlRemaining": 85000
}
```

### Delete URL

```bash
DELETE /:code
# Returns 204 No Content
```

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "kvlite": "connected"
}
```

## KVLite Commands Demonstrated

| Command | Usage |
|---------|-------|
| `SET` | Initialize click counters |
| `SETEX` | Store URL mappings with TTL |
| `GET` | Retrieve URL data |
| `DELETE` | Remove URLs |
| `EXISTS` | Check for code collisions |
| `INCR` | Atomic click counting |
| `TTL` | Check remaining expiration |
| `EXPIRE` | Set key expiration |
| `PING` | Health checks |

## KVLite Key Schema

| Key Pattern | Description |
|-------------|-------------|
| `url:{code}` | JSON object with URL mapping |
| `clicks:{code}` | Integer click counter |
| `ratelimit:urlshorten:{ip}:{window}` | Rate limit counters |

## Rate Limiting

The `/shorten` endpoint is rate-limited to 100 requests per minute per IP.

Response headers:
- `X-RateLimit-Limit`: Maximum requests allowed
- `X-RateLimit-Remaining`: Requests remaining in window
- `X-RateLimit-Reset`: Seconds until window resets

## Architecture

```
POST /shorten ──> Rate Limiter ──> URL Service ──> KVLite
                      |                 |
                      v                 v
                 INCR counter      SETEX url:{code}
                                   SET clicks:{code}

GET /:code ──────────────────────> URL Service ──> KVLite
                                        |
                                        v
                                   GET url:{code}
                                   INCR clicks:{code}
                                   302 Redirect
```

## Configuration

Environment variables (`.env`):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 3000 | Server port |
| `BASE_URL` | http://localhost:3000 | Base URL for short links |
| `KVLITE_HOST` | localhost | KVLite server host |
| `KVLITE_PORT` | 6380 | KVLite server port |
| `RATE_LIMIT_ENABLED` | true | Enable rate limiting |
| `RATE_LIMIT_MAX_REQUESTS` | 100 | Max requests per window |
| `RATE_LIMIT_WINDOW_SECONDS` | 60 | Rate limit window |

## License

MIT
