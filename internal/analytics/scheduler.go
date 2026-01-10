// internal/analytics/scheduler.go
package analytics

import (
	"sync"
	"time"
)

// CompactionEvent records a compaction occurrence
type CompactionEvent struct {
	Timestamp       time.Time
	Hour            int           // Hour of day (0-23)
	DayOfWeek       int           // 0=Sunday, 6=Saturday
	RequestRate     float64       // Requests per second at time of compaction
	Duration        time.Duration // How long compaction took
	KeyCount        int           // Number of keys at time
	WALSize         int64         // WAL size before compaction
	UserImpact      float64       // Latency increase during compaction (ms)
	WasAutomatic    bool          // True if triggered automatically
}

// SmartScheduler learns optimal compaction times
type SmartScheduler struct {
	mu              sync.RWMutex
	history         []CompactionEvent
	maxHistory      int
	requestRates    []float64 // Rolling window of request rates
	rateWindowSize  int
	
	// Learned patterns
	peakHours       map[int]bool // Hours to avoid (high traffic)
	optimalHours    map[int]bool // Hours preferred (low traffic)
	
	// Thresholds (learned over time)
	lowTrafficRate  float64 // Requests/sec considered "low traffic"
	highTrafficRate float64 // Requests/sec considered "high traffic"
}

// NewSmartScheduler creates a new smart compaction scheduler
func NewSmartScheduler() *SmartScheduler {
	return &SmartScheduler{
		history:         make([]CompactionEvent, 0),
		maxHistory:      100, // Keep last 100 compactions
		requestRates:    make([]float64, 0),
		rateWindowSize:  60, // Track last 60 measurements
		peakHours:       make(map[int]bool),
		optimalHours:    make(map[int]bool),
		lowTrafficRate:  10.0,  // Default: < 10 req/sec is low
		highTrafficRate: 100.0, // Default: > 100 req/sec is high
	}
}

// RecordCompaction records a compaction event
func (s *SmartScheduler) RecordCompaction(event CompactionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = append(s.history, event)
	
	// Keep history bounded
	if len(s.history) > s.maxHistory {
		s.history = s.history[1:]
	}

	// Learn from this event
	s.learn()
}

// RecordRequestRate records current request rate
func (s *SmartScheduler) RecordRequestRate(rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestRates = append(s.requestRates, rate)
	
	// Keep window bounded
	if len(s.requestRates) > s.rateWindowSize {
		s.requestRates = s.requestRates[1:]
	}
}

// learn analyzes history to identify patterns
func (s *SmartScheduler) learn() {
	if len(s.history) < 10 {
		return // Need more data
	}

	// Analyze which hours had low impact
	hourImpact := make(map[int][]float64)
	hourRate := make(map[int][]float64)

	for _, event := range s.history {
		hourImpact[event.Hour] = append(hourImpact[event.Hour], event.UserImpact)
		hourRate[event.Hour] = append(hourRate[event.Hour], event.RequestRate)
	}

	// Clear and rebuild optimal/peak hours
	s.peakHours = make(map[int]bool)
	s.optimalHours = make(map[int]bool)

	for hour := 0; hour < 24; hour++ {
		if rates, ok := hourRate[hour]; ok && len(rates) > 0 {
			avgRate := average(rates)
			
			if avgRate > s.highTrafficRate {
				s.peakHours[hour] = true
			} else if avgRate < s.lowTrafficRate {
				s.optimalHours[hour] = true
			}
		}
	}

	// Update thresholds based on overall patterns
	if len(s.history) > 20 {
		allRates := make([]float64, 0)
		for _, event := range s.history {
			allRates = append(allRates, event.RequestRate)
		}
		
		// Use percentiles to adjust thresholds
		s.lowTrafficRate = percentile(allRates, 0.25)   // 25th percentile
		s.highTrafficRate = percentile(allRates, 0.75)  // 75th percentile
	}
}

// ShouldCompactNow returns a score (0-1) indicating if now is a good time
// Higher score = better time to compact
func (s *SmartScheduler) ShouldCompactNow() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	hour := now.Hour()
	dayOfWeek := int(now.Weekday())

	score := 0.5 // Neutral starting score

	// Factor 1: Current hour
	if s.optimalHours[hour] {
		score += 0.3 // Strong preference
	} else if s.peakHours[hour] {
		score -= 0.3 // Strong avoidance
	}

	// Factor 2: Day of week (weekends better)
	if dayOfWeek == 0 || dayOfWeek == 6 { // Sunday or Saturday
		score += 0.1
	}

	// Factor 3: Recent request rate
	if len(s.requestRates) > 0 {
		recentRate := s.requestRates[len(s.requestRates)-1]
		
		if recentRate < s.lowTrafficRate {
			score += 0.2
		} else if recentRate > s.highTrafficRate {
			score -= 0.2
		}
	}

	// Factor 4: Time since last compaction
	if len(s.history) > 0 {
		lastCompaction := s.history[len(s.history)-1]
		timeSince := time.Since(lastCompaction.Timestamp)
		
		// If it's been a while, increase score
		if timeSince > 24*time.Hour {
			score += 0.1
		}
	}

	// Clamp score between 0 and 1
	if score > 1.0 {
		score = 1.0
	} else if score < 0.0 {
		score = 0.0
	}

	return score
}

// GetOptimalCompactionTime returns the next optimal time for compaction
func (s *SmartScheduler) GetOptimalCompactionTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	
	// Find next optimal hour
	for i := 1; i <= 24; i++ {
		nextTime := now.Add(time.Duration(i) * time.Hour)
		nextHour := nextTime.Hour()
		
		if s.optimalHours[nextHour] {
			// Round to the start of that hour
			return time.Date(
				nextTime.Year(),
				nextTime.Month(),
				nextTime.Day(),
				nextHour,
				0, 0, 0,
				nextTime.Location(),
			)
		}
	}

	// If no optimal hour found, suggest late night (3 AM)
	tomorrow := now.Add(24 * time.Hour)
	return time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		3, 0, 0, 0,
		tomorrow.Location(),
	)
}

// GetStats returns scheduler statistics
func (s *SmartScheduler) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"total_compactions": len(s.history),
		"peak_hours":        s.peakHours,
		"optimal_hours":     s.optimalHours,
		"low_traffic_rate":  s.lowTrafficRate,
		"high_traffic_rate": s.highTrafficRate,
		"should_compact_now": s.ShouldCompactNow(),
	}
}

// Helper functions

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple percentile calculation (could use more sophisticated method)
	sorted := make([]float64, len(values))
	copy(sorted, values)
	
	// Simple sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}