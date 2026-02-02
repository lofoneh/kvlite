// examples/rate_limiting.go
// Demonstrates rate limiting patterns with kvlite

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// RateLimiter implements various rate limiting strategies
type RateLimiter struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(addr string) (*RateLimiter, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome message
	reader.ReadString('\n')

	return &RateLimiter{
		conn:   conn,
		reader: reader,
		writer: writer,
	}, nil
}

// Close closes the connection
func (rl *RateLimiter) Close() {
	rl.conn.Close()
}

// sendCommand sends a command and returns the response
func (rl *RateLimiter) sendCommand(cmd string) (string, error) {
	fmt.Fprintf(rl.writer, "%s\n", cmd)
	rl.writer.Flush()

	response, err := rl.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// FixedWindowLimit implements fixed window rate limiting
// Example: 100 requests per minute
func (rl *RateLimiter) FixedWindowLimit(identifier string, limit int, windowSeconds int) (bool, int, error) {
	// Create key based on current window
	window := time.Now().Unix() / int64(windowSeconds)
	key := fmt.Sprintf("ratelimit:fixed:%s:%d", identifier, window)

	// Increment counter
	response, err := rl.sendCommand(fmt.Sprintf("INCR %s", key))
	if err != nil {
		return false, 0, err
	}

	count, _ := strconv.Atoi(response)

	// Set expiry on first request in window
	if count == 1 {
		rl.sendCommand(fmt.Sprintf("EXPIRE %s %d", key, windowSeconds))
	}

	allowed := count <= limit
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, nil
}

// SlidingWindowLogLimit implements sliding window log rate limiting
// More accurate but uses more memory
func (rl *RateLimiter) SlidingWindowLogLimit(identifier string, limit int, windowSeconds int) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - int64(windowSeconds)
	key := fmt.Sprintf("ratelimit:sliding:%s", identifier)

	// In a full implementation, we'd use a sorted set
	// For simplicity, we'll use the fixed window approach with smaller windows
	// This is a simplified demonstration

	// Use current timestamp as part of the key for fine-grained tracking
	currentSecond := now
	countKey := fmt.Sprintf("%s:%d", key, currentSecond)

	// Count requests in this second
	response, _ := rl.sendCommand(fmt.Sprintf("INCR %s", countKey))
	rl.sendCommand(fmt.Sprintf("EXPIRE %s %d", countKey, windowSeconds))

	count, _ := strconv.Atoi(response)

	// For demo purposes, use simple window
	// Real implementation would aggregate counts across the window
	perSecondLimit := limit / windowSeconds
	if perSecondLimit < 1 {
		perSecondLimit = 1
	}

	allowed := count <= perSecondLimit
	_ = windowStart // Used in full implementation

	return allowed, nil
}

// TokenBucketLimit implements token bucket rate limiting
// Allows burst traffic while maintaining average rate
func (rl *RateLimiter) TokenBucketLimit(identifier string, bucketSize int, refillRate float64) (bool, int, error) {
	key := fmt.Sprintf("ratelimit:bucket:%s", identifier)
	timestampKey := fmt.Sprintf("%s:ts", key)

	now := time.Now().Unix()

	// Get current tokens and last refill time
	tokensResp, _ := rl.sendCommand(fmt.Sprintf("GET %s", key))
	lastRefillResp, _ := rl.sendCommand(fmt.Sprintf("GET %s", timestampKey))

	var tokens float64
	var lastRefill int64

	if strings.HasPrefix(tokensResp, "-ERR") {
		// First request - full bucket
		tokens = float64(bucketSize)
		lastRefill = now
	} else {
		tokens, _ = strconv.ParseFloat(tokensResp, 64)
		lastRefill, _ = strconv.ParseInt(lastRefillResp, 10, 64)
	}

	// Calculate tokens to add based on time elapsed
	elapsed := float64(now - lastRefill)
	tokensToAdd := elapsed * refillRate
	tokens = tokens + tokensToAdd

	// Cap at bucket size
	if tokens > float64(bucketSize) {
		tokens = float64(bucketSize)
	}

	// Check if we have a token to consume
	allowed := tokens >= 1.0

	if allowed {
		tokens -= 1.0
	}

	// Save state
	rl.sendCommand(fmt.Sprintf("SET %s %.2f", key, tokens))
	rl.sendCommand(fmt.Sprintf("SET %s %d", timestampKey, now))
	rl.sendCommand(fmt.Sprintf("EXPIRE %s 3600", key))        // 1 hour cleanup
	rl.sendCommand(fmt.Sprintf("EXPIRE %s 3600", timestampKey)) // 1 hour cleanup

	return allowed, int(tokens), nil
}

