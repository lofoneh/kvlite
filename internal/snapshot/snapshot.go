// internal/snapshot/snapshot.go
package snapshot

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Snapshot represents a point-in-time backup of the store
type Snapshot struct {
	Timestamp int64             `json:"timestamp"` // Unix nano
	Version   int               `json:"version"`   // Snapshot format version
	KeyCount  int               `json:"key_count"` // Number of keys
	Data      map[string]string `json:"data"`      // The actual key-value data
}

// Options for snapshot operations
type Options struct {
	Path string // Directory for snapshot files
}

// Writer handles creating snapshots
type Writer struct {
	path string
}

// NewWriter creates a new snapshot writer
func NewWriter(opts Options) (*Writer, error) {
	if opts.Path == "" {
		opts.Path = "./data"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(opts.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	return &Writer{
		path: opts.Path,
	}, nil
}

// Create writes a snapshot of the provided data
// Uses atomic write: write to temp file, then rename
func (w *Writer) Create(data map[string]string) error {
	snapshot := &Snapshot{
		Timestamp: time.Now().UnixNano(),
		Version:   1,
		KeyCount:  len(data),
		Data:      data,
	}

	// Create temporary file
	tempPath := filepath.Join(w.path, fmt.Sprintf("kvlite.snapshot.tmp.%d", snapshot.Timestamp))
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp snapshot file: %w", err)
	}

	// Write snapshot
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(snapshot); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to flush snapshot: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to sync snapshot: %w", err)
	}

	file.Close()

	// Atomic rename: this is the critical step for crash safety
	finalPath := filepath.Join(w.path, "kvlite.snapshot")
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename snapshot: %w", err)
	}

	return nil
}

// Load reads and returns a snapshot
func Load(path string) (*Snapshot, error) {
	snapshotPath := filepath.Join(path, "kvlite.snapshot")

	file, err := os.Open(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No snapshot exists yet
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open snapshot: %w", err)
	}
	defer file.Close()

	var snapshot Snapshot
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}

	return &snapshot, nil
}

// Exists checks if a snapshot file exists
func Exists(path string) bool {
	snapshotPath := filepath.Join(path, "kvlite.snapshot")
	_, err := os.Stat(snapshotPath)
	return err == nil
}

// Delete removes a snapshot file
func Delete(path string) error {
	snapshotPath := filepath.Join(path, "kvlite.snapshot")
	err := os.Remove(snapshotPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

// Size returns the size of the snapshot file in bytes
func Size(path string) (int64, error) {
	snapshotPath := filepath.Join(path, "kvlite.snapshot")
	info, err := os.Stat(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to stat snapshot: %w", err)
	}
	return info.Size(), nil
}

// Info returns metadata about a snapshot without loading the full data
func Info(path string) (*SnapshotInfo, error) {
	snapshotPath := filepath.Join(path, "kvlite.snapshot")

	file, err := os.Open(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open snapshot: %w", err)
	}
	defer file.Close()

	// Read just the metadata (not the full data)
	var metadata struct {
		Timestamp int64 `json:"timestamp"`
		Version   int   `json:"version"`
		KeyCount  int   `json:"key_count"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot metadata: %w", err)
	}

	info, _ := os.Stat(snapshotPath)
	return &SnapshotInfo{
		Timestamp: metadata.Timestamp,
		Version:   metadata.Version,
		KeyCount:  metadata.KeyCount,
		Size:      info.Size(),
		Path:      snapshotPath,
	}, nil
}

// SnapshotInfo contains metadata about a snapshot
type SnapshotInfo struct {
	Timestamp int64
	Version   int
	KeyCount  int
	Size      int64
	Path      string
}

// Export writes a snapshot to an arbitrary path (for backup/export)
func Export(data map[string]string, destPath string) error {
	snapshot := &Snapshot{
		Timestamp: time.Now().UnixNano(),
		Version:   1,
		KeyCount:  len(data),
		Data:      data,
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush export: %w", err)
	}

	return file.Sync()
}

// Import reads a snapshot from an arbitrary path
func Import(srcPath string) (map[string]string, error) {
	file, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open import file: %w", err)
	}
	defer file.Close()

	var snapshot Snapshot
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}

	return snapshot.Data, nil
}

// Verify checks if a snapshot file is valid
func Verify(path string) error {
	snapshot, err := Load(path)
	if err != nil {
		return err
	}

	if snapshot == nil {
		return fmt.Errorf("no snapshot found")
	}

	// Basic validation
	if snapshot.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", snapshot.Version)
	}

	if len(snapshot.Data) != snapshot.KeyCount {
		return fmt.Errorf("key count mismatch: expected %d, got %d", 
			snapshot.KeyCount, len(snapshot.Data))
	}

	return nil
}

// Stream writes a snapshot using streaming to handle large datasets
func Stream(data map[string]string, w io.Writer) error {
	snapshot := &Snapshot{
		Timestamp: time.Now().UnixNano(),
		Version:   1,
		KeyCount:  len(data),
		Data:      data,
	}

	encoder := json.NewEncoder(w)
	return encoder.Encode(snapshot)
}