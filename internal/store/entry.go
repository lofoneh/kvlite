// internal/store/entry.go
package store

import (
	"time"
)

// Entry represents a key-value pair with optional TTL
type Entry struct {
	Value     string
	ExpiresAt int64 // Unix nanoseconds, 0 means no expiration
}

// NewEntry creates a new entry without TTL
func NewEntry(value string) *Entry {
	return &Entry{
		Value:     value,
		ExpiresAt: 0,
	}
}

// NewEntryWithTTL creates a new entry with TTL
func NewEntryWithTTL(value string, ttl time.Duration) *Entry {
	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).UnixNano()
	}
	return &Entry{
		Value:     value,
		ExpiresAt: expiresAt,
	}
}

// IsExpired checks if the entry has expired
func (e *Entry) IsExpired() bool {
	if e.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixNano() > e.ExpiresAt
}

// TTL returns the remaining time to live
// Returns 0 if no TTL or already expired
func (e *Entry) TTL() time.Duration {
	if e.ExpiresAt == 0 {
		return 0
	}

	remaining := e.ExpiresAt - time.Now().UnixNano()
	if remaining <= 0 {
		return 0
	}

	return time.Duration(remaining)
}

// SetExpiration sets the expiration time
func (e *Entry) SetExpiration(ttl time.Duration) {
	if ttl > 0 {
		e.ExpiresAt = time.Now().Add(ttl).UnixNano()
	} else {
		e.ExpiresAt = 0
	}
}

// RemoveExpiration removes the TTL (makes entry persistent)
func (e *Entry) RemoveExpiration() {
	e.ExpiresAt = 0
}
