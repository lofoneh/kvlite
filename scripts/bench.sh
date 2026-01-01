#!/bin/bash
# scripts/bench.sh - Benchmark script for kvlite

set -e

echo "╔═══════════════════════════════════════╗"
echo "║      kvlite Benchmark Suite           ║"
echo "╚═══════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if kvlite is running
check_server() {
    if ! nc -z localhost 6380 2>/dev/null; then
        echo "❌ kvlite server is not running on localhost:6380"
        echo "Please start the server first: ./bin/kvlite"
        exit 1
    fi
    echo -e "${GREEN}✓${NC} Server is running"
}

# Run Go benchmarks
run_go_benchmarks() {
    echo ""
    echo -e "${BLUE}Running Go Benchmarks...${NC}"
    echo "─────────────────────────────────────────"
    go test -bench=. -benchmem ./internal/store/ | grep -E "Benchmark|ns/op|allocs/op"
}

# Run concurrent client test
run_concurrent_test() {
    echo ""
    echo -e "${BLUE}Running Concurrent Client Test...${NC}"
    echo "─────────────────────────────────────────"
    
    CLIENTS=10
    REQUESTS=1000
    
    echo "Spawning $CLIENTS concurrent clients, each sending $REQUESTS requests..."
    
    for i in $(seq 1 $CLIENTS); do
        (
            for j in $(seq 1 $REQUESTS); do
                echo -e "SET key$i:$j value$j\nGET key$i:$j" | nc -N localhost 6380 > /dev/null 2>&1
            done
        ) &
    done
    
    wait
    echo -e "${GREEN}✓${NC} Completed $((CLIENTS * REQUESTS * 2)) operations"
}

# Run latency test
run_latency_test() {
    echo ""
    echo -e "${BLUE}Running Latency Test...${NC}"
    echo "─────────────────────────────────────────"
    
    ITERATIONS=100
    
    echo "Measuring average latency over $ITERATIONS iterations..."
    
    START=$(date +%s%N)
    for i in $(seq 1 $ITERATIONS); do
        echo -e "SET latency_test value$i\nGET latency_test" | nc -N localhost 6380 > /dev/null 2>&1
    done
    END=$(date +%s%N)
    
    ELAPSED=$(( (END - START) / 1000000 )) # Convert to milliseconds
    AVG_LATENCY=$(echo "scale=2; $ELAPSED / ($ITERATIONS * 2)" | bc)
    
    echo -e "${GREEN}✓${NC} Average latency: ${AVG_LATENCY}ms per operation"
}

# Run throughput test
run_throughput_test() {
    echo ""
    echo -e "${BLUE}Running Throughput Test...${NC}"
    echo "─────────────────────────────────────────"
    
    DURATION=5
    echo "Running for ${DURATION} seconds..."
    
    COUNT=0
    START=$(date +%s)
    while [ $(($(date +%s) - START)) -lt $DURATION ]; do
        echo -e "SET throughput_test value\nGET throughput_test" | nc -N localhost 6380 > /dev/null 2>&1
        COUNT=$((COUNT + 2))
    done
    
    OPS_PER_SEC=$((COUNT / DURATION))
    echo -e "${GREEN}✓${NC} Throughput: ~${OPS_PER_SEC} ops/sec"
}

# Main execution
main() {
    check_server
    run_go_benchmarks
    run_latency_test
    run_throughput_test
    run_concurrent_test
    
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║     Benchmarks Completed! ✓           ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════╝${NC}"
}

# Run if executed directly
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    main "$@"
fi