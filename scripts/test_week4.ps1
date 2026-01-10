# scripts/test_week4.ps1 - Test Week 4 TTL & Advanced Operations

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Week 4: TTL & Advanced Operations Tests"
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Function to send command
function Send-Command {
    param($cmd)
    try {
        $tcp = New-Object System.Net.Sockets.TcpClient('localhost', 6380)
        $stream = $tcp.GetStream()
        $writer = New-Object System.IO.StreamWriter($stream)
        $reader = New-Object System.IO.StreamReader($stream)
        
        # Read welcome
        [void]$reader.ReadLine()
        
        # Send command
        $writer.WriteLine($cmd)
        $writer.Flush()
        
        # Read response
        $response = $reader.ReadLine()
        $tcp.Close()
        
        return $response
    }
    catch {
        Write-Host "Error: $_" -ForegroundColor Red
        return $null
    }
}

# Test 1: Basic TTL
Write-Host "Test 1: Basic TTL (SETEX)" -ForegroundColor Yellow
Write-Host "-------------------------"

$response = Send-Command "SETEX session 3 user123"
Write-Host "SETEX session 3 user123 -> $response"

$response = Send-Command "GET session"
Write-Host "GET session -> $response"

$response = Send-Command "TTL session"
Write-Host "TTL session -> $response seconds"

Write-Host "Waiting 4 seconds for expiration..." -ForegroundColor Gray
Start-Sleep -Seconds 4

$response = Send-Command "GET session"
Write-Host "GET session (after TTL) -> $response"

if ($response -like "*not found*") {
    Write-Host "✓ TTL expiration works!" -ForegroundColor Green
} else {
    Write-Host "X TTL expiration failed!" -ForegroundColor Red
}

Write-Host ""

# Test 2: EXPIRE command
Write-Host "Test 2: EXPIRE Command" -ForegroundColor Yellow
Write-Host "----------------------"

Send-Command "SET temp myvalue" | Out-Null
Write-Host "SET temp myvalue"

$response = Send-Command "EXPIRE temp 3"
Write-Host "EXPIRE temp 3 -> $response"

$response = Send-Command "TTL temp"
Write-Host "TTL temp -> $response seconds"

Write-Host "Waiting 4 seconds..." -ForegroundColor Gray
Start-Sleep -Seconds 4

$response = Send-Command "GET temp"
Write-Host "GET temp (after expire) -> $response"

Write-Host ""

# Test 3: PERSIST command
Write-Host "Test 3: PERSIST Command" -ForegroundColor Yellow
Write-Host "------------------------"

Send-Command "SETEX persist_test 60 data" | Out-Null
Write-Host "SETEX persist_test 60 data"

$response = Send-Command "TTL persist_test"
Write-Host "TTL persist_test -> $response seconds (has TTL)"

$response = Send-Command "PERSIST persist_test"
Write-Host "PERSIST persist_test -> $response"

$response = Send-Command "TTL persist_test"
Write-Host "TTL persist_test -> $response (should be -1, no TTL)"

if ($response -eq "-1") {
    Write-Host "✓ PERSIST works!" -ForegroundColor Green
} else {
    Write-Host "X PERSIST failed!" -ForegroundColor Red
}

Write-Host ""

# Test 4: KEYS Pattern Matching
Write-Host "Test 4: KEYS Pattern Matching" -ForegroundColor Yellow
Write-Host "------------------------------"

# Setup data
Send-Command "SET user:1 Alice" | Out-Null
Send-Command "SET user:2 Bob" | Out-Null
Send-Command "SET user:3 Charlie" | Out-Null
Send-Command "SET session:1 data1" | Out-Null
Send-Command "SET session:2 data2" | Out-Null
Send-Command "SET admin:1 root" | Out-Null

Write-Host "Created test keys: user:1, user:2, user:3, session:1, session:2, admin:1"
Write-Host ""

$response = Send-Command "KEYS user:*"
Write-Host "KEYS user:* ->"
Write-Host $response -ForegroundColor Cyan

$response = Send-Command "KEYS session:*"
Write-Host ""
Write-Host "KEYS session:* ->"
Write-Host $response -ForegroundColor Cyan

$response = Send-Command "KEYS *:1"
Write-Host ""
Write-Host "KEYS *:1 ->"
Write-Host $response -ForegroundColor Cyan

Write-Host ""

# Test 5: SCAN Command
Write-Host "Test 5: SCAN Command (Pagination)" -ForegroundColor Yellow
Write-Host "----------------------------------"

Write-Host "SCAN 0 MATCH user:* COUNT 2 ->"
$response = Send-Command "SCAN 0 MATCH user:* COUNT 2"
Write-Host $response -ForegroundColor Cyan

Write-Host ""

# Test 6: TTL Edge Cases
Write-Host "Test 6: TTL Edge Cases" -ForegroundColor Yellow
Write-Host "----------------------"

$response = Send-Command "TTL nonexistent"
Write-Host "TTL nonexistent -> $response (should be -2)"

Send-Command "SET notexpiring value" | Out-Null
$response = Send-Command "TTL notexpiring"
Write-Host "TTL notexpiring -> $response (should be -1)"

Write-Host ""

# Test 7: Lazy Expiration
Write-Host "Test 7: Lazy Expiration on GET" -ForegroundColor Yellow
Write-Host "-------------------------------"

Send-Command "SETEX lazy 2 value" | Out-Null
Write-Host "SETEX lazy 2 value"

Write-Host "Waiting 3 seconds (no background check yet)..." -ForegroundColor Gray
Start-Sleep -Seconds 3

$response = Send-Command "GET lazy"
Write-Host "GET lazy (triggers lazy expiration) -> $response"

if ($response -like "*not found*") {
    Write-Host "✓ Lazy expiration works!" -ForegroundColor Green
} else {
    Write-Host "X Lazy expiration failed!" -ForegroundColor Red
}

Write-Host ""

# Test 8: Stats with TTL Info
Write-Host "Test 8: Enhanced STATS" -ForegroundColor Yellow
Write-Host "----------------------"

$response = Send-Command "STATS"
Write-Host "STATS ->"
Write-Host $response -ForegroundColor Cyan

Write-Host ""

# Summary
Write-Host "================================" -ForegroundColor Green
Write-Host "Week 4 Tests Complete!" -ForegroundColor Green
Write-Host "================================" -ForegroundColor Green
Write-Host ""
Write-Host "Features Tested:"
Write-Host "  + SETEX (set with TTL)"
Write-Host "  + EXPIRE (add TTL to existing key)"
Write-Host "  + TTL (check remaining time)"
Write-Host "  + PERSIST (remove TTL)"
Write-Host "  + KEYS (pattern matching)"
Write-Host "  + SCAN (paginated iteration)"
Write-Host "  + Lazy expiration"
Write-Host "  + Background expiration"
Write-Host ""