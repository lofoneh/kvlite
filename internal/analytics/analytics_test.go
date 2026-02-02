// internal/analytics/analytics_test.go
package analytics

import (
	"sync"
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	// Test default history size
	tracker := NewTracker(0)
	if tracker.maxHistorySize != 100 {
		t.Errorf("Expected default maxHistorySize 100, got %d", tracker.maxHistorySize)
	}

	// Test custom history size
	tracker = NewTracker(50)
	if tracker.maxHistorySize != 50 {
		t.Errorf("Expected maxHistorySize 50, got %d", tracker.maxHistorySize)
	}
}

func TestRecordRead(t *testing.T) {
	tracker := NewTracker(10)

	// Record reads
	tracker.RecordRead("key1")
	tracker.RecordRead("key1")
	tracker.RecordRead("key2")

	stats := tracker.GetStats("key1")
	if stats == nil {
		t.Fatal("Expected stats for key1")
	}
	if stats.Reads != 2 {
		t.Errorf("Expected 2 reads, got %d", stats.Reads)
	}
	if stats.Writes != 0 {
		t.Errorf("Expected 0 writes, got %d", stats.Writes)
	}

	stats = tracker.GetStats("key2")
	if stats.Reads != 1 {
		t.Errorf("Expected 1 read, got %d", stats.Reads)
	}
}

func TestRecordWrite(t *testing.T) {
	tracker := NewTracker(10)

	// Record writes
	tracker.RecordWrite("key1")
	tracker.RecordWrite("key1")
	tracker.RecordWrite("key1")

	stats := tracker.GetStats("key1")
	if stats == nil {
		t.Fatal("Expected stats for key1")
	}
	if stats.Writes != 3 {
		t.Errorf("Expected 3 writes, got %d", stats.Writes)
	}
	if stats.Reads != 0 {
		t.Errorf("Expected 0 reads, got %d", stats.Reads)
	}
}

func TestGetStats_NonExistentKey(t *testing.T) {
	tracker := NewTracker(10)

	stats := tracker.GetStats("nonexistent")
	if stats != nil {
		t.Error("Expected nil for nonexistent key")
	}
}

func TestGetStats_ReturnsCopy(t *testing.T) {
	tracker := NewTracker(10)
	tracker.RecordRead("key1")

	stats1 := tracker.GetStats("key1")
	stats2 := tracker.GetStats("key1")

	// Modify the copy
	stats1.Reads = 999

	// Original should be unchanged
	if stats2.Reads == 999 {
		t.Error("GetStats should return a copy, not the original")
	}
}

func TestCircularBuffer(t *testing.T) {
	tracker := NewTracker(5) // Small buffer

	// Record more accesses than buffer size
	for i := 0; i < 10; i++ {
		tracker.RecordRead("key1")
	}

	stats := tracker.GetStats("key1")
	if len(stats.AccessHistory) != 5 {
		t.Errorf("Expected 5 entries in history (buffer size), got %d", len(stats.AccessHistory))
	}
}

func TestGetHotKeys(t *testing.T) {
	tracker := NewTracker(10)

	// Create keys with different access counts
	for i := 0; i < 50; i++ {
		tracker.RecordRead("hot")
	}
	for i := 0; i < 30; i++ {
		tracker.RecordRead("warm")
	}
	for i := 0; i < 10; i++ {
		tracker.RecordRead("cold")
	}

	hotKeys := tracker.GetHotKeys(2)
	if len(hotKeys) != 2 {
		t.Fatalf("Expected 2 hot keys, got %d", len(hotKeys))
	}

	// First should be "hot" with most accesses
	if hotKeys[0].Key != "hot" {
		t.Errorf("Expected first hot key to be 'hot', got '%s'", hotKeys[0].Key)
	}
	if hotKeys[1].Key != "warm" {
		t.Errorf("Expected second hot key to be 'warm', got '%s'", hotKeys[1].Key)
	}
}

