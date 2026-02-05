// internal/engine/engine.go
package engine

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lofoneh/kvlite/internal/analytics"
	"github.com/lofoneh/kvlite/internal/snapshot"
	"github.com/lofoneh/kvlite/internal/store"
	"github.com/lofoneh/kvlite/internal/ttl"
	"github.com/lofoneh/kvlite/internal/wal"
)

// Engine coordinates the in-memory store, WAL, snapshots, TTL, and analytics for persistence
type Engine struct {
	store            *store.Store
	wal              *wal.WAL
	snapshotWriter   *snapshot.Writer
	ttlManager       *ttl.Manager
	analytics        *analytics.Tracker
	scheduler        *analytics.SmartScheduler
	mu               sync.RWMutex // Protects compaction operations
	compactionTicker *time.Ticker
	stopCompaction   chan struct{}

	// Compaction thresholds
	maxWALEntries int64
	maxWALSize    int64
	walEntryCount int64 // Track number of entries

	// Analytics
	enableAnalytics bool
	requestCounter  int64
	lastRateCheck   time.Time
}

// Options for creating an Engine
type Options struct {
	WALPath            string        // Path for WAL files
	SyncMode           bool          // Sync to disk after every write
	MaxWALEntries      int64         // Trigger compaction after this many entries (default: 10000)
	MaxWALSize         int64         // Trigger compaction after this size in bytes (default: 10MB)
	CompactionInterval time.Duration // How often to check for compaction (default: 1 minute)
	TTLCheckInterval   time.Duration // How often to check for expired keys (default: 1 second)
	EnableAnalytics    bool          // Enable AI-powered analytics and smart scheduling
}

// New creates a new Engine and recovers from snapshot + WAL if they exist
func New(opts Options) (*Engine, error) {
	// Set defaults
	if opts.MaxWALEntries == 0 {
		opts.MaxWALEntries = 10000 // 10K entries
	}
	if opts.MaxWALSize == 0 {
		opts.MaxWALSize = 10 * 1024 * 1024 // 10MB
	}
	if opts.CompactionInterval == 0 {
		opts.CompactionInterval = 1 * time.Minute
	}
	if opts.TTLCheckInterval == 0 {
		opts.TTLCheckInterval = 1 * time.Second
	}

	// Create store
	st := store.New()

	// Create analytics if enabled
	var analyticsTracker *analytics.Tracker
	var smartScheduler *analytics.SmartScheduler
	if opts.EnableAnalytics {
		analyticsTracker = analytics.NewTracker(100)
		smartScheduler = analytics.NewSmartScheduler()
		log.Println("Analytics enabled")
	}

	// Create WAL
	w, err := wal.New(wal.Options{
		Path:     opts.WALPath,
		SyncMode: opts.SyncMode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	// Create snapshot writer
	sw, err := snapshot.NewWriter(snapshot.Options{
		Path: opts.WALPath,
	})
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to create snapshot writer: %w", err)
	}

	// Create TTL manager
	ttlMgr := ttl.NewManager(st, ttl.Options{
		CheckInterval: opts.TTLCheckInterval,
	})

	engine := &Engine{
		store:           st,
		wal:             w,
		snapshotWriter:  sw,
		ttlManager:      ttlMgr,
		analytics:       analyticsTracker,
		scheduler:       smartScheduler,
		maxWALEntries:   opts.MaxWALEntries,
		maxWALSize:      opts.MaxWALSize,
		stopCompaction:  make(chan struct{}),
		enableAnalytics: opts.EnableAnalytics,
		lastRateCheck:   time.Now(),
	}

	// Recover from snapshot and WAL
	if err := engine.recover(opts.WALPath); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to recover: %w", err)
	}

	// Start background processes
	engine.compactionTicker = time.NewTicker(opts.CompactionInterval)
	go engine.compactionLoop()

	ttlMgr.Start()

	return engine, nil
}

