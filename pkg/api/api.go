// pkg/api/api.go
package api

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/engine"
)

// Server handles TCP connections and command processing
type Server struct {
	engine         *engine.Engine
	listener       net.Listener
	cfg            *config.Config
	activeConns    int32
	shutdownChan   chan struct{}
	wg             sync.WaitGroup
}

// NewServer creates a new Server instance
func NewServer(cfg *config.Config, eng *engine.Engine) *Server {
	return &Server{
		engine:       eng,
		cfg:          cfg,
		shutdownChan: make(chan struct{}),
	}
}

// Start begins listening for connections
func (s *Server) Start() error {
	if err := s.cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ln, err := net.Listen("tcp", s.cfg.Address())
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	s.listener = ln
	log.Printf("kvlite server listening on %s", s.cfg.Address())

	for {
		select {
		case <-s.shutdownChan:
			return nil
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.shutdownChan:
				return nil
			default:
				log.Printf("error accepting connection: %v", err)
				continue
			}
		}

		// Check connection limit
		if s.cfg.MaxConnections > 0 {
			current := atomic.LoadInt32(&s.activeConns)
			if current >= int32(s.cfg.MaxConnections) {
				log.Printf("connection limit reached, rejecting %s", conn.RemoteAddr())
				conn.Write([]byte("-ERR connection limit reached\n"))
				conn.Close()
				continue
			}
		}

		s.wg.Add(1)
		atomic.AddInt32(&s.activeConns, 1)
		go s.handleConnection(conn)
	}
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	close(s.shutdownChan)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	log.Println("server shutdown complete")
	return nil
}

// Addr returns the actual address the server is listening on.
// This is useful when using port 0 for OS-assigned random port.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.cfg.Address()
}

// handleConnection processes commands from a single client
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		atomic.AddInt32(&s.activeConns, -1)
		s.wg.Done()
	}()

	clientAddr := conn.RemoteAddr().String()
	log.Printf("client connected: %s", clientAddr)

	scanner := bufio.NewScanner(conn)
	writer := bufio.NewWriter(conn)

	// Send welcome message
	writer.WriteString("+OK kvlite ready\n")
	writer.Flush()

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line, skip
		if strings.TrimSpace(line) == "" {
			continue
		}

		response := s.processCommand(line)
		writer.WriteString(response + "\n")
		writer.Flush()

		// Handle QUIT command
		if strings.HasPrefix(response, "+OK goodbye") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("error reading from %s: %v", clientAddr, err)
	}
	log.Printf("client disconnected: %s", clientAddr)
}

