// internal/wal/wal_test.go
package wal

import (
	"path/filepath"
	"testing"
)

func TestRecord_EncodeDecodeValidate(t *testing.T) {
	tests := []struct {
		name  string
		op    OpType
		key   string
		value string
	}{
		{"simple set", OpSet, "key1", "value1"},
		{"delete", OpDelete, "key1", ""},
		{"clear", OpClear, "", ""},
		{"special chars in key", OpSet, "key|with|pipes", "value"},
		{"special chars in value", OpSet, "key", "value|with|pipes"},
		{"newlines", OpSet, "key\nwith\nnewlines", "value\nwith\nnewlines"},
		{"empty key", OpSet, "", "value"},
		{"long value", OpSet, "key", "a very long value with lots of text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create record
			record := NewRecord(tt.op, tt.key, tt.value)

			// Validate checksum
			if err := record.Validate(); err != nil {
				t.Errorf("Validate() failed: %v", err)
			}

			// Encode
			encoded := record.Encode()

			// Decode
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() failed: %v", err)
			}

			// Compare
			if decoded.Op != record.Op {
				t.Errorf("Op mismatch: got %v, want %v", decoded.Op, record.Op)
			}
			if decoded.Key != record.Key {
				t.Errorf("Key mismatch: got %v, want %v", decoded.Key, record.Key)
			}
			if decoded.Value != record.Value {
				t.Errorf("Value mismatch: got %v, want %v", decoded.Value, record.Value)
			}
			if decoded.Checksum != record.Checksum {
				t.Errorf("Checksum mismatch: got %v, want %v", decoded.Checksum, record.Checksum)
			}
		})
	}
}

func TestRecord_CorruptedChecksum(t *testing.T) {
	record := NewRecord(OpSet, "key", "value")

	// Corrupt the checksum
	record.Checksum = 12345

	if err := record.Validate(); err == nil {
		t.Error("Expected validation error for corrupted checksum, got nil")
	}
}

func TestWAL_WriteAndReplay(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create WAL
	wal, err := New(Options{Path: tmpDir, SyncMode: false})
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Write some records
	records := []*Record{
		NewRecord(OpSet, "key1", "value1"),
		NewRecord(OpSet, "key2", "value2"),
		NewRecord(OpDelete, "key1", ""),
		NewRecord(OpSet, "key3", "value3"),
	}

	for _, record := range records {
		if err := wal.Write(record); err != nil {
			t.Fatalf("Failed to write record: %v", err)
		}
	}

	// Close WAL
	if err := wal.Close(); err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}

	// Reopen and replay
	wal, err = New(Options{Path: tmpDir, SyncMode: false})
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	var replayed []*Record
	err = wal.Replay(func(r *Record) error {
		replayed = append(replayed, r)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to replay WAL: %v", err)
	}

	// Verify replayed records
	if len(replayed) != len(records) {
		t.Fatalf("Expected %d records, got %d", len(records), len(replayed))
	}

	for i, record := range records {
		if replayed[i].Op != record.Op {
			t.Errorf("Record %d: Op mismatch", i)
		}
		if replayed[i].Key != record.Key {
			t.Errorf("Record %d: Key mismatch", i)
		}
		if replayed[i].Value != record.Value {
			t.Errorf("Record %d: Value mismatch", i)
		}
	}
}

func TestWAL_Truncate(t *testing.T) {
	tmpDir := t.TempDir()

	wal, err := New(Options{Path: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Write records
	for i := 0; i < 10; i++ {
		record := NewRecord(OpSet, "key", "value")
		if err := wal.Write(record); err != nil {
			t.Fatalf("Failed to write record: %v", err)
		}
	}

	// Check size before truncate
	sizeBefore, err := wal.Size()
	if err != nil {
		t.Fatalf("Failed to get WAL size: %v", err)
	}
	if sizeBefore == 0 {
		t.Error("WAL size should not be 0 before truncate")
	}

	// Truncate
	if err := wal.Truncate(); err != nil {
		t.Fatalf("Failed to truncate WAL: %v", err)
	}

	// Check size after truncate
	sizeAfter, err := wal.Size()
	if err != nil {
		t.Fatalf("Failed to get WAL size: %v", err)
	}
	if sizeAfter != 0 {
		t.Errorf("WAL size should be 0 after truncate, got %d", sizeAfter)
	}
}

func TestWAL_ReadAll(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "kvlite.wal")

	wal, err := New(Options{Path: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Write records
	expectedRecords := []*Record{
		NewRecord(OpSet, "key1", "value1"),
		NewRecord(OpSet, "key2", "value2"),
		NewRecord(OpDelete, "key1", ""),
	}

	for _, record := range expectedRecords {
		if err := wal.Write(record); err != nil {
			t.Fatalf("Failed to write record: %v", err)
		}
	}
	wal.Close()

	// Read all
	records, err := ReadAll(walPath)
	if err != nil {
		t.Fatalf("Failed to read all records: %v", err)
	}

	if len(records) != len(expectedRecords) {
		t.Fatalf("Expected %d records, got %d", len(expectedRecords), len(records))
	}

	for i, record := range expectedRecords {
		if records[i].Op != record.Op || records[i].Key != record.Key || records[i].Value != record.Value {
			t.Errorf("Record %d mismatch", i)
		}
	}
}

func TestWAL_EmptyReplay(t *testing.T) {
	tmpDir := t.TempDir()

	wal, err := New(Options{Path: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Replay on empty WAL should not error
	count := 0
	err = wal.Replay(func(r *Record) error {
		count++
		return nil
	})

	if err != nil {
		t.Errorf("Replay on empty WAL should not error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 records, got %d", count)
	}
}

func BenchmarkWAL_Write(b *testing.B) {
	tmpDir := b.TempDir()
	wal, _ := New(Options{Path: tmpDir, SyncMode: false})
	defer wal.Close()

	record := NewRecord(OpSet, "key", "value")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wal.Write(record)
	}
}

func BenchmarkWAL_WriteSync(b *testing.B) {
	tmpDir := b.TempDir()
	wal, _ := New(Options{Path: tmpDir, SyncMode: true})
	defer wal.Close()

	record := NewRecord(OpSet, "key", "value")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wal.Write(record)
	}
}
