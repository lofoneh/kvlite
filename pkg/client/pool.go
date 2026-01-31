// pkg/client/pool.go
package client

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	ErrPoolClosed = errors.New("connection pool is closed")
	ErrTimeout    = errors.New("operation timeout")
)

// Connection wraps a net.Conn with read/write helpers
type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	pool   *Pool
}

// Pool manages a pool of connections to kvlite
type Pool struct {
	mu          sync.Mutex
	addr        string
	maxIdle     int
	maxActive   int
	idleTimeout time.Duration
	conns       chan *Connection
	active      int
	closed      bool
}

// PoolOptions configures the connection pool
type PoolOptions struct {
	Addr        string        // Server address (host:port)
	MaxIdle     int           // Max idle connections
	MaxActive   int           // Max active connections (0 = unlimited)
	IdleTimeout time.Duration // Idle connection timeout
}

// NewPool creates a new connection pool
func NewPool(opts PoolOptions) (*Pool, error) {
	if opts.Addr == "" {
		return nil, errors.New("addr is required")
	}
	
	if opts.MaxIdle <= 0 {
		opts.MaxIdle = 5
	}
	
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 5 * time.Minute
	}
	
	return &Pool{
		addr:        opts.Addr,
		maxIdle:     opts.MaxIdle,
		maxActive:   opts.MaxActive,
		idleTimeout: opts.IdleTimeout,
		conns:       make(chan *Connection, opts.MaxIdle),
		active:      0,
		closed:      false,
	}, nil
}

// Get retrieves a connection from the pool
func (p *Pool) Get() (*Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil, ErrPoolClosed
	}
	
	// Try to get idle connection
	select {
	case conn := <-p.conns:
		// Check if connection is still alive
		if conn.isAlive() {
			return conn, nil
		}
		// Connection is dead, close it
		conn.close()
		p.active--
	default:
		// No idle connections
	}
	
	// Check if we can create new connection
	if p.maxActive > 0 && p.active >= p.maxActive {
		return nil, errors.New("connection pool exhausted")
	}
	
	// Create new connection
	conn, err := p.dial()
	if err != nil {
		return nil, err
	}
	
	p.active++
	return conn, nil
}

// Put returns a connection to the pool
func (p *Pool) Put(conn *Connection) {
	if conn == nil {
		return
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		conn.close()
		return
	}
	
	// Try to return to pool
	select {
	case p.conns <- conn:
		// Successfully returned to pool
	default:
		// Pool is full, close connection
		conn.close()
		p.active--
	}
}

// dial creates a new connection
func (p *Pool) dial() (*Connection, error) {
	netConn, err := net.DialTimeout("tcp", p.addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	
	reader := bufio.NewReader(netConn)
	writer := bufio.NewWriter(netConn)
	
	// Read welcome message
	if _, err := reader.ReadString('\n'); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("failed to read welcome: %w", err)
	}
	
	return &Connection{
		conn:   netConn,
		reader: reader,
		writer: writer,
		pool:   p,
	}, nil
}

// Close closes all connections in the pool
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil
	}
	
	p.closed = true
	close(p.conns)
	
	// Close all idle connections
	for conn := range p.conns {
		conn.close()
	}
	
	return nil
}

// Stats returns pool statistics
func (p *Pool) Stats() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	return map[string]int{
		"active": p.active,
		"idle":   len(p.conns),
	}
}

// Connection methods

// Do executes a command and returns the response
func (c *Connection) Do(cmd string, args ...string) (string, error) {
	// Build command
	fullCmd := cmd
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	
	// Send command
	if _, err := c.writer.WriteString(fullCmd + "\n"); err != nil {
		c.close()
		return "", err
	}
	
	if err := c.writer.Flush(); err != nil {
		c.close()
		return "", err
	}
	
	// Read response
	response, err := c.reader.ReadString('\n')
	if err != nil {
		c.close()
		return "", err
	}
	
	return strings.TrimSpace(response), nil
}

// Close returns the connection to the pool
func (c *Connection) Close() {
	c.pool.Put(c)
}

// close actually closes the underlying connection
func (c *Connection) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// isAlive checks if connection is still valid
func (c *Connection) isAlive() bool {
	if c.conn == nil {
		return false
	}
	
	// Set a short deadline to check
	c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	defer c.conn.SetReadDeadline(time.Time{})
	
	// Try to read (should timeout immediately on healthy connection)
	buf := make([]byte, 1)
	_, err := c.conn.Read(buf)
	
	// If we get a timeout, connection is alive
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	
	// Any other error or successful read means connection is bad
	return false
}

// Client provides a high-level API using the connection pool
type Client struct {
	pool *Pool
}

// NewClient creates a new kvlite client
func NewClient(addr string) (*Client, error) {
	pool, err := NewPool(PoolOptions{
		Addr:      addr,
		MaxIdle:   10,
		MaxActive: 50,
	})
	if err != nil {
		return nil, err
	}
	
	return &Client{pool: pool}, nil
}

// Set stores a key-value pair
func (c *Client) Set(key, value string) error {
	conn, err := c.pool.Get()
	if err != nil {
		return err
	}
	defer conn.Close()
	
	response, err := conn.Do("SET", key, value)
	if err != nil {
		return err
	}
	
	if response != "+OK" {
		return fmt.Errorf("SET failed: %s", response)
	}
	
	return nil
}

// Get retrieves a value by key
func (c *Client) Get(key string) (string, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	
	response, err := conn.Do("GET", key)
	if err != nil {
		return "", err
	}
	
	if strings.HasPrefix(response, "-ERR") {
		return "", errors.New(response)
	}
	
	return response, nil
}

// Delete removes a key
func (c *Client) Delete(key string) error {
	conn, err := c.pool.Get()
	if err != nil {
		return err
	}
	defer conn.Close()
	
	response, err := conn.Do("DELETE", key)
	if err != nil {
		return err
	}
	
	if strings.HasPrefix(response, "-ERR") {
		return errors.New(response)
	}
	
	return nil
}

// MSet sets multiple key-value pairs
func (c *Client) MSet(pairs map[string]string) error {
	conn, err := c.pool.Get()
	if err != nil {
		return err
	}
	defer conn.Close()
	
	args := make([]string, 0, len(pairs)*2)
	for k, v := range pairs {
		args = append(args, k, v)
	}
	
	response, err := conn.Do("MSET", args...)
	if err != nil {
		return err
	}
	
	if response != "+OK" {
		return fmt.Errorf("MSET failed: %s", response)
	}
	
	return nil
}

// MGet retrieves multiple values
func (c *Client) MGet(keys []string) ([]string, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	
	response, err := conn.Do("MGET", keys...)
	if err != nil {
		return nil, err
	}
	
	return strings.Split(response, "\n"), nil
}

// Close closes the client and its connection pool
func (c *Client) Close() error {
	return c.pool.Close()
}

// Stats returns client statistics
func (c *Client) Stats() map[string]int {
	return c.pool.Stats()
}