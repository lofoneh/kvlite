// examples/counters.go
// Demonstrates counter and analytics patterns with kvlite

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

// Counter provides atomic counter operations
type Counter struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

// NewCounter creates a new counter client
func NewCounter(addr string) (*Counter, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome message
	reader.ReadString('\n')

	return &Counter{
		conn:   conn,
		reader: reader,
		writer: writer,
	}, nil
}

// Close closes the connection
func (c *Counter) Close() {
	c.conn.Close()
}

// sendCommand sends a command and returns the response
func (c *Counter) sendCommand(cmd string) (string, error) {
	fmt.Fprintf(c.writer, "%s\n", cmd)
	c.writer.Flush()

	response, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// Increment increments a counter and returns the new value
func (c *Counter) Increment(name string) (int64, error) {
	response, err := c.sendCommand(fmt.Sprintf("INCR %s", name))
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(response, 10, 64)
}

// Decrement decrements a counter and returns the new value
func (c *Counter) Decrement(name string) (int64, error) {
	response, err := c.sendCommand(fmt.Sprintf("DECR %s", name))
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(response, 10, 64)
}

// Get gets the current value of a counter
func (c *Counter) Get(name string) (int64, error) {
	response, err := c.sendCommand(fmt.Sprintf("GET %s", name))
	if err != nil {
		return 0, err
	}

	if strings.HasPrefix(response, "-ERR") {
		return 0, nil // Counter doesn't exist yet
	}

	return strconv.ParseInt(response, 10, 64)
}

// Set sets a counter to a specific value
func (c *Counter) Set(name string, value int64) error {
	response, err := c.sendCommand(fmt.Sprintf("SET %s %d", name, value))
	if err != nil {
		return err
	}

	if response != "+OK" {
		return fmt.Errorf("set failed: %s", response)
	}

	return nil
}

// Reset resets a counter to zero
func (c *Counter) Reset(name string) error {
	return c.Set(name, 0)
}

// PageViewTracker tracks page views
type PageViewTracker struct {
	counter *Counter
}

// TrackPageView records a page view
func (p *PageViewTracker) TrackPageView(path string) (int64, error) {
	key := fmt.Sprintf("pageviews:%s", path)
	return p.counter.Increment(key)
}

// GetPageViews gets total views for a path
func (p *PageViewTracker) GetPageViews(path string) (int64, error) {
	key := fmt.Sprintf("pageviews:%s", path)
	return p.counter.Get(key)
}

// TrackUniqueVisitor tracks unique visitors (simplified)
func (p *PageViewTracker) TrackUniqueVisitor(path string, visitorID string) error {
	// In production, use a set or hyperloglog for unique counting
	key := fmt.Sprintf("visitors:%s:%s", path, visitorID)
	_, err := p.counter.sendCommand(fmt.Sprintf("SETEX %s 86400 1", key)) // 24 hour TTL
	return err
}

// TimeWindowCounter tracks counts in time windows
type TimeWindowCounter struct {
	counter *Counter
}

// IncrementHourly increments a counter in the current hour window
func (t *TimeWindowCounter) IncrementHourly(name string) (int64, error) {
	hour := time.Now().Format("2006010215")
	key := fmt.Sprintf("%s:hourly:%s", name, hour)

	count, err := t.counter.Increment(key)
	if err != nil {
		return 0, err
	}

	// Set expiry (keep 48 hours of data)
	if count == 1 {
		t.counter.sendCommand(fmt.Sprintf("EXPIRE %s %d", key, 48*3600))
	}

	return count, nil
}

// IncrementDaily increments a counter in the current day window
func (t *TimeWindowCounter) IncrementDaily(name string) (int64, error) {
	day := time.Now().Format("20060102")
	key := fmt.Sprintf("%s:daily:%s", name, day)

	count, err := t.counter.Increment(key)
	if err != nil {
		return 0, err
	}

	// Set expiry (keep 30 days of data)
	if count == 1 {
		t.counter.sendCommand(fmt.Sprintf("EXPIRE %s %d", key, 30*86400))
	}

	return count, nil
}

// GetHourlyCounts gets hourly counts for the last N hours
func (t *TimeWindowCounter) GetHourlyCounts(name string, hours int) (map[string]int64, error) {
	counts := make(map[string]int64)

	for i := 0; i < hours; i++ {
		hour := time.Now().Add(-time.Duration(i) * time.Hour).Format("2006010215")
		key := fmt.Sprintf("%s:hourly:%s", name, hour)

		count, _ := t.counter.Get(key)
		counts[hour] = count
	}

	return counts, nil
}

func main() {
	fmt.Println("kvlite Counters & Analytics Demo")
	fmt.Println("=================================")

	// Connect to kvlite server
	counter, err := NewCounter("localhost:6380")
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer counter.Close()

	// Example 1: Basic Counter Operations
	fmt.Println("\n1. Basic Counter Operations")

	counter.Reset("demo:counter")

	for i := 0; i < 5; i++ {
		value, _ := counter.Increment("demo:counter")
		fmt.Printf("   Increment: %d\n", value)
	}

	value, _ := counter.Decrement("demo:counter")
	fmt.Printf("   Decrement: %d\n", value)

	current, _ := counter.Get("demo:counter")
	fmt.Printf("   Current value: %d\n", current)

	// Example 2: Page View Tracking
	fmt.Println("\n2. Page View Tracking")

	pageTracker := &PageViewTracker{counter: counter}

	pages := []string{"/home", "/products", "/about", "/home", "/home", "/products"}

	for _, page := range pages {
		views, _ := pageTracker.TrackPageView(page)
		fmt.Printf("   %s: %d views\n", page, views)
	}

	fmt.Println("\n   Summary:")
	for _, page := range []string{"/home", "/products", "/about"} {
		views, _ := pageTracker.GetPageViews(page)
		fmt.Printf("   %s: %d total views\n", page, views)
	}

	// Example 3: Active Users Counter
	fmt.Println("\n3. Active Users Counter")

	// Simulate users logging in/out
	counter.sendCommand("SET active:users 0")

	fmt.Println("   Users logging in:")
	for i := 1; i <= 5; i++ {
		val, _ := counter.Increment("active:users")
		fmt.Printf("   User %d logged in. Active users: %d\n", i, val)
	}

	fmt.Println("\n   Users logging out:")
	for i := 1; i <= 2; i++ {
		val, _ := counter.Decrement("active:users")
		fmt.Printf("   User %d logged out. Active users: %d\n", i, val)
	}

	// Example 4: Time Window Counters
	fmt.Println("\n4. Time Window Counters")

	timeCounter := &TimeWindowCounter{counter: counter}

	// Track some events
	for i := 0; i < 10; i++ {
		count, _ := timeCounter.IncrementHourly("api:requests")
		fmt.Printf("   API request %d (hourly total: %d)\n", i+1, count)
	}

	for i := 0; i < 5; i++ {
		count, _ := timeCounter.IncrementDaily("user:signups")
		fmt.Printf("   Signup %d (daily total: %d)\n", i+1, count)
	}

	// Example 5: Real-Time Statistics
	fmt.Println("\n5. Real-Time Statistics")

	// Sales statistics
	counter.Set("stats:orders:total", 1547)
	counter.Set("stats:orders:today", 23)
	counter.Set("stats:revenue:today", 4599)

	// Simulate new orders
	for i := 0; i < 3; i++ {
		counter.Increment("stats:orders:total")
		counter.Increment("stats:orders:today")
		// Add revenue (simulated)
		response, _ := counter.sendCommand("GET stats:revenue:today")
		current, _ := strconv.ParseInt(response, 10, 64)
		counter.Set("stats:revenue:today", current+199)
	}

	total, _ := counter.Get("stats:orders:total")
	today, _ := counter.Get("stats:orders:today")
	revenue, _ := counter.Get("stats:revenue:today")

	fmt.Printf("   Total orders: %d\n", total)
	fmt.Printf("   Orders today: %d\n", today)
	fmt.Printf("   Revenue today: $%.2f\n", float64(revenue)/100)

	// Example 6: Error Rate Tracking
	fmt.Println("\n6. Error Rate Tracking")

	counter.Set("errors:api:total", 0)
	counter.Set("requests:api:total", 0)

	// Simulate requests (some with errors)
	requests := []bool{true, true, true, false, true, true, false, true, true, true}
	for i, success := range requests {
		counter.Increment("requests:api:total")
		if !success {
			counter.Increment("errors:api:total")
		}
		fmt.Printf("   Request %d: success=%v\n", i+1, success)
	}

	totalReq, _ := counter.Get("requests:api:total")
	totalErr, _ := counter.Get("errors:api:total")
	errorRate := float64(totalErr) / float64(totalReq) * 100

	fmt.Printf("\n   Total requests: %d\n", totalReq)
	fmt.Printf("   Total errors: %d\n", totalErr)
	fmt.Printf("   Error rate: %.1f%%\n", errorRate)

	// Example 7: Leaderboard (simple implementation)
	fmt.Println("\n7. Simple Leaderboard")

	// Set scores
	players := map[string]int64{
		"alice":   1500,
		"bob":     2300,
		"charlie": 1800,
		"diana":   2100,
		"eve":     1950,
	}

	for player, score := range players {
		counter.Set(fmt.Sprintf("score:%s", player), score)
		fmt.Printf("   %s: %d points\n", player, score)
	}

	// Update a score
	counter.Increment("score:alice")
	counter.Increment("score:alice")
	newScore, _ := counter.Get("score:alice")
	fmt.Printf("\n   Alice's new score: %d\n", newScore)

	// Example 8: Key Patterns for Analytics
	fmt.Println("\n8. Analytics Key Patterns")
	patterns := map[string]string{
		"count:{entity}:{id}":              "Entity counters",
		"stats:{metric}:{period}:{window}": "Time-windowed stats",
		"score:{user}":                     "User scores",
		"rate:{resource}:{window}":         "Rate counters",
		"hits:{page}:{date}":               "Daily page hits",
		"active:{resource}":                "Active resource counts",
	}

	for pattern, desc := range patterns {
		fmt.Printf("   %s -> %s\n", pattern, desc)
	}

	fmt.Println("\nCounters & Analytics demo complete!")
}
