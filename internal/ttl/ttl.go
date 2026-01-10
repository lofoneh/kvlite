// internal/ttl/ttl.go
package ttl

import (
	"log"
	"sync"
	"time"
)

// Manager handles automatic expiration of keys
type Manager struct {
	store          StoreInterface
	ticker         *time.Ticker
	stopChan       chan struct{}
	wg             sync.WaitGroup
	checkInterval  time.Duration
	stats          Stats
	mu             sync.RWMutex
}

// StoreInterface defines the methods needed from store
type StoreInterface interface {
	DeleteExpired() int
}

// Stats tracks expiration statistics
type Stats struct {
	TotalExpired     int64
	LastCheckTime    time.Time
	LastExpiredCount int
	ChecksPerformed  int64
}

// Options for TTL manager
type Options struct {
	CheckInterval time.Duration // How often to check for expired keys
}

// NewManager creates a new TTL manager
func NewManager(store StoreInterface, opts Options) *Manager {
	if opts.CheckInterval == 0 {
		opts.CheckInterval = 1 * time.Second // Default: check every second
	}

	return &Manager{
		store:         store,
		checkInterval: opts.CheckInterval,
		stopChan:      make(chan struct{}),
	}
}

// Start begins the background expiration goroutine
func (m *Manager) Start() {
	m.ticker = time.NewTicker(m.checkInterval)
	m.wg.Add(1)
	
	go m.expirationLoop()
	log.Printf("TTL manager started (check interval: %v)", m.checkInterval)
}

// Stop stops the background expiration goroutine
func (m *Manager) Stop() {
	close(m.stopChan)
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.wg.Wait()
	log.Println("TTL manager stopped")
}

// expirationLoop runs in the background and deletes expired keys
func (m *Manager) expirationLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ticker.C:
			m.checkAndDeleteExpired()
		case <-m.stopChan:
			return
		}
	}
}

// checkAndDeleteExpired checks for and deletes expired keys
func (m *Manager) checkAndDeleteExpired() {
	start := time.Now()
	deleted := m.store.DeleteExpired()
	elapsed := time.Since(start)

	// Update stats
	m.mu.Lock()
	m.stats.TotalExpired += int64(deleted)
	m.stats.LastCheckTime = start
	m.stats.LastExpiredCount = deleted
	m.stats.ChecksPerformed++
	m.mu.Unlock()

	if deleted > 0 {
		log.Printf("Expired %d keys in %v", deleted, elapsed)
	}
}

// ForceCheck manually triggers an expiration check
func (m *Manager) ForceCheck() int {
	m.checkAndDeleteExpired()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats.LastExpiredCount
}

// Stats returns current expiration statistics
func (m *Manager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// ResetStats resets the statistics counters
func (m *Manager) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats = Stats{}
}