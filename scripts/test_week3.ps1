# scripts/test_week3.ps1 - Test Week 3 compaction features

Write-Host "================================"
Write-Host "Week 3 Compaction Tests"
Write-Host "================================"
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

# Test 1: Automatic Compaction
Write-Host "Test 1: Automatic Compaction" -ForegroundColor Cyan
Write-Host "------------------------------"
Write-Host "Writing 15 entries (threshold is 10)..."

for ($i=1; $i -le 15; $i++) {
    $response = Send-Command "SET key$i value$i"
    if ($response -eq "+OK") {
        Write-Host "  ✓ SET key$i" -ForegroundColor Green
    } else {
        Write-Host "  ✗ SET key$i failed: $response" -ForegroundColor Red
    }
    Start-Sleep -Milliseconds 100
}

Write-Host ""
Write-Host "Waiting for automatic compaction..."
Start-Sleep -Seconds 5

Write-Host ""
Write-Host "Checking stats..."
$stats = Send-Command "STATS"
Write-Host "Stats: $stats" -ForegroundColor Yellow
Write-Host ""

# Test 2: Manual Compaction
Write-Host "Test 2: Manual Compaction" -ForegroundColor Cyan
Write-Host "-------------------------"

Write-Host "Adding more data..."
for ($i=16; $i -le 20; $i++) {
    Send-Command "SET key$i value$i" | Out-Null
}

Write-Host "Triggering manual compaction..."
$response = Send-Command "COMPACT"
Write-Host "Response: $response" -ForegroundColor Yellow

if ($response -eq "+OK") {
    Write-Host "✓ Compaction succeeded!" -ForegroundColor Green
} else {
    Write-Host "✗ Compaction failed!" -ForegroundColor Red
}

Write-Host ""
$stats = Send-Command "STATS"
Write-Host "Stats after compaction: $stats" -ForegroundColor Yellow
Write-Host ""

# Test 3: Check Files
Write-Host "Test 3: Check Files" -ForegroundColor Cyan
Write-Host "-------------------"

if (Test-Path ".\data\kvlite.snapshot") {
    $size = (Get-Item ".\data\kvlite.snapshot").Length
    Write-Host "✓ Snapshot exists: $size bytes" -ForegroundColor Green
    
    Write-Host ""
    Write-Host "Snapshot preview:"
    Get-Content ".\data\kvlite.snapshot" | Select-Object -First 10
} else {
    Write-Host "✗ Snapshot not found!" -ForegroundColor Red
}

Write-Host ""

if (Test-Path ".\data\kvlite.wal") {
    $size = (Get-Item ".\data\kvlite.wal").Length
    Write-Host "✓ WAL exists: $size bytes" -ForegroundColor Green
} else {
    Write-Host "✗ WAL not found!" -ForegroundColor Red
}

Write-Host ""
Write-Host "================================"
Write-Host "Tests Complete!"
Write-Host "================================"