/**
 * Load testing script for URL Shortener API
 * Run with: npx ts-node scripts/load-test.ts
 */

const BASE_URL = process.env.BASE_URL || 'http://localhost:3000';
const TOTAL_REQUESTS = parseInt(process.env.REQUESTS || '100', 10);
const CONCURRENCY = parseInt(process.env.CONCURRENCY || '10', 10);

interface TestResult {
  operation: string;
  success: boolean;
  duration: number;
  statusCode?: number;
}

const results: TestResult[] = [];

async function makeRequest(
  method: string,
  path: string,
  body?: object
): Promise<TestResult> {
  const start = performance.now();
  const operation = `${method} ${path}`;

  try {
    const response = await fetch(`${BASE_URL}${path}`, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      redirect: 'manual',
    });

    const duration = performance.now() - start;
    return {
      operation,
      success: response.status < 400,
      duration,
      statusCode: response.status,
    };
  } catch (error) {
    const duration = performance.now() - start;
    return {
      operation,
      success: false,
      duration,
    };
  }
}

interface ShortenResponse {
  shortCode?: string;
  shortUrl?: string;
  error?: string;
}

async function runBatch(batchSize: number): Promise<string[]> {
  const codes: string[] = [];
  const promises: Promise<void>[] = [];

  for (let i = 0; i < batchSize; i++) {
    const promise = (async () => {
      // Create URL
      const createResult = await makeRequest('POST', '/shorten', {
        url: `https://example.com/test/${Date.now()}/${i}`,
        expiresIn: 300,
      });
      results.push(createResult);

      if (createResult.success && createResult.statusCode === 201) {
        const response = await fetch(`${BASE_URL}/shorten`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            url: `https://example.com/load/${Date.now()}/${i}`,
            expiresIn: 300,
          }),
        });
        const data = (await response.json()) as ShortenResponse;
        if (data.shortCode) {
          codes.push(data.shortCode);

          // Test redirect
          const redirectResult = await makeRequest('GET', `/${data.shortCode}`);
          results.push(redirectResult);

          // Get stats
          const statsResult = await makeRequest('GET', `/stats/${data.shortCode}`);
          results.push(statsResult);
        }
      }
    })();
    promises.push(promise);
  }

  await Promise.all(promises);
  return codes;
}

function printStats(): void {
  const successful = results.filter((r) => r.success);
  const failed = results.filter((r) => !r.success);
  const durations = results.map((r) => r.duration);

  const avg = durations.reduce((a, b) => a + b, 0) / durations.length;
  const sorted = [...durations].sort((a, b) => a - b);
  const p50 = sorted[Math.floor(sorted.length * 0.5)];
  const p95 = sorted[Math.floor(sorted.length * 0.95)];
  const p99 = sorted[Math.floor(sorted.length * 0.99)];
  const min = sorted[0];
  const max = sorted[sorted.length - 1];

  console.log('\n========================================');
  console.log('           LOAD TEST RESULTS           ');
  console.log('========================================');
  console.log(`Total Requests:  ${results.length}`);
  console.log(`Successful:      ${successful.length} (${((successful.length / results.length) * 100).toFixed(1)}%)`);
  console.log(`Failed:          ${failed.length}`);
  console.log('');
  console.log('Response Times (ms):');
  console.log(`  Min:    ${min.toFixed(2)}`);
  console.log(`  Avg:    ${avg.toFixed(2)}`);
  console.log(`  P50:    ${p50.toFixed(2)}`);
  console.log(`  P95:    ${p95.toFixed(2)}`);
  console.log(`  P99:    ${p99.toFixed(2)}`);
  console.log(`  Max:    ${max.toFixed(2)}`);
  console.log('');

  // Group by operation
  const byOperation: Record<string, TestResult[]> = {};
  for (const r of results) {
    const op = r.operation.split(' ')[0];
    if (!byOperation[op]) byOperation[op] = [];
    byOperation[op].push(r);
  }

  console.log('By Operation:');
  for (const [op, opResults] of Object.entries(byOperation)) {
    const opDurations = opResults.map((r) => r.duration);
    const opAvg = opDurations.reduce((a, b) => a + b, 0) / opDurations.length;
    const opSuccess = opResults.filter((r) => r.success).length;
    console.log(`  ${op.padEnd(6)} - ${opResults.length} requests, ${opAvg.toFixed(2)}ms avg, ${opSuccess} success`);
  }

  console.log('========================================\n');
}

async function main(): Promise<void> {
  console.log('URL Shortener Load Test');
  console.log(`Base URL: ${BASE_URL}`);
  console.log(`Total Requests: ${TOTAL_REQUESTS}`);
  console.log(`Concurrency: ${CONCURRENCY}`);
  console.log('');

  // Health check
  console.log('Checking server health...');
  const health = await makeRequest('GET', '/health');
  if (!health.success) {
    console.error('Server is not healthy. Aborting.');
    process.exit(1);
  }
  console.log('Server is healthy. Starting load test...\n');

  const startTime = performance.now();
  const batches = Math.ceil(TOTAL_REQUESTS / CONCURRENCY);
  let allCodes: string[] = [];

  for (let i = 0; i < batches; i++) {
    const batchSize = Math.min(CONCURRENCY, TOTAL_REQUESTS - i * CONCURRENCY);
    process.stdout.write(`\rBatch ${i + 1}/${batches} (${batchSize} concurrent)...`);
    const codes = await runBatch(batchSize);
    allCodes = allCodes.concat(codes);
  }

  const totalTime = performance.now() - startTime;
  console.log(`\n\nCompleted in ${(totalTime / 1000).toFixed(2)}s`);
  console.log(`Throughput: ${(results.length / (totalTime / 1000)).toFixed(2)} req/s`);

  printStats();

  // Cleanup
  console.log('Cleaning up test data...');
  for (const code of allCodes) {
    await makeRequest('DELETE', `/${code}`);
  }
  console.log(`Deleted ${allCodes.length} test URLs.`);
}

main().catch(console.error);
