// internal/wal/wal.go
package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// WAL (Write-Ahead Log) provides durable storage for operations
type WAL struct {
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	path     string
	syncMode bool // If true, sync after every write
}

// Options for creating a WAL
type Options struct {
	Path     string // Directory path for WAL files
	SyncMode bool   // Sync to disk after every write (slower but safer)
}

// New creates a new WAL instance
func New(opts Options) (*WAL, error) {
	if opts.Path == "" {
		opts.Path = "./data"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(opts.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	walPath := filepath.Join(opts.Path, "kvlite.wal")

	// Open file in append mode, create if doesn't exist
	file, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WAL{
		file:     file,
		writer:   bufio.NewWriter(file),
		path:     walPath,
		syncMode: opts.SyncMode,
	}, nil
}

// Write appends a record to the WAL
func (w *WAL) Write(record *Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Encode and write the record
	encoded := record.Encode()
	if _, err := w.writer.WriteString(encoded); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	// Flush buffer
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	// Sync to disk if in sync mode
	if w.syncMode {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync to disk: %w", err)
		}
	}

	return nil
}

// Replay reads all records from the WAL and calls the provided function for each
func (w *WAL) Replay(fn func(*Record) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Open file for reading
	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			// No WAL file exists yet, nothing to replay
			return nil
		}
		return fmt.Errorf("failed to open WAL for replay: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Decode record
		record, err := Decode(line)
		if err != nil {
			return fmt.Errorf("failed to decode record at line %d: %w", lineNum, err)
		}

		// Apply record
		if err := fn(record); err != nil {
			return fmt.Errorf("failed to apply record at line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL: %w", err)
	}

	return nil
}

// Sync forces a sync to disk
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Sync()
}

// Close closes the WAL file
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// Truncate removes all data from the WAL (used after snapshots)
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush and close current file
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if err := w.file.Close(); err != nil {
		return err
	}

	// Truncate the file
	if err := os.Truncate(w.path, 0); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	// Reopen file
	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen WAL after truncate: %w", err)
	}

	w.file = file
	w.writer = bufio.NewWriter(file)

	return nil
}

// Size returns the current size of the WAL file in bytes
func (w *WAL) Size() (int64, error) {
	info, err := os.Stat(w.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// Path returns the path to the WAL file
func (w *WAL) Path() string {
	return w.path
}

// ReadAll reads all records from the WAL file
func ReadAll(path string) ([]*Record, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open WAL: %w", err)
	}
	defer file.Close()

	var records []*Record
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if line == "" {
			continue
		}

		record, err := Decode(line)
		if err != nil {
			return nil, fmt.Errorf("failed to decode record at line %d: %w", lineNum, err)
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("error reading WAL: %w", err)
	}

	return records, nil
}