func TestGetHotKeys_MoreRequestedThanAvailable(t *testing.T) {
	tracker := NewTracker(10)
	tracker.RecordRead("key1")
	tracker.RecordRead("key2")

	hotKeys := tracker.GetHotKeys(10)
	if len(hotKeys) != 2 {
		t.Errorf("Expected 2 hot keys, got %d", len(hotKeys))
	}
}

func TestGetHotKeys_EmptyTracker(t *testing.T) {
	tracker := NewTracker(10)

	hotKeys := tracker.GetHotKeys(5)
	if len(hotKeys) != 0 {
		t.Errorf("Expected 0 hot keys, got %d", len(hotKeys))
	}
}

func TestGetColdKeys(t *testing.T) {
	tracker := NewTracker(10)

	// Record an access
	tracker.RecordRead("old_key")

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Record another access
	tracker.RecordRead("new_key")

	// Get cold keys (not accessed in last 25ms)
	coldKeys := tracker.GetColdKeys(25 * time.Millisecond)

	found := false
	for _, k := range coldKeys {
		if k.Key == "old_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected old_key to be in cold keys")
	}
}

func TestGetReadWriteRatio(t *testing.T) {
	tracker := NewTracker(10)

	// Test non-existent key
	ratio, reads, writes := tracker.GetReadWriteRatio("nonexistent")
	if ratio != 0 || reads != 0 || writes != 0 {
		t.Error("Expected all zeros for nonexistent key")
	}

	// Test key with only reads
	for i := 0; i < 10; i++ {
		tracker.RecordRead("reads_only")
	}
	ratio, reads, writes = tracker.GetReadWriteRatio("reads_only")
	if ratio != 10 || reads != 10 || writes != 0 {
		t.Errorf("Expected ratio=10, reads=10, writes=0; got ratio=%f, reads=%d, writes=%d", ratio, reads, writes)
	}

	// Test key with reads and writes
	for i := 0; i < 8; i++ {
		tracker.RecordRead("mixed")
	}
	for i := 0; i < 2; i++ {
		tracker.RecordWrite("mixed")
	}
	ratio, reads, writes = tracker.GetReadWriteRatio("mixed")
	if ratio != 4.0 || reads != 8 || writes != 2 {
		t.Errorf("Expected ratio=4, reads=8, writes=2; got ratio=%f, reads=%d, writes=%d", ratio, reads, writes)
	}
}

func TestSuggestTTL(t *testing.T) {
	tracker := NewTracker(10)

	// Test non-existent key
	ttl := tracker.SuggestTTL("nonexistent")
	if ttl != 0 {
		t.Errorf("Expected 0 TTL for nonexistent key, got %v", ttl)
	}

	// Test key with high access rate (should suggest long TTL)
	// Many accesses in a short time = high rate
	for i := 0; i < 100; i++ {
		tracker.RecordRead("high_access")
	}
	ttl = tracker.SuggestTTL("high_access")
	// High access rate should return either 24h or 1h depending on timing
	if ttl != 24*time.Hour && ttl != 1*time.Hour {
		t.Errorf("Expected 24h or 1h TTL for high access key, got %v", ttl)
	}

	// Test that SuggestTTL returns a valid duration for any accessed key
	tracker.RecordRead("any_key")
	ttl = tracker.SuggestTTL("any_key")
	validTTLs := []time.Duration{10 * time.Minute, 1 * time.Hour, 24 * time.Hour}
	found := false
	for _, valid := range validTTLs {
		if ttl == valid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected TTL to be one of %v, got %v", validTTLs, ttl)
	}
}

func TestDetectAnomalies_EmptyTracker(t *testing.T) {
	tracker := NewTracker(10)

	anomalies := tracker.DetectAnomalies()
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomalies for empty tracker, got %d", len(anomalies))
	}
}

func TestDetectAnomalies(t *testing.T) {
	tracker := NewTracker(10)

	// Create normal keys with similar access patterns
	for i := 0; i < 10; i++ {
		tracker.RecordRead("normal1")
		tracker.RecordRead("normal2")
		tracker.RecordRead("normal3")
	}

	// Create anomalous key with much higher access
	for i := 0; i < 100; i++ {
		tracker.RecordRead("anomaly")
	}

	anomalies := tracker.DetectAnomalies()

	found := false
	for _, k := range anomalies {
		if k == "anomaly" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'anomaly' to be detected as anomalous")
	}
}

