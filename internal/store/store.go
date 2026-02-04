// internal/store/store.go
package store

import (
	"sync"
	"time"
)

// Store implements a thread-safe in-memory key-value store with TTL support
type Store struct {
	mu   sync.RWMutex
	data map[string]*Entry
}

// New creates a new Store instance
func New() *Store {
	return &Store{
		data: make(map[string]*Entry),
	}
}

// Set stores a key-value pair without TTL
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = NewEntry(value)
}

// SetWithTTL stores a key-value pair with TTL
func (s *Store) SetWithTTL(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = NewEntryWithTTL(value, ttl)
}

// Get retrieves a value by key (with lazy expiration)
// Returns the value and true if found and not expired, empty string and false otherwise
func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		return "", false
	}

	// Lazy expiration: delete if expired
	if entry.IsExpired() {
		delete(s.data, key)
		return "", false
	}

	return entry.Value, true
}

// GetEntry retrieves the full entry (including TTL info)
func (s *Store) GetEntry(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok {
		return nil, false
	}

	if entry.IsExpired() {
		return nil, false
	}

	return entry, true
}

// Delete removes a key-value pair
// Returns true if the key existed, false otherwise
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, existed := s.data[key]
	delete(s.data, key)
	return existed
}

// Expire sets a TTL on an existing key
// Returns true if key exists, false otherwise
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || entry.IsExpired() {
		return false
	}

	entry.SetExpiration(ttl)
	return true
}

// Persist removes TTL from a key
// Returns true if key exists, false otherwise
func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || entry.IsExpired() {
		return false
	}

	entry.RemoveExpiration()
	return true
}

// TTL returns the remaining time to live for a key
// Returns 0 if no TTL or key doesn't exist
func (s *Store) TTL(key string) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok || entry.IsExpired() {
		return 0
	}

	return entry.TTL()
}

// Len returns the number of non-expired keys in the store
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, entry := range s.data {
		if !entry.IsExpired() {
			count++
		}
	}
	return count
}

// Clear removes all keys from the store
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*Entry)
}

// Range iterates over all non-expired key-value pairs
// The function f should return true to continue iteration, false to stop
func (s *Store) Range(f func(key, value string) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for key, entry := range s.data {
		if entry.IsExpired() {
			continue
		}
		if !f(key, entry.Value) {
			break
		}
	}
}

// RangeWithTTL iterates over all non-expired entries with TTL info
func (s *Store) RangeWithTTL(f func(key string, entry *Entry) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for key, entry := range s.data {
		if entry.IsExpired() {
			continue
		}
		if !f(key, entry) {
			break
		}
	}
}

// DeleteExpired removes all expired keys
// Returns the number of keys deleted
func (s *Store) DeleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	for key, entry := range s.data {
		if entry.IsExpired() {
			delete(s.data, key)
			deleted++
		}
	}
	return deleted
}

// Keys returns all non-expired keys matching the pattern
// Pattern supports glob-style matching: * matches any sequence, ? matches single char
func (s *Store) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for key, entry := range s.data {
		if entry.IsExpired() {
			continue
		}
		if matchPattern(pattern, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// matchPattern performs glob-style pattern matching
// Supports: * (matches any sequence), ? (matches single char)
func matchPattern(pattern, str string) bool {
	// Fast path: match all
	if pattern == "*" {
		return true
	}

	// Fast path: exact match
	if pattern == str {
		return true
	}

	return globMatch(pattern, str, 0, 0)
}

// globMatch implements recursive glob matching
func globMatch(pattern, str string, pIdx, sIdx int) bool {
	pLen := len(pattern)
	sLen := len(str)

	// Both exhausted - match
	if pIdx == pLen && sIdx == sLen {
		return true
	}

	// Pattern exhausted but string not - no match
	if pIdx == pLen {
		return false
	}

	// Check for wildcards
	if pIdx < pLen && pattern[pIdx] == '*' {
		// Skip consecutive stars
		for pIdx < pLen && pattern[pIdx] == '*' {
			pIdx++
		}

		// Star at end matches everything
		if pIdx == pLen {
			return true
		}

		// Try matching star with 0 or more characters
		for sIdx <= sLen {
			if globMatch(pattern, str, pIdx, sIdx) {
				return true
			}
			sIdx++
		}
		return false
	}

	// String exhausted but pattern not - check if remaining is all stars
	if sIdx == sLen {
		for pIdx < pLen {
			if pattern[pIdx] != '*' {
				return false
			}
			pIdx++
		}
		return true
	}

	// Match single character or ?
	if pattern[pIdx] == '?' || pattern[pIdx] == str[sIdx] {
		return globMatch(pattern, str, pIdx+1, sIdx+1)
	}

	return false
}

// Scan returns keys matching pattern with pagination
// cursor: start position (0 to start)
// count: max keys to return (0 = default 10)
// Returns: next cursor, keys, hasMore
func (s *Store) Scan(cursor int, pattern string, count int) (int, []string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if count <= 0 {
		count = 10 // Default page size
	}

	// Collect all matching keys
	var allKeys []string
	for key, entry := range s.data {
		if entry.IsExpired() {
			continue
		}
		if matchPattern(pattern, key) {
			allKeys = append(allKeys, key)
		}
	}

	// Paginate
	start := cursor
	if start >= len(allKeys) {
		return 0, []string{}, false
	}

	end := start + count
	hasMore := end < len(allKeys)
	if end > len(allKeys) {
		end = len(allKeys)
	}

	keys := allKeys[start:end]
	nextCursor := 0
	if hasMore {
		nextCursor = end
	}

	return nextCursor, keys, hasMore
}
