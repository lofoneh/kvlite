// internal/snapshot/snapshot_test.go
package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshot_CreateAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	writer, err := NewWriter(Options{Path: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Create test data
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// Create snapshot
	if err := writer.Create(data); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Load snapshot
	snapshot, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}
	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil")
		return
	}

	if snapshot.KeyCount != len(data) {
		t.Errorf("Expected %d keys, got %d", len(data), snapshot.KeyCount)
	}

	for k, v := range data {
		if snapshot.Data[k] != v {
			t.Errorf("Key %s: expected %s, got %s", k, v, snapshot.Data[k])
		}
	}
}

func TestSnapshot_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	writer, err := NewWriter(Options{Path: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	data := map[string]string{"key": "value"}

	// Create first snapshot
	if err := writer.Create(data); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Create second snapshot (should replace first atomically)
	data2 := map[string]string{"key": "value2", "key2": "value2"}
	if err := writer.Create(data2); err != nil {
		t.Fatalf("Failed to create second snapshot: %v", err)
	}

	// Load and verify we got the second snapshot
	snapshot, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	if snapshot.Data["key"] != "value2" {
		t.Errorf("Expected value2, got %s", snapshot.Data["key"])
	}

	if len(snapshot.Data) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(snapshot.Data))
	}

	// Verify no temp files left behind
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".tmp" {
			t.Errorf("Found temp file: %s", file.Name())
		}
	}
}

func TestSnapshot_NoSnapshotExists(t *testing.T) {
	tmpDir := t.TempDir()

	snapshot, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if snapshot != nil {
		t.Error("Expected nil snapshot when none exists")
	}
}

func TestSnapshot_Exists(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially should not exist
	if Exists(tmpDir) {
		t.Error("Snapshot should not exist initially")
	}

	// Create snapshot
	writer, _ := NewWriter(Options{Path: tmpDir})
	data := map[string]string{"key": "value"}
	_ = writer.Create(data)

	// Now should exist
	if !Exists(tmpDir) {
		t.Error("Snapshot should exist after creation")
	}
}

func TestSnapshot_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	writer, _ := NewWriter(Options{Path: tmpDir})
	data := map[string]string{"key": "value"}
	_ = writer.Create(data)

	// Delete snapshot
	if err := Delete(tmpDir); err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}

	// Verify deleted
	if Exists(tmpDir) {
		t.Error("Snapshot should not exist after deletion")
	}

	// Delete again should not error
	if err := Delete(tmpDir); err != nil {
		t.Errorf("Delete on non-existent snapshot should not error: %v", err)
	}
}

func TestSnapshot_Size(t *testing.T) {
	tmpDir := t.TempDir()

	writer, _ := NewWriter(Options{Path: tmpDir})
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	_ = writer.Create(data)

	size, err := Size(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get size: %v", err)
	}

	if size == 0 {
		t.Error("Expected non-zero size")
	}
}

func TestSnapshot_Info(t *testing.T) {
	tmpDir := t.TempDir()

	writer, _ := NewWriter(Options{Path: tmpDir})
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	_ = writer.Create(data)

	info, err := Info(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get info: %v", err)
	}

	if info == nil {
		t.Fatal("Expected info, got nil")
		return
	}

	if info.KeyCount != 3 {
		t.Errorf("Expected 3 keys, got %d", info.KeyCount)
	}

	if info.Version != 1 {
		t.Errorf("Expected version 1, got %d", info.Version)
	}

	if info.Size == 0 {
		t.Error("Expected non-zero size")
	}
}

func TestSnapshot_ExportImport(t *testing.T) {
	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export.json")

	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	// Export
	if err := Export(data, exportPath); err != nil {
		t.Fatalf("Failed to export: %v", err)
	}

	// Import
	imported, err := Import(exportPath)
	if err != nil {
		t.Fatalf("Failed to import: %v", err)
	}

	// Verify
	if len(imported) != len(data) {
		t.Errorf("Expected %d keys, got %d", len(data), len(imported))
	}

	for k, v := range data {
		if imported[k] != v {
			t.Errorf("Key %s: expected %s, got %s", k, v, imported[k])
		}
	}
}

func TestSnapshot_Verify(t *testing.T) {
	tmpDir := t.TempDir()

	writer, _ := NewWriter(Options{Path: tmpDir})
	data := map[string]string{"key": "value"}
	_ = writer.Create(data)

	// Verify valid snapshot
	if err := Verify(tmpDir); err != nil {
		t.Errorf("Valid snapshot failed verification: %v", err)
	}

	// Verify non-existent snapshot
	if err := Verify(t.TempDir()); err == nil {
		t.Error("Expected error for non-existent snapshot")
	}
}

func TestSnapshot_LargeDataset(t *testing.T) {
	tmpDir := t.TempDir()

	writer, _ := NewWriter(Options{Path: tmpDir})

	// Create large dataset
	data := make(map[string]string)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		data[key] = value
	}

	// Create snapshot
	if err := writer.Create(data); err != nil {
		t.Fatalf("Failed to create large snapshot: %v", err)
	}

	// Load and verify
	snapshot, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load large snapshot: %v", err)
	}

	if len(snapshot.Data) != 10000 {
		t.Errorf("Expected 10000 keys, got %d", len(snapshot.Data))
	}
}

func TestSnapshot_EmptyData(t *testing.T) {
	tmpDir := t.TempDir()

	writer, _ := NewWriter(Options{Path: tmpDir})
	data := make(map[string]string)

	if err := writer.Create(data); err != nil {
		t.Fatalf("Failed to create empty snapshot: %v", err)
	}

	snapshot, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load empty snapshot: %v", err)
	}

	if len(snapshot.Data) != 0 {
		t.Errorf("Expected 0 keys, got %d", len(snapshot.Data))
	}
}

func BenchmarkSnapshot_Create(b *testing.B) {
	tmpDir := b.TempDir()
	writer, _ := NewWriter(Options{Path: tmpDir})

	data := make(map[string]string)
	for i := 0; i < 1000; i++ {
		data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = writer.Create(data)
	}
}

func BenchmarkSnapshot_Load(b *testing.B) {
	tmpDir := b.TempDir()
	writer, _ := NewWriter(Options{Path: tmpDir})

	data := make(map[string]string)
	for i := 0; i < 1000; i++ {
		data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}
	_ = writer.Create(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Load(tmpDir)
	}
}