// recover loads snapshot (if exists) and replays WAL
func (e *Engine) recover(path string) error {
	log.Println("Starting recovery...")

	// Step 1: Load snapshot if it exists
	snap, err := snapshot.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load snapshot: %w", err)
	}

	if snap != nil {
		log.Printf("Loading snapshot with %d keys...", snap.KeyCount)
		for key, value := range snap.Data {
			e.store.Set(key, value)
		}
		log.Printf("Snapshot loaded: %d keys", snap.KeyCount)
	} else {
		log.Println("No snapshot found, starting fresh")
	}

	// Step 2: Replay WAL for operations after snapshot
	log.Println("Replaying WAL...")
	walCount := 0
	err = e.wal.Replay(func(record *wal.Record) error {
		switch record.Op {
		case wal.OpSet:
			e.store.Set(record.Key, record.Value)
		case wal.OpDelete:
			e.store.Delete(record.Key)
		case wal.OpClear:
			e.store.Clear()
		default:
			return fmt.Errorf("unknown operation: %s", record.Op)
		}
		walCount++
		e.walEntryCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to replay WAL: %w", err)
	}

	log.Printf("Recovery complete: %d keys in store, %d WAL entries replayed",
		e.store.Len(), walCount)
	return nil
}

// Set stores a key-value pair and writes to WAL
func (e *Engine) Set(key, value string) error {
	// Record analytics
	if e.enableAnalytics && e.analytics != nil {
		e.analytics.RecordWrite(key)
		e.trackRequestRate()
	}

	// Write to WAL first (durability)
	record := wal.NewRecord(wal.OpSet, key, value)
	if err := e.wal.Write(record); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Then update in-memory store
	e.store.Set(key, value)

	// Increment WAL entry count
	e.mu.Lock()
	e.walEntryCount++
	e.mu.Unlock()

	return nil
}

// Get retrieves a value by key
func (e *Engine) Get(key string) (string, bool) {
	// Record analytics
	if e.enableAnalytics && e.analytics != nil {
		e.analytics.RecordRead(key)
		e.trackRequestRate()
	}

	return e.store.Get(key) // Store handles lazy expiration
}

// SetWithTTL stores a key-value pair with TTL and writes to WAL
func (e *Engine) SetWithTTL(key, value string, ttl time.Duration) error {
	// Write to WAL first (durability)
	record := wal.NewRecord(wal.OpSet, key, value)
	if err := e.wal.Write(record); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Then update in-memory store with TTL
	e.store.SetWithTTL(key, value, ttl)

	// Increment WAL entry count
	e.mu.Lock()
	e.walEntryCount++
	e.mu.Unlock()

	return nil
}

// Expire sets TTL on an existing key
func (e *Engine) Expire(key string, ttl time.Duration) bool {
	return e.store.Expire(key, ttl)
}

// Persist removes TTL from a key
func (e *Engine) Persist(key string) bool {
	return e.store.Persist(key)
}

// TTL returns the remaining time to live for a key
func (e *Engine) TTL(key string) time.Duration {
	return e.store.TTL(key)
}

// Keys returns all keys matching the pattern
func (e *Engine) Keys(pattern string) []string {
	return e.store.Keys(pattern)
}

// Scan returns keys with pagination
func (e *Engine) Scan(cursor int, pattern string, count int) (int, []string, bool) {
	return e.store.Scan(cursor, pattern, count)
}

