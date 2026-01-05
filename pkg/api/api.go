// pkg/api/api.go
package api

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lofoneh/kvlite/internal/config"
	"github.com/lofoneh/kvlite/internal/engine"
)

// Server handles TCP connections and command processing
type Server struct {
	engine       *engine.Engine
	listener       net.Listener
	cfg            *config.Config
	activeConns    int32
	shutdownChan   chan struct{}
	wg             sync.WaitGroup
}

// NewServer creates a new Server instance
func NewServer(cfg *config.Config, eng *engine.Engine) *Server {
	return &Server{
		engine:        eng,
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
// Protocol:
//   SET key value -> +OK
//   GET key       -> value or -ERR key not found
//   DELETE key    -> +OK or -ERR key not found
//   EXISTS key    -> 1 or 0
//   KEYS          -> list of keys
//   CLEAR         -> +OK
//   PING          -> +PONG
//   QUIT          -> +OK goodbye
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
		
		return fmt.Sprintf("+OK keys=%d connections=%d wal_size=%d wal_entries=%d needs_compaction=%v", 
			s.engine.Len(), 
			atomic.LoadInt32(&s.activeConns),
			walSize,
			walEntries,
			needsCompaction)

	default:
		return fmt.Sprintf("-ERR unknown command '%s'", cmd)
	}
}