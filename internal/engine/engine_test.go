// internal/engine/engine_test.go
package engine

import (
	"testing"
)

func TestEngine_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()

	engine, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Set a value
	if err := engine.Set("key1", "value1"); err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	// Get the value
	val, ok := engine.Get("key1")
	if !ok {
		t.Fatal("Expected key to exist")
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %s", val)
	}
}

func TestEngine_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	engine, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Set and delete
	_ = engine.Set("key1", "value1")
	deleted, err := engine.Delete("key1")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}
	if !deleted {
		t.Error("Expected key to be deleted")
	}

	// Verify deletion
	_, ok := engine.Get("key1")
	if ok {
		t.Error("Expected key to not exist after deletion")
	}

	// Delete non-existent key
	deleted, err = engine.Delete("nonexistent")
	if err != nil {
		t.Fatalf("Failed to delete non-existent key: %v", err)
	}
	if deleted {
		t.Error("Expected delete to return false for non-existent key")
	}
}

func TestEngine_Clear(t *testing.T) {
	tmpDir := t.TempDir()

	engine, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Set multiple values
	_ = engine.Set("key1", "value1")
	_ = engine.Set("key2", "value2")
	_ = engine.Set("key3", "value3")

	if engine.Len() != 3 {
		t.Errorf("Expected 3 keys, got %d", engine.Len())
	}

	// Clear
	if err := engine.Clear(); err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	if engine.Len() != 0 {
		t.Errorf("Expected 0 keys after clear, got %d", engine.Len())
	}
}

func TestEngine_Recovery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create engine and write data
	engine1, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	_ = engine1.Set("key1", "value1")
	_ = engine1.Set("key2", "value2")
	_, _ = engine1.Delete("key1")
	_ = engine1.Set("key3", "value3")

	// Close engine (simulating restart)
	if err := engine1.Close(); err != nil {
		t.Fatalf("Failed to close engine: %v", err)
	}

	// Create new engine (should recover from WAL)
	engine2, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine2.Close()

	// Verify recovered state
	_, ok := engine2.Get("key1")
	if ok {
		t.Error("key1 should not exist after recovery (was deleted)")
	}

	val, ok := engine2.Get("key2")
	if !ok || val != "value2" {
		t.Errorf("Expected key2=value2, got %s (exists: %v)", val, ok)
	}

	val, ok = engine2.Get("key3")
	if !ok || val != "value3" {
		t.Errorf("Expected key3=value3, got %s (exists: %v)", val, ok)
	}

	if engine2.Len() != 2 {
		t.Errorf("Expected 2 keys after recovery, got %d", engine2.Len())
	}
}

func TestEngine_RecoveryWithClear(t *testing.T) {
	tmpDir := t.TempDir()

	// Create engine and write data
	engine1, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	_ = engine1.Set("key1", "value1")
	_ = engine1.Set("key2", "value2")
	_ = engine1.Clear()
	_ = engine1.Set("key3", "value3")
	engine1.Close()

	// Recover
	engine2, err := New(Options{WALPath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine2.Close()

	// Should only have key3
	if engine2.Len() != 1 {
		t.Errorf("Expected 1 key after recovery, got %d", engine2.Len())
	}

	val, ok := engine2.Get("key3")
	if !ok || val != "value3" {
		t.Errorf("Expected key3=value3, got %s (exists: %v)", val, ok)
	}
}

func TestEngine_MultipleCycles(t *testing.T) {
	tmpDir := t.TempDir()

	// Cycle 1
	e1, _ := New(Options{WALPath: tmpDir})
	_ = e1.Set("key1", "v1")
	e1.Close()

	// Cycle 2
	e2, _ := New(Options{WALPath: tmpDir})
	_ = e2.Set("key2", "v2")
	e2.Close()

	// Cycle 3
	e3, _ := New(Options{WALPath: tmpDir})
	_ = e3.Set("key3", "v3")
	e3.Close()

	// Verify all data persisted
	e4, _ := New(Options{WALPath: tmpDir})
	defer e4.Close()

	if e4.Len() != 3 {
		t.Errorf("Expected 3 keys, got %d", e4.Len())
	}

	for i := 1; i <= 3; i++ {
		key := "key" + string(rune('0'+i))
		val, ok := e4.Get(key)
		if !ok {
			t.Errorf("Key %s should exist", key)
		}
		expected := "v" + string(rune('0'+i))
		if val != expected {
			t.Errorf("Expected %s=%s, got %s", key, expected, val)
		}
	}
}

func BenchmarkEngine_Set(b *testing.B) {
	tmpDir := b.TempDir()
	engine, _ := New(Options{WALPath: tmpDir, SyncMode: false})
	defer engine.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Set("key", "value")
	}
}

func BenchmarkEngine_Get(b *testing.B) {
	tmpDir := b.TempDir()
	engine, _ := New(Options{WALPath: tmpDir})
	defer engine.Close()

	_ = engine.Set("key", "value")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = engine.Get("key")
	}
}
