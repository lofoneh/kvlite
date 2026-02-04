// internal/store/store_test.go
package store

import (
	"sync"
	"testing"
)

func TestStore_SetGet(t *testing.T) {
	s := New()

	// Test setting and getting a value
	s.Set("key1", "value1")
	val, ok := s.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	// Test getting non-existent key
	_, ok = s.Get("nonexistent")
	if ok {
		t.Error("expected key to not exist")
	}
}

func TestStore_Delete(t *testing.T) {
	s := New()

	s.Set("key1", "value1")

	// Delete existing key
	existed := s.Delete("key1")
	if !existed {
		t.Error("expected key1 to exist before deletion")
	}

	// Verify deletion
	_, ok := s.Get("key1")
	if ok {
		t.Error("expected key1 to not exist after deletion")
	}

	// Delete non-existent key
	existed = s.Delete("key1")
	if existed {
		t.Error("expected key1 to not exist")
	}
}

func TestStore_Len(t *testing.T) {
	s := New()

	if s.Len() != 0 {
		t.Errorf("expected length 0, got %d", s.Len())
	}

	s.Set("key1", "value1")
	s.Set("key2", "value2")

	if s.Len() != 2 {
		t.Errorf("expected length 2, got %d", s.Len())
	}

	s.Delete("key1")

	if s.Len() != 1 {
		t.Errorf("expected length 1, got %d", s.Len())
	}
}

func TestStore_Clear(t *testing.T) {
	s := New()

	s.Set("key1", "value1")
	s.Set("key2", "value2")
	s.Clear()

	if s.Len() != 0 {
		t.Errorf("expected length 0 after clear, got %d", s.Len())
	}
}

func TestStore_Concurrent(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "key"
				value := "value"
				s.Set(key, value)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.Get("key")
			}
		}()
	}

	// Concurrent deletes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.Delete("key")
			}
		}()
	}

	wg.Wait()
	// If we reach here without deadlock or race condition, test passes
}

func BenchmarkStore_Set(b *testing.B) {
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set("key", "value")
	}
}

func BenchmarkStore_Get(b *testing.B) {
	s := New()
	s.Set("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get("key")
	}
}

func BenchmarkStore_ConcurrentReadWrite(b *testing.B) {
	s := New()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				s.Set("key", "value")
			} else {
				s.Get("key")
			}
			i++
		}
	})
}
