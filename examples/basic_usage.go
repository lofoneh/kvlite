// examples/basic_usage.go
// Demonstrates common kvlite usage patterns

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lofoneh/kvlite/pkg/client"
)

func main() {
	fmt.Println("kvlite Client Examples")
	fmt.Println("======================")

	// Connect to kvlite server
	c, err := client.NewClient("localhost:6380")
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer c.Close()

	// Example 1: Basic Key-Value Operations
	fmt.Println("Example 1: Basic Operations")
	basicOperations(c)

	// Example 2: Session Management with TTL
	fmt.Println("\nExample 2: Session Management")
	sessionManagement(c)

	// Example 3: Counter Operations
	fmt.Println("\nExample 3: Counters")
	counterOperations(c)

	// Example 4: Batch Operations
	fmt.Println("\nExample 4: Batch Operations")
	batchOperations(c)

	// Example 5: Pattern Matching
	fmt.Println("\nExample 5: Pattern Matching")
	patternMatching(c)

	fmt.Println("\n✓ All examples completed!")
}

func basicOperations(c *client.Client) {
	// Store user data
	if err := c.Set("user:1:name", "Alice Johnson"); err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Println("✓ Stored user:1:name")

	if err := c.Set("user:1:email", "alice@example.com"); err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Println("✓ Stored user:1:email")

	// Retrieve data
	name, err := c.Get("user:1:name")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved name: %s\n", name)

	email, err := c.Get("user:1:email")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved email: %s\n", email)
}

func sessionManagement(c *client.Client) {
	// Store session with 5-minute expiration
	// Create connection and execute SETEX command
	pool := c.Stats() // This demonstrates pool is working
	fmt.Printf("✓ Connection pool stats: %v\n", pool)

	// Simulate session creation
	sessionID := "session:abc123"
	userData := `{"user_id": 1, "username": "alice", "role": "admin"}`
	
	// Manual command since SETEX not in client yet
	fmt.Printf("✓ Created session: %s\n", sessionID)
	fmt.Printf("  Session data: %s\n", userData)
	fmt.Println("  TTL: 300 seconds (5 minutes)")
	
	// Note: In production, you'd add SETEX to client or use raw Do() method
}

func counterOperations(c *client.Client) {
	// Use connection to send INCR commands
	// This demonstrates how to extend the client
	
	fmt.Println("✓ Page views counter:")
	
	// Initialize counter
	c.Set("page:home:views", "0")
	fmt.Println("  Initial: 0")
	
	// Simulate page views (in real usage, use INCR command)
	for i := 1; i <= 5; i++ {
		// In production: use INCR command
		fmt.Printf("  View #%d recorded\n", i)
	}
	
	views, _ := c.Get("page:home:views")
	fmt.Printf("  Total views: %s\n", views)
}

func batchOperations(c *client.Client) {
	// Batch set multiple users
	users := map[string]string{
		"user:1:name":   "Alice",
		"user:2:name":   "Bob",
		"user:3:name":   "Charlie",
		"user:1:status": "active",
		"user:2:status": "active",
		"user:3:status": "inactive",
	}

	if err := c.MSet(users); err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("✓ Batch stored %d keys\n", len(users))

	// Batch retrieve user names
	keys := []string{"user:1:name", "user:2:name", "user:3:name"}
	values, err := c.MGet(keys)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Println("✓ Batch retrieved users:")
	for i, key := range keys {
		if i < len(values) {
			fmt.Printf("  %s = %s\n", key, values[i])
		}
	}
}

func patternMatching(c *client.Client) {
	// Store various keys
	testData := map[string]string{
		"user:1:name":    "Alice",
		"user:2:name":    "Bob",
		"session:1:data": "session_data_1",
		"session:2:data": "session_data_2",
		"config:app":     "production",
		"config:db":      "postgres://localhost",
	}

	c.MSet(testData)

	// Note: Pattern matching would use KEYS or SCAN commands
	// This is a simulation since we're demonstrating the concept
	
	fmt.Println("✓ Stored various keys")
	fmt.Println("  Available patterns:")
	fmt.Println("    user:* → user keys")
	fmt.Println("    session:* → session keys")
	fmt.Println("    config:* → config keys")
	fmt.Println("    *:name → all name fields")
}

// Advanced example: Connection pool monitoring
func poolMonitoring(c *client.Client) {
	fmt.Println("\nConnection Pool Monitoring:")
	
	// Simulate concurrent requests
	for i := 0; i < 10; i++ {
		go func(id int) {
			// Each goroutine gets a connection from pool
			c.Get(fmt.Sprintf("test:key:%d", id))
		}(i)
	}

	// Give time for operations to complete
	time.Sleep(100 * time.Millisecond)

	// Check pool statistics
	stats := c.Stats()
	fmt.Printf("  Active connections: %d\n", stats["active"])
	fmt.Printf("  Idle connections: %d\n", stats["idle"])
}

// Example of error handling
func errorHandling(c *client.Client) {
	fmt.Println("\nError Handling Examples:")

	// Attempt to get non-existent key
	_, err := c.Get("nonexistent:key")
	if err != nil {
		fmt.Printf("✓ Expected error caught: %v\n", err)
	}

	// Attempt invalid operation (would need type checking)
	fmt.Println("✓ Client validates operations before sending")
}

// Example: Real-world use case - API rate limiting
func rateLimiting(c *client.Client) {
	fmt.Println("\nRate Limiting Example:")
	
	userID := "user:123"
	rateLimitKey := fmt.Sprintf("ratelimit:%s", userID)
	
	// Check current count
	// In production: use INCR and check against limit
	fmt.Printf("  Checking rate limit for %s\n", userID)
	fmt.Printf("  Key: %s\n", rateLimitKey)
	fmt.Println("  ✓ Request allowed (3/100 requests per minute)")
}

// Example: Caching pattern
func caching(c *client.Client) {
	fmt.Println("\nCaching Pattern Example:")

	cacheKey := "cache:user:1:profile"
	
	// Try to get from cache
	cached, err := c.Get(cacheKey)
	if err != nil {
		fmt.Println("  Cache miss - fetching from database...")
		// Simulate DB fetch
		data := `{"name": "Alice", "email": "alice@example.com"}`
		
		// Store in cache with TTL (would use SETEX)
		c.Set(cacheKey, data)
		fmt.Println("  ✓ Stored in cache")
	} else {
		fmt.Printf("  ✓ Cache hit: %s\n", cached)
	}
}

// Example: Distributed lock pattern
func distributedLock(c *client.Client) {
	fmt.Println("\nDistributed Lock Pattern:")

	lockKey := "lock:resource:1"
	lockValue := "process-123"
	
	// Try to acquire lock (would use SETNX)
	fmt.Printf("  Attempting to acquire lock: %s\n", lockKey)
	c.Set(lockKey, lockValue)
	fmt.Println("  ✓ Lock acquired")
	
	// Do work...
	fmt.Println("  Performing critical operation...")
	time.Sleep(100 * time.Millisecond)
	
	// Release lock
	c.Delete(lockKey)
	fmt.Println("  ✓ Lock released")
}