// LeakyBucketLimit implements leaky bucket rate limiting
// Smooths out bursty traffic
func (rl *RateLimiter) LeakyBucketLimit(identifier string, bucketSize int, leakRate float64) (bool, int, error) {
	key := fmt.Sprintf("ratelimit:leaky:%s", identifier)
	timestampKey := fmt.Sprintf("%s:ts", key)

	now := time.Now().Unix()

	// Get current water level and last leak time
	levelResp, _ := rl.sendCommand(fmt.Sprintf("GET %s", key))
	lastLeakResp, _ := rl.sendCommand(fmt.Sprintf("GET %s", timestampKey))

	var level float64
	var lastLeak int64

	if strings.HasPrefix(levelResp, "-ERR") {
		level = 0
		lastLeak = now
	} else {
		level, _ = strconv.ParseFloat(levelResp, 64)
		lastLeak, _ = strconv.ParseInt(lastLeakResp, 10, 64)
	}

	// Calculate water that leaked out
	elapsed := float64(now - lastLeak)
	leaked := elapsed * leakRate
	level = level - leaked

	if level < 0 {
		level = 0
	}

	// Check if bucket has room for another request
	allowed := level < float64(bucketSize)

	if allowed {
		level += 1.0
	}

	// Save state
	rl.sendCommand(fmt.Sprintf("SET %s %.2f", key, level))
	rl.sendCommand(fmt.Sprintf("SET %s %d", timestampKey, now))
	rl.sendCommand(fmt.Sprintf("EXPIRE %s 3600", key))
	rl.sendCommand(fmt.Sprintf("EXPIRE %s 3600", timestampKey))

	return allowed, bucketSize - int(level), nil
}

func main() {
	fmt.Println("kvlite Rate Limiting Demo")
	fmt.Println("=========================")

	// Connect to kvlite server
	rl, err := NewRateLimiter("localhost:6380")
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer rl.Close()

	// Example 1: Fixed Window Rate Limiting
	fmt.Println("\n1. Fixed Window Rate Limiting")
	fmt.Println("   100 requests per 60 seconds")

	for i := 1; i <= 5; i++ {
		allowed, remaining, _ := rl.FixedWindowLimit("user:123", 100, 60)
		fmt.Printf("   Request %d: allowed=%v, remaining=%d\n", i, allowed, remaining)
	}

	// Example 2: Token Bucket Rate Limiting
	fmt.Println("\n2. Token Bucket Rate Limiting")
	fmt.Println("   Bucket size: 10, Refill rate: 1 token/second")
	fmt.Println("   Allows bursts up to bucket size")

	for i := 1; i <= 12; i++ {
		allowed, tokens, _ := rl.TokenBucketLimit("api:client:456", 10, 1.0)
		status := "ALLOWED"
		if !allowed {
			status = "DENIED"
		}
		fmt.Printf("   Request %d: %s (tokens remaining: %d)\n", i, status, tokens)
	}

	// Wait for tokens to refill
	fmt.Println("   Waiting 3 seconds for token refill...")
	time.Sleep(3 * time.Second)

	for i := 1; i <= 3; i++ {
		allowed, tokens, _ := rl.TokenBucketLimit("api:client:456", 10, 1.0)
		status := "ALLOWED"
		if !allowed {
			status = "DENIED"
		}
		fmt.Printf("   Request %d: %s (tokens remaining: %d)\n", i, status, tokens)
	}

	// Example 3: Leaky Bucket Rate Limiting
	fmt.Println("\n3. Leaky Bucket Rate Limiting")
	fmt.Println("   Bucket size: 5, Leak rate: 1/second")
	fmt.Println("   Smooths bursty traffic")

	for i := 1; i <= 7; i++ {
		allowed, remaining, _ := rl.LeakyBucketLimit("endpoint:/api/data", 5, 1.0)
		status := "ALLOWED"
		if !allowed {
			status = "DENIED"
		}
		fmt.Printf("   Request %d: %s (remaining capacity: %d)\n", i, status, remaining)
	}

	// Example 4: Per-IP Rate Limiting
	fmt.Println("\n4. Per-IP Address Rate Limiting")
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.1"}

	for i, ip := range ips {
		allowed, remaining, _ := rl.FixedWindowLimit(fmt.Sprintf("ip:%s", ip), 100, 60)
		fmt.Printf("   Request %d from %s: allowed=%v, remaining=%d\n",
			i+1, ip, allowed, remaining)
	}

	// Example 5: Tiered Rate Limiting
	fmt.Println("\n5. Tiered Rate Limiting (by user tier)")
	tiers := map[string]int{
		"free":       10,
		"basic":      100,
		"premium":    1000,
		"enterprise": 10000,
	}

	for tier, limit := range tiers {
		allowed, remaining, _ := rl.FixedWindowLimit(
			fmt.Sprintf("tier:%s:user:789", tier),
			limit,
			60,
		)
		fmt.Printf("   %s tier (limit=%d): allowed=%v, remaining=%d\n",
			tier, limit, allowed, remaining)
	}

	// Example 6: API Endpoint Rate Limiting
	fmt.Println("\n6. Per-Endpoint Rate Limiting")
	endpoints := map[string]int{
		"/api/search":  10,  // Expensive operation
		"/api/users":   100, // Normal operation
		"/api/health":  1000, // Health check
	}

	for endpoint, limit := range endpoints {
		allowed, remaining, _ := rl.FixedWindowLimit(
			fmt.Sprintf("endpoint:%s", endpoint),
			limit,
			60,
		)
		fmt.Printf("   %s (limit=%d/min): allowed=%v, remaining=%d\n",
			endpoint, limit, allowed, remaining)
	}

	fmt.Println("\nRate limiting demo complete!")
	fmt.Println("\nKey patterns used:")
	fmt.Println("  - ratelimit:fixed:{identifier}:{window}")
	fmt.Println("  - ratelimit:bucket:{identifier}")
	fmt.Println("  - ratelimit:leaky:{identifier}")
}
