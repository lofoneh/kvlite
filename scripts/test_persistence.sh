#!/bin/bash
# scripts/test_persistence.sh - Test crash recovery

set -e

echo "╔══════════════════════════════════════════╗"
echo "║  Testing kvlite Persistence (Week 2)     ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Clean up any existing test data
cleanup() {
    echo "Cleaning up..."
    pkill kvlite 2>/dev/null || true
    rm -rf ./test-data
    sleep 1
}

# Cleanup on exit
trap cleanup EXIT

# Start kvlite with test data directory
start_server() {
    echo -e "${BLUE}Starting kvlite...${NC}"
    ./bin/kvlite -wal-path ./test-data -port 6381 > /dev/null 2>&1 &
    KVLITE_PID=$!
    sleep 2
    
    if ! ps -p $KVLITE_PID > /dev/null 2>&1; then
        echo -e "${RED}✗${NC} Failed to start kvlite"
        exit 1
    fi
    
    echo -e "${GREEN}✓${NC} kvlite started (PID: $KVLITE_PID)"
}

# Stop kvlite
stop_server() {
    echo -e "${BLUE}Stopping kvlite...${NC}"
    kill $KVLITE_PID 2>/dev/null || true
    wait $KVLITE_PID 2>/dev/null || true
    sleep 1
    echo -e "${GREEN}✓${NC} kvlite stopped"
}

# Send command via PowerShell (Windows compatible)
send_command() {
    local cmd="$1"
    if command -v powershell &> /dev/null; then
        powershell -Command "
            \$tcp = New-Object System.Net.Sockets.TcpClient('localhost', 6381);
            \$stream = \$tcp.GetStream();
            \$writer = New-Object System.IO.StreamWriter(\$stream);
            \$reader = New-Object System.IO.StreamReader(\$stream);
            [void]\$reader.ReadLine();
            \$writer.WriteLine('$cmd'); \$writer.Flush();
            \$response = \$reader.ReadLine();
            \$tcp.Close();
            Write-Output \$response;
        " 2>/dev/null
    else
        # Fallback to /dev/tcp
        exec 3<>/dev/tcp/localhost/6381
        read -u 3 welcome
        echo "$cmd" >&3
        read -u 3 response
        exec 3<&-
        exec 3>&-
        echo "$response"
    fi
}

# Test 1: Basic persistence
test_basic_persistence() {
    echo ""
    echo -e "${BLUE}Test 1: Basic Persistence${NC}"
    echo "────────────────────────────────────────"
    
    # Start server
    start_server
    
    # Write data
    echo "Writing test data..."
    result=$(send_command "SET key1 value1")
    if [[ "$result" != "+OK" ]]; then
        echo -e "${RED}✗${NC} Failed to SET key1"
        return 1
    fi
    
    result=$(send_command "SET key2 value2")
    if [[ "$result" != "+OK" ]]; then
        echo -e "${RED}✗${NC} Failed to SET key2"
        return 1
    fi
    
    result=$(send_command "SET key3 value3")
    if [[ "$result" != "+OK" ]]; then
        echo -e "${RED}✗${NC} Failed to SET key3"
        return 1
    fi
    
    echo -e "${GREEN}✓${NC} Data written"
    
    # Stop server
    stop_server
    
    # Restart server
    echo "Restarting server..."
    start_server
    
    # Verify data persisted
    echo "Verifying data..."
    result=$(send_command "GET key1")
    if [[ "$result" != "value1" ]]; then
        echo -e "${RED}✗${NC} key1 not persisted (got: $result)"
        return 1
    fi
    
    result=$(send_command "GET key2")
    if [[ "$result" != "value2" ]]; then
        echo -e "${RED}✗${NC} key2 not persisted (got: $result)"
        return 1
    fi
    
    result=$(send_command "GET key3")
    if [[ "$result" != "value3" ]]; then
        echo -e "${RED}✗${NC} key3 not persisted (got: $result)"
        return 1
    fi
    
    echo -e "${GREEN}✓${NC} All data persisted correctly!"
    
    stop_server
}

# Test 2: DELETE persistence
test_delete_persistence() {
    echo ""
    echo -e "${BLUE}Test 2: DELETE Persistence${NC}"
    echo "────────────────────────────────────────"
    
    start_server
    
    # Set and delete
    send_command "SET temp value" > /dev/null
    send_command "DELETE temp" > /dev/null
    
    stop_server
    start_server
    
    # Verify deletion persisted
    result=$(send_command "GET temp")
    if [[ "$result" == *"key not found"* ]]; then
        echo -e "${GREEN}✓${NC} DELETE persisted correctly"
    else
        echo -e "${RED}✗${NC} DELETE not persisted (got: $result)"
        return 1
    fi
    
    stop_server
}

# Test 3: CLEAR persistence
test_clear_persistence() {
    echo ""
    echo -e "${BLUE}Test 3: CLEAR Persistence${NC}"
    echo "────────────────────────────────────────"
    
    start_server
    
    # Set multiple keys then clear
    send_command "SET a 1" > /dev/null
    send_command "SET b 2" > /dev/null
    send_command "SET c 3" > /dev/null
    send_command "CLEAR" > /dev/null
    send_command "SET d 4" > /dev/null
    
    stop_server
    start_server
    
    # Verify only 'd' exists
    result=$(send_command "GET a")
    if [[ "$result" != *"key not found"* ]]; then
        echo -e "${RED}✗${NC} Key 'a' should not exist"
        return 1
    fi
    
    result=$(send_command "GET d")
    if [[ "$result" != "4" ]]; then
        echo -e "${RED}✗${NC} Key 'd' not persisted"
        return 1
    fi
    
    echo -e "${GREEN}✓${NC} CLEAR persisted correctly"
    
    stop_server
}

# Test 4: Multiple restarts
test_multiple_restarts() {
    echo ""
    echo -e "${BLUE}Test 4: Multiple Restarts${NC}"
    echo "────────────────────────────────────────"
    
    # Cycle 1
    start_server
    send_command "SET cycle1 data1" > /dev/null
    stop_server
    
    # Cycle 2
    start_server
    send_command "SET cycle2 data2" > /dev/null
    stop_server
    
    # Cycle 3
    start_server
    send_command "SET cycle3 data3" > /dev/null
    stop_server
    
    # Verify all data
    start_server
    result1=$(send_command "GET cycle1")
    result2=$(send_command "GET cycle2")
    result3=$(send_command "GET cycle3")
    
    if [[ "$result1" == "data1" ]] && [[ "$result2" == "data2" ]] && [[ "$result3" == "data3" ]]; then
        echo -e "${GREEN}✓${NC} All data persisted across multiple restarts"
    else
        echo -e "${RED}✗${NC} Data lost across restarts"
        return 1
    fi
    
    stop_server
}

# Main execution
main() {
    cleanup
    
    test_basic_persistence
    test_delete_persistence
    test_clear_persistence
    test_multiple_restarts
    
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  All Persistence Tests Passed! ✓         ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════╝${NC}"
    echo ""
    echo "WAL file created at: ./test-data/kvlite.wal"
    echo "Check it with: cat ./test-data/kvlite.wal"
}

main "$@"