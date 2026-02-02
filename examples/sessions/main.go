// examples/sessions.go
// Demonstrates session management patterns with kvlite

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionManager handles session operations
type SessionManager struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	ttl    int // Session TTL in seconds
}

// NewSessionManager creates a new session manager
func NewSessionManager(addr string, ttlSeconds int) (*SessionManager, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read welcome message
	reader.ReadString('\n')

	return &SessionManager{
		conn:   conn,
		reader: reader,
		writer: writer,
		ttl:    ttlSeconds,
	}, nil
}

// Close closes the connection
func (sm *SessionManager) Close() {
	sm.conn.Close()
}

// sendCommand sends a command and returns the response
func (sm *SessionManager) sendCommand(cmd string) (string, error) {
	fmt.Fprintf(sm.writer, "%s\n", cmd)
	sm.writer.Flush()

	response, err := sm.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// generateSessionID creates a secure random session ID
func generateSessionID() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// CreateSession creates a new session for a user
func (sm *SessionManager) CreateSession(userID int, username, role, ipAddress string) (*Session, error) {
	session := &Session{
		ID:        generateSessionID(),
		UserID:    userID,
		Username:  username,
		Role:      role,
		IPAddress: ipAddress,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(sm.ttl) * time.Second),
	}

	// Serialize session data
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	// Store with TTL using SETEX
	key := fmt.Sprintf("session:%s", session.ID)
	cmd := fmt.Sprintf("SETEX %s %d %s", key, sm.ttl, string(data))

	response, err := sm.sendCommand(cmd)
	if err != nil {
		return nil, err
	}

	if response != "+OK" {
		return nil, fmt.Errorf("failed to create session: %s", response)
	}

	// Also maintain user -> session mapping for single-session enforcement
	userSessionKey := fmt.Sprintf("user:session:%d", userID)
	sm.sendCommand(fmt.Sprintf("SETEX %s %d %s", userSessionKey, sm.ttl, session.ID))

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	cmd := fmt.Sprintf("GET %s", key)

	response, err := sm.sendCommand(cmd)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(response, "-ERR") {
		return nil, fmt.Errorf("session not found")
	}

	var session Session
	if err := json.Unmarshal([]byte(response), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// RefreshSession extends the session TTL
func (sm *SessionManager) RefreshSession(sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	cmd := fmt.Sprintf("EXPIRE %s %d", key, sm.ttl)

	response, err := sm.sendCommand(cmd)
	if err != nil {
		return err
	}

	if response != "1" {
		return fmt.Errorf("session not found")
	}

	return nil
}

// InvalidateSession destroys a session
func (sm *SessionManager) InvalidateSession(sessionID string) error {
	// First get the session to find the user
	session, err := sm.GetSession(sessionID)
	if err == nil {
		// Remove user -> session mapping
		userSessionKey := fmt.Sprintf("user:session:%d", session.UserID)
		sm.sendCommand(fmt.Sprintf("DELETE %s", userSessionKey))
	}

	// Delete the session
	key := fmt.Sprintf("session:%s", sessionID)
	cmd := fmt.Sprintf("DELETE %s", key)

	_, err = sm.sendCommand(cmd)
	return err
}

// GetUserSession gets the current session for a user
func (sm *SessionManager) GetUserSession(userID int) (*Session, error) {
	userSessionKey := fmt.Sprintf("user:session:%d", userID)
	cmd := fmt.Sprintf("GET %s", userSessionKey)

	response, err := sm.sendCommand(cmd)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(response, "-ERR") {
		return nil, fmt.Errorf("no active session for user")
	}

	// Response is the session ID
	return sm.GetSession(response)
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (sm *SessionManager) InvalidateAllUserSessions(userID int) error {
	session, err := sm.GetUserSession(userID)
	if err != nil {
		return nil // No sessions to invalidate
	}

	return sm.InvalidateSession(session.ID)
}

func main() {
	fmt.Println("kvlite Session Management Demo")
	fmt.Println("==============================")

	// Connect to kvlite server
	sm, err := NewSessionManager("localhost:6380", 3600) // 1 hour TTL
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer sm.Close()

	// Example 1: Create a session
	fmt.Println("\n1. Creating User Session")
	session, err := sm.CreateSession(
		1001,
		"alice",
		"admin",
		"192.168.1.100",
	)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		return
	}
	fmt.Printf("   Session created: %s\n", session.ID[:16]+"...")
	fmt.Printf("   User: %s (ID: %d)\n", session.Username, session.UserID)
	fmt.Printf("   Role: %s\n", session.Role)
	fmt.Printf("   Expires: %s\n", session.ExpiresAt.Format(time.RFC3339))

	// Example 2: Retrieve session
	fmt.Println("\n2. Retrieving Session")
	retrieved, err := sm.GetSession(session.ID)
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	} else {
		fmt.Printf("   Retrieved session for: %s\n", retrieved.Username)
		fmt.Printf("   From IP: %s\n", retrieved.IPAddress)
	}

	// Example 3: Refresh session
	fmt.Println("\n3. Refreshing Session (extending TTL)")
	if err := sm.RefreshSession(session.ID); err != nil {
		log.Printf("Failed to refresh session: %v", err)
	} else {
		fmt.Println("   Session TTL extended")
	}

	// Example 4: Get user's current session
	fmt.Println("\n4. Getting User's Current Session")
	userSession, err := sm.GetUserSession(1001)
	if err != nil {
		log.Printf("Failed to get user session: %v", err)
	} else {
		fmt.Printf("   Active session for user 1001: %s\n", userSession.ID[:16]+"...")
	}

	// Example 5: Create second session (demonstrates single-session enforcement)
	fmt.Println("\n5. Creating Second Session (replaces first)")
	session2, _ := sm.CreateSession(1001, "alice", "admin", "192.168.1.101")
	fmt.Printf("   New session: %s\n", session2.ID[:16]+"...")
	fmt.Printf("   From new IP: %s\n", session2.IPAddress)

	// Example 6: Invalidate session
	fmt.Println("\n6. Invalidating Session (logout)")
	if err := sm.InvalidateSession(session2.ID); err != nil {
		log.Printf("Failed to invalidate: %v", err)
	} else {
		fmt.Println("   Session invalidated")
	}

	// Verify session is gone
	_, err = sm.GetSession(session2.ID)
	if err != nil {
		fmt.Println("   Confirmed: session no longer exists")
	}

	// Example 7: Session data structure
	fmt.Println("\n7. Session Data Structure")
	fmt.Println("   Key pattern: session:{session_id}")
	fmt.Println("   User mapping: user:session:{user_id} -> session_id")
	fmt.Println("   TTL: Auto-expires after configured duration")
	fmt.Println("   Data: JSON-serialized session object")

	fmt.Println("\nSession management demo complete!")
}
