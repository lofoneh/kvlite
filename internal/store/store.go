// internal/store/store.go
package store

import (
	"sync"
)

// Store implements a thread-safe in-memory key-value store
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// New creates a new Store instance
func New() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Set stores a key-value pair
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get retrieves a value by key
// Returns the value and true if found, empty string and false otherwise
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
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

// Len returns the number of keys in the store
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Clear removes all keys from the store
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]string)
}

// Range iterates over all key-value pairs
// The function f should return true to continue iteration, false to stop
func (s *Store) Range(f func(key, value string) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for key, value := range s.data {
		if !f(key, value) {
			break
		}
	}
}