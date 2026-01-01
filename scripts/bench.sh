#!/bin/bash
# scripts/simple_bench.sh - Simple, reliable benchmark script

set -e

echo "╔═══════════════════════════════════════╗"
echo "║      kvlite Benchmark Suite           ║"
echo "╚═══════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# Check if server is running
check_server() {
    if command -v powershell &> /dev/null; then
        if ! powershell -Command "
            try {
                \$tcp = New-Object System.Net.Sockets.TcpClient('localhost', 6380);
                \$tcp.Close();
                exit 0;
            } catch {
                exit 1;
            }
        " 2>/dev/null; then
            echo "❌ Server not running on localhost:6380"
            echo "Start it with: ./bin/kvlite.exe"
            exit 1
        fi
    fi
    echo -e "${GREEN}✓${NC} Server is running"
}

# Run Go benchmarks and capture results
run_go_benchmarks() {
    echo ""
    echo -e "${BLUE}Go Store Benchmarks${NC}"
    echo "─────────────────────────────────────────"
    
    # Run benchmarks and save output
    local bench_output=$(go test -bench=. -benchmem ./internal/store/ 2>&1)
    echo "$bench_output" | grep -E "Benchmark|ok"
    
    # Extract performance metrics
    BENCH_SET=$(echo "$bench_output" | grep "BenchmarkStore_Set-" | awk '{print $3}')
    BENCH_GET=$(echo "$bench_output" | grep "BenchmarkStore_Get-" | awk '{print $3}')
    BENCH_CONCURRENT=$(echo "$bench_output" | grep "BenchmarkStore_ConcurrentReadWrite-" | awk '{print $3}')
    
    # Calculate ops/sec (convert ns to ops/sec: 1,000,000,000 / ns)
    if [ -n "$BENCH_SET" ]; then
        SET_OPS=$(awk "BEGIN {printf \"%.1f\", 1000000000 / $BENCH_SET / 1000000}")
    fi
    if [ -n "$BENCH_GET" ]; then
        GET_OPS=$(awk "BEGIN {printf \"%.1f\", 1000000000 / $BENCH_GET / 1000000}")
    fi
    if [ -n "$BENCH_CONCURRENT" ]; then
        CONCURRENT_OPS=$(awk "BEGIN {printf \"%.1f\", 1000000000 / $BENCH_CONCURRENT / 1000000}")
    fi
}

# Run simple network test
run_network_test() {
    echo ""
    echo -e "${BLUE}Network Performance Test${NC}"
    echo "─────────────────────────────────────────"
    
    if command -v powershell &> /dev/null; then
        echo "Testing 100 round-trips..."
        
        local network_result=$(powershell -Command "
            \$ErrorActionPreference = 'SilentlyContinue'
            \$start = Get-Date
            \$success = 0
            
            for (\$i=1; \$i -le 100; \$i++) {
                try {
                    \$tcp = New-Object System.Net.Sockets.TcpClient('localhost', 6380)
                    \$stream = \$tcp.GetStream()
                    \$writer = New-Object System.IO.StreamWriter(\$stream)
                    \$reader = New-Object System.IO.StreamReader(\$stream)
                    
                    # Read welcome
                    [void]\$reader.ReadLine()
                    
                    # PING test
                    \$writer.WriteLine('PING')
                    \$writer.Flush()
                    [void]\$reader.ReadLine()
                    
                    \$tcp.Close()
                    \$success++
                }
                catch {
                    # Silent fail
                }
            }
            
            \$end = Get-Date
            \$elapsed = (\$end - \$start).TotalMilliseconds
            \$avg = \$elapsed / \$success
            \$throughput = [math]::Round(1000/\$avg, 0)
            
            Write-Output \"\$success|\$([math]::Round(\$avg, 2))|\$throughput\"
        ")
        
        # Parse results
        IFS='|' read -r NET_SUCCESS NET_LATENCY NET_THROUGHPUT <<< "$network_result"
        
        echo -e "${GREEN}✓${NC} Completed $NET_SUCCESS/100 requests"
        echo -e "${GREEN}✓${NC} Average latency: ${NET_LATENCY}ms per request"
        echo -e "${GREEN}✓${NC} Throughput: ~${NET_THROUGHPUT} requests/sec"
    else
        echo "PowerShell not available, skipping network test"
        NET_SUCCESS="N/A"
        NET_LATENCY="N/A"
        NET_THROUGHPUT="N/A"
    fi
}

# Summary
show_summary() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║   Benchmark Results Summary           ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════╝${NC}"
    echo ""
    echo "In-Memory Performance:"
    
    if [ -n "$BENCH_SET" ] && [ -n "$SET_OPS" ]; then
        echo "  • SET: ${BENCH_SET} ns/op (${SET_OPS}M ops/sec)"
    else
        echo "  • SET: No data"
    fi
    
    if [ -n "$BENCH_GET" ] && [ -n "$GET_OPS" ]; then
        echo "  • GET: ${BENCH_GET} ns/op (${GET_OPS}M ops/sec)"
    else
        echo "  • GET: No data"
    fi
    
    if [ -n "$BENCH_CONCURRENT" ] && [ -n "$CONCURRENT_OPS" ]; then
        echo "  • Concurrent: ${BENCH_CONCURRENT} ns/op (${CONCURRENT_OPS}M ops/sec)"
    else
        echo "  • Concurrent: No data"
    fi
    
    echo ""
    echo "Network Performance:"
    
    if [ "$NET_LATENCY" != "N/A" ] && [ -n "$NET_LATENCY" ]; then
        echo "  • Latency: ${NET_LATENCY}ms per request"
        echo "  • Throughput: ${NET_THROUGHPUT} requests/sec"
        echo "  • Success rate: ${NET_SUCCESS}/100"
    else
        echo "  • Network tests not available"
    fi
    
    echo ""
    echo -e "${GREEN}✓ All benchmarks complete!${NC}"
}

# Main
main() {
    check_server
    run_go_benchmarks
    run_network_test
    show_summary
}

main "$@"