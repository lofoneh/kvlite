// internal/ttl/ttl_test.go
package ttl

import (
	"sync"
	"testing"
	"time"
)

// MockStore for testing
type MockStore struct {
	mu            sync.RWMutex
	expiredCount  int
	deleteExpired func() int
}

func (m *MockStore) DeleteExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteExpired != nil {
		return m.deleteExpired()
	}
	return m.expiredCount
}

func TestManager_Start_Stop(t *testing.T) {
	store := &MockStore{}
	mgr := NewManager(store, Options{
		CheckInterval: 100 * time.Millisecond,
	})

	mgr.Start()
	time.Sleep(50 * time.Millisecond)
	mgr.Stop()

	// Should not panic
}

func TestManager_AutomaticExpiration(t *testing.T) {
	callCount := 0
	store := &MockStore{
		deleteExpired: func() int {
			callCount++
			return 5
		},
	}

	mgr := NewManager(store, Options{
		CheckInterval: 50 * time.Millisecond,
	})

	mgr.Start()
	time.Sleep(150 * time.Millisecond) // Should trigger ~3 checks
	mgr.Stop()

	if callCount < 2 {
		t.Errorf("Expected at least 2 checks, got %d", callCount)
	}

	stats := mgr.Stats()
	if stats.TotalExpired == 0 {
		t.Error("Expected some expired keys in stats")
	}
}

func TestManager_ForceCheck(t *testing.T) {
	store := &MockStore{
		deleteExpired: func() int {
			return 10
		},
	}

	mgr := NewManager(store, Options{
		CheckInterval: 1 * time.Hour, // Won't trigger automatically
	})

	mgr.Start()
	defer mgr.Stop()

	deleted := mgr.ForceCheck()
	if deleted != 10 {
		t.Errorf("Expected 10 deleted, got %d", deleted)
	}

	stats := mgr.Stats()
	if stats.TotalExpired != 10 {
		t.Errorf("Expected 10 total expired, got %d", stats.TotalExpired)
	}
}

func TestManager_Stats(t *testing.T) {
	store := &MockStore{
		deleteExpired: func() int {
			return 3
		},
	}

	mgr := NewManager(store, Options{
		CheckInterval: 50 * time.Millisecond,
	})

	mgr.Start()
	time.Sleep(100 * time.Millisecond)
	mgr.Stop()

	stats := mgr.Stats()
	
	if stats.ChecksPerformed == 0 {
		t.Error("Expected some checks performed")
	}

	if stats.TotalExpired == 0 {
		t.Error("Expected some total expired")
	}

	if stats.LastCheckTime.IsZero() {
		t.Error("Expected last check time to be set")
	}
}

func TestManager_ResetStats(t *testing.T) {
	store := &MockStore{
		deleteExpired: func() int {
			return 5
		},
	}

	mgr := NewManager(store, Options{
		CheckInterval: 50 * time.Millisecond,
	})

	mgr.Start()
	time.Sleep(100 * time.Millisecond)
	mgr.Stop()

	// Stats should have values
	stats := mgr.Stats()
	if stats.TotalExpired == 0 {
		t.Error("Expected non-zero stats before reset")
	}

	// Reset
	mgr.ResetStats()
	stats = mgr.Stats()

	if stats.TotalExpired != 0 {
		t.Error("Expected zero stats after reset")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	store := &MockStore{
		deleteExpired: func() int {
			return 1
		},
	}

	mgr := NewManager(store, Options{
		CheckInterval: 10 * time.Millisecond,
	})

	mgr.Start()
	defer mgr.Stop()

	// Concurrent stats access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				mgr.Stats()
				mgr.ForceCheck()
			}
		}()
	}

	wg.Wait()
}