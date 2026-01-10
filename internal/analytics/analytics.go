// internal/analytics/analytics.go
package analytics

import (
	"sort"
	"sync"
	"time"
)

// KeyStats tracks statistics for a single key
type KeyStats struct {
	Key           string
	Reads         int64
	Writes        int64
	LastAccess    time.Time
	CreatedAt     time.Time
	AccessHistory []time.Time // Last N accesses
}

// Tracker tracks access patterns for all keys
type Tracker struct {
	mu               sync.RWMutex
	stats            map[string]*KeyStats
	maxHistorySize   int
	totalReads       int64
	totalWrites      int64
	anomalyThreshold float64 // Threshold for anomaly detection
}

// NewTracker creates a new analytics tracker
func NewTracker(maxHistorySize int) *Tracker {
	if maxHistorySize <= 0 {
		maxHistorySize = 100 // Default: keep last 100 accesses
	}
	
	return &Tracker{
		stats:            make(map[string]*KeyStats),
		maxHistorySize:   maxHistorySize,
		anomalyThreshold: 3.0, // 3x standard deviation
	}
}

// RecordRead records a read operation
func (t *Tracker) RecordRead(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats, exists := t.stats[key]
	if !exists {
		stats = &KeyStats{
			Key:           key,
			CreatedAt:     time.Now(),
			AccessHistory: make([]time.Time, 0, t.maxHistorySize),
		}
		t.stats[key] = stats
	}

	stats.Reads++
	stats.LastAccess = time.Now()
	stats.AccessHistory = append(stats.AccessHistory, time.Now())

	// Keep history bounded
	if len(stats.AccessHistory) > t.maxHistorySize {
		stats.AccessHistory = stats.AccessHistory[1:]
	}

	t.totalReads++
}

// RecordWrite records a write operation
func (t *Tracker) RecordWrite(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats, exists := t.stats[key]
	if !exists {
		stats = &KeyStats{
			Key:           key,
			CreatedAt:     time.Now(),
			AccessHistory: make([]time.Time, 0, t.maxHistorySize),
		}
		t.stats[key] = stats
	}

	stats.Writes++
	stats.LastAccess = time.Now()
	stats.AccessHistory = append(stats.AccessHistory, time.Now())

	if len(stats.AccessHistory) > t.maxHistorySize {
		stats.AccessHistory = stats.AccessHistory[1:]
	}

	t.totalWrites++
}

// GetStats returns stats for a specific key
func (t *Tracker) GetStats(key string) *KeyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats, exists := t.stats[key]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	statsCopy := *stats
	statsCopy.AccessHistory = make([]time.Time, len(stats.AccessHistory))
	copy(statsCopy.AccessHistory, stats.AccessHistory)

	return &statsCopy
}

// GetHotKeys returns the N most accessed keys
func (t *Tracker) GetHotKeys(n int) []*KeyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	type keyAccess struct {
		key         string
		totalAccess int64
	}

	accesses := make([]keyAccess, 0, len(t.stats))
	for key, stats := range t.stats {
		accesses = append(accesses, keyAccess{
			key:         key,
			totalAccess: stats.Reads + stats.Writes,
		})
	}

	// Sort by total access count
	sort.Slice(accesses, func(i, j int) bool {
		return accesses[i].totalAccess > accesses[j].totalAccess
	})

	// Return top N
	if n > len(accesses) {
		n = len(accesses)
	}

	result := make([]*KeyStats, n)
	for i := 0; i < n; i++ {
		stats := t.stats[accesses[i].key]
		statsCopy := *stats
		result[i] = &statsCopy
	}

	return result
}

// GetColdKeys returns keys that haven't been accessed recently
func (t *Tracker) GetColdKeys(threshold time.Duration) []*KeyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-threshold)
	result := make([]*KeyStats, 0)

	for _, stats := range t.stats {
		if stats.LastAccess.Before(cutoff) {
			statsCopy := *stats
			result = append(result, &statsCopy)
		}
	}

	return result
}

// GetReadWriteRatio returns the read/write ratio for a key
// Returns: ratio (reads/writes), reads, writes
func (t *Tracker) GetReadWriteRatio(key string) (float64, int64, int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats, exists := t.stats[key]
	if !exists {
		return 0, 0, 0
	}

	if stats.Writes == 0 {
		return float64(stats.Reads), stats.Reads, stats.Writes
	}

	ratio := float64(stats.Reads) / float64(stats.Writes)
	return ratio, stats.Reads, stats.Writes
}

// SuggestTTL suggests a TTL based on access patterns
func (t *Tracker) SuggestTTL(key string) time.Duration {
	stats := t.GetStats(key)
	if stats == nil {
		return 0
	}

	// If accessed frequently, suggest longer TTL
	totalAccess := stats.Reads + stats.Writes
	age := time.Since(stats.CreatedAt)

	if age == 0 {
		return 1 * time.Hour // Default
	}

	accessRate := float64(totalAccess) / age.Seconds()

	// High access rate = longer TTL
	if accessRate > 1.0 { // More than 1 access/second
		return 24 * time.Hour
	} else if accessRate > 0.1 { // More than 1 access/10 seconds
		return 1 * time.Hour
	} else {
		return 10 * time.Minute
	}
}

// DetectAnomalies detects unusual access patterns
func (t *Tracker) DetectAnomalies() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	anomalies := make([]string, 0)

	// Calculate average access rate
	var totalAccess int64
	for _, stats := range t.stats {
		totalAccess += stats.Reads + stats.Writes
	}

	if len(t.stats) == 0 {
		return anomalies
	}

	avgAccess := float64(totalAccess) / float64(len(t.stats))

	// Find keys with unusually high access
	for key, stats := range t.stats {
		keyAccess := float64(stats.Reads + stats.Writes)
		if keyAccess > avgAccess*t.anomalyThreshold {
			anomalies = append(anomalies, key)
		}
	}

	return anomalies
}

// GetGlobalStats returns overall statistics
func (t *Tracker) GetGlobalStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"total_keys":   len(t.stats),
		"total_reads":  t.totalReads,
		"total_writes": t.totalWrites,
		"read_write_ratio": func() float64 {
			if t.totalWrites == 0 {
				return float64(t.totalReads)
			}
			return float64(t.totalReads) / float64(t.totalWrites)
		}(),
	}
}

// Reset clears all statistics
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats = make(map[string]*KeyStats)
	t.totalReads = 0
	t.totalWrites = 0
}

// RemoveKey removes statistics for a key (called when key is deleted)
func (t *Tracker) RemoveKey(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.stats, key)
}