// examples/distributed_locks.go
// Demonstrates distributed locking patterns with kvlite

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// DistributedLock provides distributed locking functionality
type DistributedLock struct {
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	lockName string
	lockID   string
	ttl      int // Lock TTL in seconds
}

// LockManager manages distributed locks
type LockManager struct {
	addr string
}

// NewLockManager creates a new lock manager
func NewLockManager(addr string) *LockManager {
	return &LockManager{addr: addr}
}

// generateLockID creates a unique lock identifier
func generateLockID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewLock creates a new distributed lock
func (lm *LockManager) NewLock(name string, ttlSeconds int) (*DistributedLock, error) {
	conn, err := net.Dial("tcp", lm.addr)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome message
	reader.ReadString('\n')

	return &DistributedLock{
		conn:     conn,
		reader:   reader,
		writer:   writer,
		lockName: name,
		lockID:   generateLockID(),
		ttl:      ttlSeconds,
	}, nil
}

// sendCommand sends a command and returns the response
func (dl *DistributedLock) sendCommand(cmd string) (string, error) {
	fmt.Fprintf(dl.writer, "%s\n", cmd)
	dl.writer.Flush()

	response, err := dl.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// Acquire attempts to acquire the lock
// Returns true if lock was acquired, false if held by another process
func (dl *DistributedLock) Acquire() (bool, error) {
	key := fmt.Sprintf("lock:%s", dl.lockName)

	// Check if lock exists
	response, err := dl.sendCommand(fmt.Sprintf("GET %s", key))
	if err != nil {
		return false, err
	}

	if !strings.HasPrefix(response, "-ERR") {
		// Lock exists - check if it's ours (for re-entrant locking)
		if response == dl.lockID {
			// Refresh TTL
			dl.sendCommand(fmt.Sprintf("EXPIRE %s %d", key, dl.ttl))
			return true, nil
		}
		// Lock held by someone else
		return false, nil
	}

	// Try to acquire the lock using SETEX
	response, err = dl.sendCommand(fmt.Sprintf("SETEX %s %d %s", key, dl.ttl, dl.lockID))
	if err != nil {
		return false, err
	}

	if response == "+OK" {
		return true, nil
	}

	return false, nil
}

// AcquireWithRetry attempts to acquire the lock with retries
func (dl *DistributedLock) AcquireWithRetry(maxAttempts int, retryDelay time.Duration) (bool, error) {
	for i := 0; i < maxAttempts; i++ {
		acquired, err := dl.Acquire()
		if err != nil {
			return false, err
		}
		if acquired {
			return true, nil
		}
		if i < maxAttempts-1 {
			time.Sleep(retryDelay)
		}
	}
	return false, nil
}

// Release releases the lock (only if we own it)
func (dl *DistributedLock) Release() (bool, error) {
	key := fmt.Sprintf("lock:%s", dl.lockName)

	// Check if we own the lock
	response, err := dl.sendCommand(fmt.Sprintf("GET %s", key))
	if err != nil {
		return false, err
	}

	if response != dl.lockID {
		// We don't own this lock
		return false, nil
	}

	// Delete the lock
	response, err = dl.sendCommand(fmt.Sprintf("DELETE %s", key))
	if err != nil {
		return false, err
	}

	return response == "+OK", nil
}

// Extend extends the lock TTL
func (dl *DistributedLock) Extend() (bool, error) {
	key := fmt.Sprintf("lock:%s", dl.lockName)

	// Check if we own the lock
	response, err := dl.sendCommand(fmt.Sprintf("GET %s", key))
	if err != nil {
		return false, err
	}

	if response != dl.lockID {
		return false, nil
	}

	// Extend TTL
	response, err = dl.sendCommand(fmt.Sprintf("EXPIRE %s %d", key, dl.ttl))
	if err != nil {
		return false, err
	}

	return response == "1", nil
}

// IsHeld checks if the lock is currently held (by anyone)
func (dl *DistributedLock) IsHeld() (bool, string, error) {
	key := fmt.Sprintf("lock:%s", dl.lockName)

	response, err := dl.sendCommand(fmt.Sprintf("GET %s", key))
	if err != nil {
		return false, "", err
	}

	if strings.HasPrefix(response, "-ERR") {
		return false, "", nil
	}

	return true, response, nil
}

// Close closes the lock connection
func (dl *DistributedLock) Close() {
	dl.conn.Close()
}

// Semaphore implements a distributed semaphore
type Semaphore struct {
	manager  *LockManager
	name     string
	maxCount int
	ttl      int
}

// NewSemaphore creates a distributed semaphore
func (lm *LockManager) NewSemaphore(name string, maxCount int, ttlSeconds int) *Semaphore {
	return &Semaphore{
		manager:  lm,
		name:     name,
		maxCount: maxCount,
		ttl:      ttlSeconds,
	}
}

// Acquire acquires a semaphore slot
func (s *Semaphore) Acquire() (*DistributedLock, error) {
	// Try each slot until we get one
	for i := 0; i < s.maxCount; i++ {
		lock, err := s.manager.NewLock(fmt.Sprintf("%s:slot:%d", s.name, i), s.ttl)
		if err != nil {
			continue
		}

		acquired, _ := lock.Acquire()
		if acquired {
			return lock, nil
		}
		lock.Close()
	}

	return nil, fmt.Errorf("semaphore full")
}

func main() {
	fmt.Println("kvlite Distributed Locking Demo")
	fmt.Println("================================")

	lm := NewLockManager("localhost:6380")

	// Example 1: Basic Lock Usage
	fmt.Println("\n1. Basic Lock Acquisition")

	lock1, err := lm.NewLock("resource:database", 30)
	if err != nil {
		log.Fatal("Failed to create lock:", err)
	}
	defer lock1.Close()

	acquired, _ := lock1.Acquire()
	fmt.Printf("   Lock acquired: %v\n", acquired)

	// Check if held
	held, holder, _ := lock1.IsHeld()
	fmt.Printf("   Lock held: %v, holder: %s...\n", held, holder[:16])

	// Release
	released, _ := lock1.Release()
	fmt.Printf("   Lock released: %v\n", released)

	// Example 2: Lock Contention
	fmt.Println("\n2. Lock Contention Demo")

	lock2a, _ := lm.NewLock("critical:section", 10)
	lock2b, _ := lm.NewLock("critical:section", 10)
	defer lock2a.Close()
	defer lock2b.Close()

	// First process acquires
	acquired, _ = lock2a.Acquire()
	fmt.Printf("   Process A acquired: %v\n", acquired)

	// Second process tries to acquire (should fail)
	acquired, _ = lock2b.Acquire()
	fmt.Printf("   Process B acquired: %v (expected: false)\n", acquired)

	// Release and retry
	lock2a.Release()
	acquired, _ = lock2b.Acquire()
	fmt.Printf("   Process B acquired after release: %v\n", acquired)
	lock2b.Release()

	// Example 3: Lock with Retry
	fmt.Println("\n3. Lock with Retry")

	lock3, _ := lm.NewLock("retry:resource", 5)
	defer lock3.Close()

	// Acquire with retries
	acquired, _ = lock3.AcquireWithRetry(3, 100*time.Millisecond)
	fmt.Printf("   Acquired with retry: %v\n", acquired)
	lock3.Release()

	// Example 4: Lock Extension
	fmt.Println("\n4. Lock TTL Extension")

	lock4, _ := lm.NewLock("long:operation", 5)
	defer lock4.Close()

	lock4.Acquire()
	fmt.Println("   Lock acquired (TTL: 5 seconds)")

	// Simulate long operation
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)
		extended, _ := lock4.Extend()
		fmt.Printf("   Extended lock at %ds: %v\n", (i+1)*2, extended)
	}
	lock4.Release()

	// Example 5: Concurrent Workers with Locks
	fmt.Println("\n5. Concurrent Workers with Locks")

	var wg sync.WaitGroup
	results := make(chan string, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			lock, _ := lm.NewLock("shared:resource", 3)
			defer lock.Close()

			// Try to acquire with retries
			acquired, _ := lock.AcquireWithRetry(5, 200*time.Millisecond)

			if acquired {
				results <- fmt.Sprintf("   Worker %d: acquired lock, processing...", workerID)
				time.Sleep(500 * time.Millisecond) // Simulate work
				lock.Release()
				results <- fmt.Sprintf("   Worker %d: released lock", workerID)
			} else {
				results <- fmt.Sprintf("   Worker %d: failed to acquire lock", workerID)
			}
		}(i)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		fmt.Println(result)
	}

	// Example 6: Distributed Semaphore
	fmt.Println("\n6. Distributed Semaphore (max 3 concurrent)")

	semaphore := lm.NewSemaphore("api:connections", 3, 10)

	var semWg sync.WaitGroup
	for i := 0; i < 5; i++ {
		semWg.Add(1)
		go func(workerID int) {
			defer semWg.Done()

			lock, err := semaphore.Acquire()
			if err != nil {
				fmt.Printf("   Worker %d: %v\n", workerID, err)
				return
			}
			defer func() {
				lock.Release()
				lock.Close()
			}()

			fmt.Printf("   Worker %d: acquired semaphore slot\n", workerID)
			time.Sleep(1 * time.Second)
			fmt.Printf("   Worker %d: releasing slot\n", workerID)
		}(i)
	}

	semWg.Wait()

	// Example 7: Lock Best Practices
	fmt.Println("\n7. Lock Best Practices")
	fmt.Println("   - Always use TTL to prevent deadlocks")
	fmt.Println("   - Check lock ownership before release")
	fmt.Println("   - Use unique lock IDs per process")
	fmt.Println("   - Extend TTL for long operations")
	fmt.Println("   - Implement retry with backoff")
	fmt.Println("   - Clean up: close connections after use")

	fmt.Println("\nDistributed locking demo complete!")
}