// processCommand parses and executes commands
func (s *Server) processCommand(line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "-ERR empty command"
	}

	cmd := strings.ToUpper(parts[0])

	switch cmd {
	case "SET":
		if len(parts) < 3 {
			return "-ERR SET requires key and value"
		}
		key := parts[1]
		value := strings.Join(parts[2:], " ")
		if err := s.engine.Set(key, value); err != nil {
			return fmt.Sprintf("-ERR failed to set: %v", err)
		}
		return "+OK"

	case "SETEX":
		if len(parts) < 4 {
			return "-ERR SETEX requires key, seconds, and value"
		}
		key := parts[1]
		seconds, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || seconds <= 0 {
			return "-ERR invalid TTL"
		}
		value := strings.Join(parts[3:], " ")

		ttl := time.Duration(seconds) * time.Second
		if err := s.engine.SetWithTTL(key, value, ttl); err != nil {
			return fmt.Sprintf("-ERR failed to set: %v", err)
		}
		return "+OK"

	case "GET":
		if len(parts) < 2 {
			return "-ERR GET requires key"
		}
		key := parts[1]
		val, ok := s.engine.Get(key)
		if !ok {
			return "-ERR key not found"
		}
		return val

	case "DELETE", "DEL":
		if len(parts) < 2 {
			return "-ERR DELETE requires key"
		}
		key := parts[1]
		deleted, err := s.engine.Delete(key)
		if err != nil {
			return fmt.Sprintf("-ERR failed to delete: %v", err)
		}
		if deleted {
			return "+OK"
		}
		return "-ERR key not found"

	case "EXISTS":
		if len(parts) < 2 {
			return "-ERR EXISTS requires key"
		}
		key := parts[1]
		_, ok := s.engine.Get(key)
		if ok {
			return "1"
		}
		return "0"

	case "EXPIRE":
		if len(parts) < 3 {
			return "-ERR EXPIRE requires key and seconds"
		}
		key := parts[1]
		seconds, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || seconds <= 0 {
			return "-ERR invalid TTL"
		}

		ttl := time.Duration(seconds) * time.Second
		if s.engine.Expire(key, ttl) {
			return "1"
		}
		return "0"

	case "TTL":
		if len(parts) < 2 {
			return "-ERR TTL requires key"
		}
		key := parts[1]

		// Check if key exists first
		_, exists := s.engine.Get(key)
		if !exists {
			return "-2" // Key doesn't exist
		}

		ttl := s.engine.TTL(key)
		if ttl == 0 {
			return "-1" // No TTL
		}

		seconds := int64(ttl.Seconds())
		return fmt.Sprintf("%d", seconds)

	case "PERSIST":
		if len(parts) < 2 {
			return "-ERR PERSIST requires key"
		}
		key := parts[1]
		if s.engine.Persist(key) {
			return "1"
		}
		return "0"

	case "KEYS":
		pattern := "*"
		if len(parts) >= 2 {
			pattern = parts[1]
		}

		keys := s.engine.Keys(pattern)
		if len(keys) == 0 {
			return "(empty list)"
		}

		return strings.Join(keys, "\n")

	case "SCAN":
		cursor := 0
		pattern := "*"
		count := 10

		if len(parts) < 2 {
			return "-ERR SCAN requires cursor"
		}

		var err error
		cursor, err = strconv.Atoi(parts[1])
		if err != nil {
			return "-ERR invalid cursor"
		}

		// Parse optional arguments
		for i := 2; i < len(parts); i++ {
			arg := strings.ToUpper(parts[i])
			switch arg {
			case "MATCH":
				if i+1 < len(parts) {
					pattern = parts[i+1]
					i++
				}
			case "COUNT":
				if i+1 < len(parts) {
					count, err = strconv.Atoi(parts[i+1])
					if err != nil || count <= 0 {
						return "-ERR invalid count"
					}
					i++
				}
			}
		}

		nextCursor, keys, _ := s.engine.Scan(cursor, pattern, count)

		result := fmt.Sprintf("%d", nextCursor)
		if len(keys) > 0 {
			result += "\n" + strings.Join(keys, "\n")
		}
		return result

	case "CLEAR":
		if err := s.engine.Clear(); err != nil {
			return fmt.Sprintf("-ERR failed to clear: %v", err)
		}
		return "+OK"

	case "PING":
		return "+PONG"

	case "QUIT":
		return "+OK goodbye"

	case "INFO":
		walSize, _ := s.engine.WALSize()
		return fmt.Sprintf("+OK keys=%d connections=%d wal_size=%d",
			s.engine.Len(),
			atomic.LoadInt32(&s.activeConns),
			walSize)

	case "SYNC":
		if err := s.engine.Sync(); err != nil {
			return fmt.Sprintf("-ERR failed to sync: %v", err)
		}
		return "+OK"

	case "COMPACT":
		if err := s.engine.ForceCompact(); err != nil {
			return fmt.Sprintf("-ERR failed to compact: %v", err)
		}
		return "+OK"

	case "STATS":
		stats := s.engine.CompactionStats()
		walSize := stats["wal_size"].(int64)
		walEntries := stats["wal_entries"].(int64)
		needsCompaction := stats["needs_compaction"].(bool)
		ttlExpired := stats["ttl_total_expired"].(int64)
		ttlChecks := stats["ttl_checks"].(int64)

		result := fmt.Sprintf("+OK keys=%d connections=%d wal_size=%d wal_entries=%d needs_compaction=%v ttl_expired=%d ttl_checks=%d",
			s.engine.Len(),
			atomic.LoadInt32(&s.activeConns),
			walSize,
			walEntries,
			needsCompaction,
			ttlExpired,
			ttlChecks)
		
		// Add analytics stats if enabled
		if analyticsEnabled, ok := stats["analytics_enabled"].(bool); ok && analyticsEnabled {
			totalReads := stats["total_reads"].(int64)
			totalWrites := stats["total_writes"].(int64)
			result += fmt.Sprintf(" reads=%d writes=%d", totalReads, totalWrites)
		}
		
		return result
	
	case "ANALYZE":
		if len(parts) < 2 {
			return "-ERR ANALYZE requires key"
		}
		key := parts[1]
		
		keyStats := s.engine.GetKeyStats(key)
		if keyStats == nil {
			return "-ERR analytics not enabled or key not found"
		}
		
		return fmt.Sprintf("+OK reads=%d writes=%d last_access=%s created=%s",
			keyStats.Reads,
			keyStats.Writes,
			keyStats.LastAccess.Format(time.RFC3339),
			keyStats.CreatedAt.Format(time.RFC3339))
	
	case "HOTKEYS":
		count := 10
		if len(parts) >= 2 {
			var err error
			count, err = strconv.Atoi(parts[1])
			if err != nil || count <= 0 {
				return "-ERR invalid count"
			}
		}
		
		hotKeys := s.engine.GetHotKeys(count)
		if hotKeys == nil {
			return "-ERR analytics not enabled"
		}
		
		if len(hotKeys) == 0 {
			return "(empty list)"
		}
		
		result := ""
		for _, stats := range hotKeys {
			result += fmt.Sprintf("%s (reads=%d writes=%d)\n", stats.Key, stats.Reads, stats.Writes)
		}
		return strings.TrimSuffix(result, "\n")
	
	case "SUGGEST-TTL":
		if len(parts) < 2 {
			return "-ERR SUGGEST-TTL requires key"
		}
		key := parts[1]
		
		suggestedTTL := s.engine.SuggestTTL(key)
		if suggestedTTL == 0 {
			return "-ERR analytics not enabled or insufficient data"
		}
		
		seconds := int64(suggestedTTL.Seconds())
		return fmt.Sprintf("%d", seconds)
	
	case "ANOMALIES":
		anomalies := s.engine.DetectAnomalies()
		if anomalies == nil {
			return "-ERR analytics not enabled"
		}
		
		if len(anomalies) == 0 {
			return "(no anomalies detected)"
		}
		
		return strings.Join(anomalies, "\n")
		
	case "MSET":
		if len(parts) < 3 || len(parts)%2 != 1 {
			return "-ERR MSET requires key value pairs"
		}
		
		// Batch set operation
		for i := 1; i < len(parts); i += 2 {
			key := parts[i]
			value := parts[i+1]
			if err := s.engine.Set(key, value); err != nil {
				return fmt.Sprintf("-ERR failed at key %s: %v", key, err)
			}
		}
		return "+OK"

	case "MGET":
		if len(parts) < 2 {
			return "-ERR MGET requires at least one key"
		}
		
		// Batch get operation
		var results []string
		for _, key := range parts[1:] {
			val, ok := s.engine.Get(key)
			if ok {
				results = append(results, val)
			} else {
				results = append(results, "(nil)")
			}
		}
		return strings.Join(results, "\n")

	case "MDEL":
		if len(parts) < 2 {
			return "-ERR MDEL requires at least one key"
		}
		
		// Batch delete operation
		deleted := 0
		for _, key := range parts[1:] {
			if ok, err := s.engine.Delete(key); err == nil && ok {
				deleted++
			}
		}
		return fmt.Sprintf("%d", deleted)

	case "HEALTH":
		// Health check endpoint
		walSize, walErr := s.engine.WALSize()
		status := "healthy"
		
		if walErr != nil {
			status = "degraded"
		}
		
		health := fmt.Sprintf(`{
	"status": "%s",
	"keys": %d,
	"connections": %d,
	"wal_size": %d,
	"wal_healthy": %v
	}`, status, s.engine.Len(), atomic.LoadInt32(&s.activeConns), walSize, walErr == nil)
		
		return health

	case "CONFIG":
		if len(parts) < 2 {
			return "-ERR CONFIG requires subcommand"
		}
		
		subCmd := strings.ToUpper(parts[1])
		switch subCmd {
		case "GET":
			if len(parts) < 3 {
				return "-ERR CONFIG GET requires parameter name"
			}
			param := strings.ToLower(parts[2])
			switch param {
			case "max_connections":
				return fmt.Sprintf("%d", s.cfg.MaxConnections)
			case "host":
				return s.cfg.Host
			case "port":
				return fmt.Sprintf("%d", s.cfg.Port)
			default:
				return "-ERR unknown config parameter"
			}
		default:
			return "-ERR unknown CONFIG subcommand"
		}

	case "INCR":
		if len(parts) < 2 {
			return "-ERR INCR requires key"
		}
		key := parts[1]
		
		// Get current value
		val, exists := s.engine.Get(key)
		current := int64(0)
		
		if exists {
			var err error
			current, err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				return "-ERR value is not an integer"
			}
		}
		
		// Increment
		current++
		newVal := strconv.FormatInt(current, 10)
		
		if err := s.engine.Set(key, newVal); err != nil {
			return fmt.Sprintf("-ERR failed to set: %v", err)
		}
		
		return fmt.Sprintf("%d", current)

	case "DECR":
		if len(parts) < 2 {
			return "-ERR DECR requires key"
		}
		key := parts[1]
		
		// Get current value
		val, exists := s.engine.Get(key)
		current := int64(0)
		
		if exists {
			var err error
			current, err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				return "-ERR value is not an integer"
			}
		}
		
		// Decrement
		current--
		newVal := strconv.FormatInt(current, 10)
		
		if err := s.engine.Set(key, newVal); err != nil {
			return fmt.Sprintf("-ERR failed to set: %v", err)
		}
		
		return fmt.Sprintf("%d", current)

	case "APPEND":
		if len(parts) < 3 {
			return "-ERR APPEND requires key and value"
		}
		key := parts[1]
		appendVal := strings.Join(parts[2:], " ")
		
		// Get current value
		val, exists := s.engine.Get(key)
		if !exists {
			val = ""
		}
		
		// Append
		newVal := val + appendVal
		if err := s.engine.Set(key, newVal); err != nil {
			return fmt.Sprintf("-ERR failed to set: %v", err)
		}
		
		return fmt.Sprintf("%d", len(newVal))

	case "STRLEN":
		if len(parts) < 2 {
			return "-ERR STRLEN requires key"
		}
		key := parts[1]
		
		val, exists := s.engine.Get(key)
		if !exists {
			return "0"
		}
		
		return fmt.Sprintf("%d", len(val))

	default:
		return fmt.Sprintf("-ERR unknown command '%s'", cmd)
	}
}