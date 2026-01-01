// scripts/client_test.go
// Simple test client for kvlite
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	addr := "localhost:6380"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	fmt.Printf("Connecting to kvlite at %s...\n", addr)
	
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read welcome message
	reader := bufio.NewReader(conn)
	welcome, _ := reader.ReadString('\n')
	fmt.Print(welcome)

	// Run test commands
	commands := []string{
		"PING",
		"SET user:1 Alice",
		"SET user:2 Bob",
		"GET user:1",
		"GET user:2",
		"EXISTS user:1",
		"EXISTS user:999",
		"DELETE user:2",
		"GET user:2",
		"INFO",
	}

	fmt.Println("\nRunning test commands:")
	fmt.Println("=====================")

	for _, cmd := range commands {
		fmt.Printf("> %s\n", cmd)
		
		// Send command
		fmt.Fprintf(conn, "%s\n", cmd)
		
		// Read response
		response, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading response: %v", err)
			continue
		}
		
		fmt.Printf("< %s", response)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n✓ All tests completed successfully!")
	
	// Interactive mode
	fmt.Println("\nEntering interactive mode. Type 'exit' to quit.")
	
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		if line == "exit" {
			break
		}
		
		// Send command
		fmt.Fprintf(conn, "%s\n", line)
		
		// Read response
		response, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		
		fmt.Printf("< %s", response)
		
		if strings.HasPrefix(response, "+OK goodbye") {
			break
		}
	}
}