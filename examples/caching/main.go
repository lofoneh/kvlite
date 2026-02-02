// examples/caching.go
// Demonstrates caching patterns with kvlite

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/lofoneh/kvlite/pkg/client"
)

// User represents a user entity from database
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// UserCache demonstrates cache-aside pattern
type UserCache struct {
	client *client.Client
	ttl    int // TTL in seconds
}

// NewUserCache creates a new user cache
func NewUserCache(c *client.Client, ttlSeconds int) *UserCache {
	return &UserCache{
		client: c,
		ttl:    ttlSeconds,
	}
}

// GetUser implements cache-aside pattern:
// 1. Try cache first
// 2. If miss, fetch from "database" and populate cache
func (uc *UserCache) GetUser(userID int) (*User, error) {
	cacheKey := fmt.Sprintf("user:%d", userID)

	// Try cache first
	cached, err := uc.client.Get(cacheKey)
	if err == nil {
		// Cache hit - deserialize and return
		var user User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			fmt.Printf("  Cache HIT for user %d\n", userID)
			return &user, nil
		}
	}

	// Cache miss - fetch from "database"
	fmt.Printf("  Cache MISS for user %d - fetching from database\n", userID)
	user := uc.fetchFromDatabase(userID)

	// Populate cache
	data, _ := json.Marshal(user)
	uc.client.Set(cacheKey, string(data))

	return user, nil
}

// InvalidateUser removes user from cache
func (uc *UserCache) InvalidateUser(userID int) error {
	cacheKey := fmt.Sprintf("user:%d", userID)
	return uc.client.Delete(cacheKey)
}

// fetchFromDatabase simulates database fetch
func (uc *UserCache) fetchFromDatabase(userID int) *User {
	// Simulate database latency
	time.Sleep(50 * time.Millisecond)

	return &User{
		ID:        userID,
		Name:      fmt.Sprintf("User %d", userID),
		Email:     fmt.Sprintf("user%d@example.com", userID),
		CreatedAt: time.Now(),
	}
}

// ProductCache demonstrates write-through caching
type ProductCache struct {
	client *client.Client
}

// Product represents a product entity
type Product struct {
	SKU   string  `json:"sku"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

// UpdateProduct demonstrates write-through:
// 1. Write to database first
// 2. Update cache on success
func (pc *ProductCache) UpdateProduct(product *Product) error {
	fmt.Printf("  Updating product %s in database...\n", product.SKU)

	// Step 1: Write to database (simulated)
	time.Sleep(20 * time.Millisecond)
	fmt.Printf("  Database updated for %s\n", product.SKU)

	// Step 2: Update cache
	cacheKey := fmt.Sprintf("product:%s", product.SKU)
	data, _ := json.Marshal(product)
	if err := pc.client.Set(cacheKey, string(data)); err != nil {
		return fmt.Errorf("cache update failed: %w", err)
	}
	fmt.Printf("  Cache updated for %s\n", product.SKU)

	return nil
}

// GetProduct retrieves product (cache-through read)
func (pc *ProductCache) GetProduct(sku string) (*Product, error) {
	cacheKey := fmt.Sprintf("product:%s", sku)

	cached, err := pc.client.Get(cacheKey)
	if err == nil {
		var product Product
		if err := json.Unmarshal([]byte(cached), &product); err == nil {
			return &product, nil
		}
	}

	return nil, fmt.Errorf("product not found: %s", sku)
}

func main() {
	fmt.Println("kvlite Caching Patterns Demo")
	fmt.Println("============================")

	// Connect to kvlite server
	c, err := client.NewClient("localhost:6380")
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer c.Close()

	// Example 1: Cache-Aside Pattern
	fmt.Println("\n1. Cache-Aside Pattern (Lazy Loading)")
	fmt.Println("   Read from cache first, populate on miss")

	userCache := NewUserCache(c, 300) // 5 minute TTL

	// First access - cache miss
	fmt.Println("\n   First access (cache miss):")
	user, _ := userCache.GetUser(123)
	fmt.Printf("   Got user: %s <%s>\n", user.Name, user.Email)

	// Second access - cache hit
	fmt.Println("\n   Second access (cache hit):")
	user, _ = userCache.GetUser(123)
	fmt.Printf("   Got user: %s <%s>\n", user.Name, user.Email)

	// Invalidate and re-fetch
	fmt.Println("\n   After invalidation:")
	userCache.InvalidateUser(123)
	user, _ = userCache.GetUser(123)
	fmt.Printf("   Got user: %s <%s>\n", user.Name, user.Email)

	// Example 2: Write-Through Pattern
	fmt.Println("\n2. Write-Through Pattern")
	fmt.Println("   Update database and cache together")

	productCache := &ProductCache{client: c}

	product := &Product{
		SKU:   "WIDGET-001",
		Name:  "Super Widget",
		Price: 29.99,
		Stock: 100,
	}

	fmt.Println("\n   Creating product:")
	productCache.UpdateProduct(product)

	// Read back
	p, _ := productCache.GetProduct("WIDGET-001")
	fmt.Printf("   Product: %s - $%.2f (%d in stock)\n", p.Name, p.Price, p.Stock)

	// Update
	fmt.Println("\n   Updating stock:")
	product.Stock = 95
	productCache.UpdateProduct(product)

	p, _ = productCache.GetProduct("WIDGET-001")
	fmt.Printf("   Product: %s - $%.2f (%d in stock)\n", p.Name, p.Price, p.Stock)

	// Example 3: Cache Key Patterns
	fmt.Println("\n3. Cache Key Naming Patterns")
	fmt.Println("   Consistent key naming for organization")

	keyExamples := map[string]string{
		"user:123":                "User by ID",
		"user:email:alice@ex.com": "User by email",
		"session:abc123":          "User session",
		"cache:api:/users/123":    "API response cache",
		"config:feature:darkmode": "Feature flag",
		"rate:ip:192.168.1.1":     "Rate limit counter",
	}

	for key, desc := range keyExamples {
		fmt.Printf("   %s -> %s\n", key, desc)
	}

	fmt.Println("\nCaching demo complete!")
}
