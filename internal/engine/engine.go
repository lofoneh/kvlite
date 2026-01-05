// internal/engine/engine.go
package engine

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lofoneh/kvlite/internal/snapshot"
	"github.com/lofoneh/kvlite/internal/store"
	"github.com/lofoneh/kvlite/internal/wal"
)

// Engine coordinates the in-memory store, WAL, and snapshots for persistence
type Engine struct {
	store            *store.Store
	wal              *wal.WAL
	snapshotWriter   *snapshot.Writer
	mu               sync.RWMutex // Protects compaction operations
	compactionTicker *time.Ticker
	stopCompaction   chan struct{}
	
	// Compaction thresholds
	maxWALEntries int64
	maxWALSize    int64
	walEntryCount int64 // Track number of entries
}

// Options for creating an Engine
type Options struct {
	WALPath            string // Path for WAL files
	SyncMode           bool   // Sync to disk after every write
	MaxWALEntries      int64  // Trigger compaction after this many entries (default: 10000)
	MaxWALSize         int64  // Trigger compaction after this size in bytes (default: 10MB)
	CompactionInterval time.Duration // How often to check for compaction (default: 1 minute)
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

	// Create store
	st := store.New()

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

	engine := &Engine{
		store:            st,
		wal:              w,
		snapshotWriter:   sw,
		maxWALEntries:    opts.MaxWALEntries,
		maxWALSize:       opts.MaxWALSize,
		stopCompaction:   make(chan struct{}),
	}

	// Recover from snapshot and WAL
	if err := engine.recover(opts.WALPath); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to recover: %w", err)
	}

	// Start background compaction checker
	engine.compactionTicker = time.NewTicker(opts.CompactionInterval)
	go engine.compactionLoop()

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
	return e.store.Get(key)
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

// Close closes the engine and WAL
func (e *Engine) Close() error {
	log.Println("Closing engine...")
	
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
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check entry count threshold
	if e.walEntryCount >= e.maxWALEntries {
		return true
	}

	// Check size threshold
	size, err := e.wal.Size()
	if err == nil && size >= e.maxWALSize {
		return true
	}

	return false
}

// Compact creates a snapshot and truncates the WAL
func (e *Engine) Compact() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Println("Starting compaction...")
	start := time.Now()

	// Get current store state (this is a snapshot of keys, not a deep copy)
	// We need to create a copy to avoid race conditions
	data := make(map[string]string)
	
	// This is safe because store operations are already protected by RWMutex
	// We just need to copy the map to ensure the snapshot is consistent
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

	return nil
}

// ForceCompact manually triggers compaction
func (e *Engine) ForceCompact() error {
	return e.Compact()
}

// CompactionStats returns statistics about compaction state
func (e *Engine) CompactionStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	walSize, _ := e.wal.Size()
	
	return map[string]interface{}{
		"wal_entries":      e.walEntryCount,
		"wal_size":         walSize,
		"max_wal_entries":  e.maxWALEntries,
		"max_wal_size":     e.maxWALSize,
		"needs_compaction": e.walEntryCount >= e.maxWALEntries || walSize >= e.maxWALSize,
	}
}