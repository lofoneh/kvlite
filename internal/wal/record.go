// internal/wal/record.go
package wal

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"
)

// OpType represents the type of operation
type OpType string

const (
	OpSet    OpType = "SET"
	OpDelete OpType = "DELETE"
	OpClear  OpType = "CLEAR"
)

// Record represents a single WAL entry
type Record struct {
	Timestamp int64  // Unix timestamp in nanoseconds
	Op        OpType // Operation type
	Key       string // Key (empty for CLEAR)
	Value     string // Value (empty for DELETE and CLEAR)
	Checksum  uint32 // CRC32 checksum for integrity
}

// NewRecord creates a new WAL record
func NewRecord(op OpType, key, value string) *Record {
	r := &Record{
		Timestamp: time.Now().UnixNano(),
		Op:        op,
		Key:       key,
		Value:     value,
	}
	r.Checksum = r.calculateChecksum()
	return r
}

// calculateChecksum computes CRC32 checksum of the record data
func (r *Record) calculateChecksum() uint32 {
	data := fmt.Sprintf("%d|%s|%s|%s", r.Timestamp, r.Op, r.Key, r.Value)
	return crc32.ChecksumIEEE([]byte(data))
}

// Validate checks if the record's checksum is valid
func (r *Record) Validate() error {
	expected := r.calculateChecksum()
	if r.Checksum != expected {
		return fmt.Errorf("checksum mismatch: expected %d, got %d", expected, r.Checksum)
	}
	return nil
}

// escape escapes special characters for safe storage
func escape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslash first
	s = strings.ReplaceAll(s, "|", "\\|")   // Escape pipe
	s = strings.ReplaceAll(s, "\n", "\\n")  // Escape newline
	return s
}

// unescape reverses the escaping
func unescape(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")  // Unescape newline
	s = strings.ReplaceAll(s, "\\|", "|")   // Unescape pipe
	s = strings.ReplaceAll(s, "\\\\", "\\") // Unescape backslash last
	return s
}

// Encode converts the record to a string format for writing to disk
// Format: timestamp|operation|key|value|checksum\n
func (r *Record) Encode() string {
	key := escape(r.Key)
	value := escape(r.Value)
	
	return fmt.Sprintf("%d|%s|%s|%s|%d\n", r.Timestamp, r.Op, key, value, r.Checksum)
}

// Decode parses a string into a Record
func Decode(line string) (*Record, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	// Split carefully - we need to handle escaped pipes
	parts := splitRecord(line)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid record format: expected 5 fields, got %d", len(parts))
	}

	// Parse timestamp
	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Parse operation
	op := OpType(parts[1])
	if op != OpSet && op != OpDelete && op != OpClear {
		return nil, fmt.Errorf("invalid operation: %s", op)
	}

	// Unescape key and value
	key := unescape(parts[2])
	value := unescape(parts[3])

	// Parse checksum
	checksum, err := strconv.ParseUint(parts[4], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid checksum: %w", err)
	}

	record := &Record{
		Timestamp: timestamp,
		Op:        op,
		Key:       key,
		Value:     value,
		Checksum:  uint32(checksum),
	}

	// Validate checksum
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("record validation failed: %w", err)
	}

	return record, nil
}

// splitRecord splits a record line by | but respects escaped pipes
func splitRecord(line string) []string {
	var parts []string
	var current strings.Builder
	escaped := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		
		if c == '\\' {
			current.WriteByte(c)
			escaped = true
			continue
		}
		
		if c == '|' {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		
		current.WriteByte(c)
	}
	
	// Add the last part
	parts = append(parts, current.String())
	
	return parts
}

// String returns a human-readable representation
func (r *Record) String() string {
	t := time.Unix(0, r.Timestamp)
	return fmt.Sprintf("[%s] %s %s=%s (checksum: %d)", 
		t.Format(time.RFC3339), r.Op, r.Key, r.Value, r.Checksum)
}