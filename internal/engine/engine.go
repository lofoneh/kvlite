// internal/engine/engine.go
package engine

import (
	"fmt"
	"log"

	"github.com/lofoneh/kvlite/internal/store"
	"github.com/lofoneh/kvlite/internal/wal"
)

// Engine coordinates the in-memory store and WAL for persistence
type Engine struct {
	store *store.Store
	wal   *wal.WAL
}

// Options for creating an Engine
type Options struct {
	WALPath  string // Path for WAL files
	SyncMode bool   // Sync to disk after every write
}

// New creates a new Engine and recovers from WAL if it exists
func New(opts Options) (*Engine, error) {
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

	engine := &Engine{
		store: st,
		wal:   w,
	}

	// Replay WAL to recover state
	if err := engine.recover(); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}

	return engine, nil
}

// recover replays the WAL to restore the store state
func (e *Engine) recover() error {
	log.Println("Starting WAL replay...")

	count := 0
	err := e.wal.Replay(func(record *wal.Record) error {
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
		count++
		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("WAL replay complete: %d operations recovered, %d keys in store", count, e.store.Len())
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