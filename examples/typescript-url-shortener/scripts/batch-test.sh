#!/bin/bash
# Batch testing script for URL Shortener API
# Usage: ./scripts/batch-test.sh [count] [base_url]

COUNT=${1:-10}
BASE_URL=${2:-http://localhost:3000}

echo "==================================="
echo "URL Shortener Batch Test"
echo "==================================="
echo "Requests: $COUNT"
echo "Base URL: $BASE_URL"
echo ""

# Array to store created short codes
declare -a CODES

echo "--- Creating $COUNT short URLs ---"
for i in $(seq 1 $COUNT); do
  RESPONSE=$(curl -s -X POST "$BASE_URL/shorten" \
    -H "Content-Type: application/json" \
    -d "{\"url\": \"https://example.com/page/$i\", \"expiresIn\": 3600}")

  CODE=$(echo $RESPONSE | grep -o '"shortCode":"[^"]*"' | cut -d'"' -f4)
  CODES+=("$CODE")
  echo "[$i] Created: $CODE"
done

echo ""
echo "--- Testing redirects ---"
for CODE in "${CODES[@]}"; do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/$CODE")
  echo "GET /$CODE -> HTTP $STATUS"
done

echo ""
echo "--- Getting stats ---"
for CODE in "${CODES[@]:0:3}"; do
  STATS=$(curl -s "$BASE_URL/stats/$CODE")
  echo "Stats for $CODE: $STATS"
done

echo ""
echo "--- Rate limit test (rapid requests) ---"
for i in $(seq 1 5); do
  RESPONSE=$(curl -s -w "\nHTTP: %{http_code}" -X POST "$BASE_URL/shorten" \
    -H "Content-Type: application/json" \
    -d "{\"url\": \"https://example.com/ratelimit/$i\"}")
  echo "Request $i: $(echo $RESPONSE | tail -1)"
done

echo ""
echo "--- Cleanup: Deleting test URLs ---"
for CODE in "${CODES[@]}"; do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/$CODE")
  echo "DELETE /$CODE -> HTTP $STATUS"
done

echo ""
echo "==================================="
echo "Batch test complete!"
echo "==================================="