func TestGetGlobalStats(t *testing.T) {
	tracker := NewTracker(10)

	// Record some operations
	for i := 0; i < 10; i++ {
		tracker.RecordRead("key1")
	}
	for i := 0; i < 5; i++ {
		tracker.RecordWrite("key2")
	}

	stats := tracker.GetGlobalStats()

	if stats["total_keys"].(int) != 2 {
		t.Errorf("Expected 2 total keys, got %d", stats["total_keys"])
	}
	if stats["total_reads"].(int64) != 10 {
		t.Errorf("Expected 10 total reads, got %d", stats["total_reads"])
	}
	if stats["total_writes"].(int64) != 5 {
		t.Errorf("Expected 5 total writes, got %d", stats["total_writes"])
	}
	if stats["read_write_ratio"].(float64) != 2.0 {
		t.Errorf("Expected read/write ratio of 2.0, got %f", stats["read_write_ratio"])
	}
}

func TestGetGlobalStats_NoWrites(t *testing.T) {
	tracker := NewTracker(10)
	tracker.RecordRead("key1")

	stats := tracker.GetGlobalStats()
	if stats["read_write_ratio"].(float64) != 1.0 {
		t.Errorf("Expected read/write ratio of 1.0 (reads only), got %f", stats["read_write_ratio"])
	}
}

func TestReset(t *testing.T) {
	tracker := NewTracker(10)

	// Add some data
	tracker.RecordRead("key1")
	tracker.RecordWrite("key2")

	// Reset
	tracker.Reset()

	// Verify everything is cleared
	if tracker.GetStats("key1") != nil {
		t.Error("Expected key1 to be cleared after reset")
	}
	stats := tracker.GetGlobalStats()
	if stats["total_keys"].(int) != 0 {
		t.Error("Expected 0 total keys after reset")
	}
}

func TestRemoveKey(t *testing.T) {
	tracker := NewTracker(10)

	tracker.RecordRead("key1")
	tracker.RecordRead("key2")

	tracker.RemoveKey("key1")

	if tracker.GetStats("key1") != nil {
		t.Error("Expected key1 to be removed")
	}
	if tracker.GetStats("key2") == nil {
		t.Error("Expected key2 to still exist")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracker := NewTracker(100)
	var wg sync.WaitGroup

	// Spawn multiple goroutines reading and writing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tracker.RecordRead("concurrent_key")
				tracker.RecordWrite("concurrent_key")
				tracker.GetStats("concurrent_key")
				tracker.GetHotKeys(5)
			}
		}(i)
	}

	wg.Wait()

	stats := tracker.GetStats("concurrent_key")
	if stats == nil {
		t.Fatal("Expected stats for concurrent_key")
	}

	// Each goroutine does 100 reads and 100 writes
	expectedReads := int64(10 * 100)
	expectedWrites := int64(10 * 100)

	if stats.Reads != expectedReads {
		t.Errorf("Expected %d reads, got %d", expectedReads, stats.Reads)
	}
	if stats.Writes != expectedWrites {
		t.Errorf("Expected %d writes, got %d", expectedWrites, stats.Writes)
	}
}

func BenchmarkRecordRead(b *testing.B) {
	tracker := NewTracker(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.RecordRead("benchmark_key")
	}
}

func BenchmarkRecordWrite(b *testing.B) {
	tracker := NewTracker(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.RecordWrite("benchmark_key")
	}
}

func BenchmarkGetHotKeys(b *testing.B) {
	tracker := NewTracker(100)
	// Setup: create many keys
	for i := 0; i < 1000; i++ {
		for j := 0; j < 10; j++ {
			tracker.RecordRead("key_" + string(rune(i)))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.GetHotKeys(10)
	}
}

func BenchmarkConcurrentReadWrite(b *testing.B) {
	tracker := NewTracker(100)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tracker.RecordRead("parallel_key")
			tracker.RecordWrite("parallel_key")
		}
	})
}