// Delete removes a key-value pair and writes to WAL
func (e *Engine) Delete(key string) (bool, error) {
	// Check if key exists
	_, exists := e.store.Get(key)
	if !exists {
		return false, nil
	}

	// Write to WAL first
	record := wal.NewRecord(wal.OpDelete, key, "")
	if err := e.wal.Write(record); err != nil {
		return false, fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Then delete from in-memory store
	e.store.Delete(key)

	// Increment WAL entry count
	e.mu.Lock()
	e.walEntryCount++
	e.mu.Unlock()

	return true, nil
}

// Clear removes all keys and writes to WAL
func (e *Engine) Clear() error {
	// Write to WAL first
	record := wal.NewRecord(wal.OpClear, "", "")
	if err := e.wal.Write(record); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Then clear in-memory store
	e.store.Clear()

	// Increment WAL entry count
	e.mu.Lock()
	e.walEntryCount++
	e.mu.Unlock()

	return nil
}

// Len returns the number of keys in the store
func (e *Engine) Len() int {
	return e.store.Len()
}

// Sync forces a sync of the WAL to disk
func (e *Engine) Sync() error {
	return e.wal.Sync()
}

// Close closes the engine, WAL, and TTL manager
func (e *Engine) Close() error {
	log.Println("Closing engine...")

	// Stop TTL manager
	if e.ttlManager != nil {
		e.ttlManager.Stop()
	}

	// Stop compaction loop
	close(e.stopCompaction)
	if e.compactionTicker != nil {
		e.compactionTicker.Stop()
	}

	if err := e.wal.Close(); err != nil {
		return fmt.Errorf("failed to close WAL: %w", err)
	}
	log.Println("Engine closed")
	return nil
}

// WALSize returns the current size of the WAL file in bytes
func (e *Engine) WALSize() (int64, error) {
	return e.wal.Size()
}

// WALPath returns the path to the WAL file
func (e *Engine) WALPath() string {
	return e.wal.Path()
}

// compactionLoop runs in the background and triggers compaction when needed
func (e *Engine) compactionLoop() {
	for {
		select {
		case <-e.compactionTicker.C:
			if e.needsCompaction() {
				log.Println("Compaction triggered by background checker")
				if err := e.Compact(); err != nil {
					log.Printf("Compaction failed: %v", err)
				}
			}
		case <-e.stopCompaction:
			return
		}
	}
}

// needsCompaction checks if compaction should be triggered
func (e *Engine) needsCompaction() bool {
	// Read immutable config fields without lock (set once during init, never change)
	// This ensures consistent access pattern with Set/Get which also read without lock
	enableAnalytics := e.enableAnalytics
	scheduler := e.scheduler

	e.mu.RLock()
	walEntryCount := e.walEntryCount
	maxWALEntries := e.maxWALEntries
	maxWALSize := e.maxWALSize
	e.mu.RUnlock()

	// Check hard limits first
	if walEntryCount >= maxWALEntries {
		return true
	}

	size, err := e.wal.Size()
	if err == nil && size >= maxWALSize {
		return true
	}

	// If analytics enabled, use smart scheduling
	if enableAnalytics && scheduler != nil {
		score := scheduler.ShouldCompactNow()
		// If score > 0.7 and we're approaching limits, compact
		if score > 0.7 && (walEntryCount >= maxWALEntries/2 || (err == nil && size >= maxWALSize/2)) {
			return true
		}
	}

	return false
}

// Compact creates a snapshot and truncates the WAL
func (e *Engine) Compact() error {
	// Read immutable config fields without lock (set once during init, never change)
	enableAnalytics := e.enableAnalytics
	scheduler := e.scheduler

	e.mu.Lock()
	defer e.mu.Unlock()

	log.Println("Starting compaction...")
	start := time.Now()
	keyCount := e.store.Len()
	walSizeBefore, _ := e.wal.Size()

	// Get current store state
	data := make(map[string]string)
	e.store.Range(func(key, value string) bool {
		data[key] = value
		return true
	})

	// Create snapshot (atomic write)
	if err := e.snapshotWriter.Create(data); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Truncate WAL (all data is now in snapshot)
	if err := e.wal.Truncate(); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	// Reset entry count
	e.walEntryCount = 0

	elapsed := time.Since(start)
	log.Printf("Compaction complete: %d keys compacted in %v", len(data), elapsed)

	// Record compaction event for analytics
	if enableAnalytics && scheduler != nil {
		event := analytics.CompactionEvent{
			Timestamp:    start,
			Hour:         start.Hour(),
			DayOfWeek:    int(start.Weekday()),
			RequestRate:  0, // Would need to track this
			Duration:     elapsed,
			KeyCount:     keyCount,
			WALSize:      walSizeBefore,
			UserImpact:   elapsed.Seconds() * 1000, // ms
			WasAutomatic: true,
		}
		scheduler.RecordCompaction(event)
	}

	return nil
}

// ForceCompact manually triggers compaction
func (e *Engine) ForceCompact() error {
	return e.Compact()
}

// CompactionStats returns statistics about compaction state
func (e *Engine) CompactionStats() map[string]interface{} {
	// Read immutable config fields without lock (set once during init, never change)
	enableAnalytics := e.enableAnalytics
	analyticsTracker := e.analytics
	scheduler := e.scheduler

	e.mu.RLock()
	walEntryCount := e.walEntryCount
	maxWALEntries := e.maxWALEntries
	maxWALSize := e.maxWALSize
	e.mu.RUnlock()

	walSize, _ := e.wal.Size()
	ttlStats := e.ttlManager.Stats()

	stats := map[string]interface{}{
		"wal_entries":       walEntryCount,
		"wal_size":          walSize,
		"max_wal_entries":   maxWALEntries,
		"max_wal_size":      maxWALSize,
		"needs_compaction":  walEntryCount >= maxWALEntries || walSize >= maxWALSize,
		"ttl_total_expired": ttlStats.TotalExpired,
		"ttl_last_check":    ttlStats.LastCheckTime,
		"ttl_checks":        ttlStats.ChecksPerformed,
	}

	// Add analytics stats if enabled
	if enableAnalytics && analyticsTracker != nil {
		globalStats := analyticsTracker.GetGlobalStats()
		stats["analytics_enabled"] = true
		stats["total_reads"] = globalStats["total_reads"]
		stats["total_writes"] = globalStats["total_writes"]
		stats["read_write_ratio"] = globalStats["read_write_ratio"]

		if scheduler != nil {
			stats["should_compact_score"] = scheduler.ShouldCompactNow()
		}
	}

	return stats
}

// trackRequestRate tracks request rate for smart scheduling
func (e *Engine) trackRequestRate() {
	if !e.enableAnalytics || e.scheduler == nil {
		return
	}

	e.mu.Lock()
	e.requestCounter++

	// Calculate rate every second
	now := time.Now()
	elapsed := now.Sub(e.lastRateCheck)

	if elapsed >= 1*time.Second {
		rate := float64(e.requestCounter) / elapsed.Seconds()
		e.scheduler.RecordRequestRate(rate)
		e.requestCounter = 0
		e.lastRateCheck = now
	}
	e.mu.Unlock()
}

// GetKeyStats returns analytics for a specific key
func (e *Engine) GetKeyStats(key string) *analytics.KeyStats {
	if !e.enableAnalytics || e.analytics == nil {
		return nil
	}
	return e.analytics.GetStats(key)
}

// GetHotKeys returns the N most accessed keys
func (e *Engine) GetHotKeys(n int) []*analytics.KeyStats {
	if !e.enableAnalytics || e.analytics == nil {
		return nil
	}
	return e.analytics.GetHotKeys(n)
}

// SuggestTTL suggests TTL for a key based on access patterns
func (e *Engine) SuggestTTL(key string) time.Duration {
	if !e.enableAnalytics || e.analytics == nil {
		return 0
	}
	return e.analytics.SuggestTTL(key)
}

// DetectAnomalies detects unusual access patterns
func (e *Engine) DetectAnomalies() []string {
	if !e.enableAnalytics || e.analytics == nil {
		return nil
	}
	return e.analytics.DetectAnomalies()
}
