import { getKVLiteClient, KVLiteClient } from '../kvlite-client';
import { config } from '../config';
import { RateLimitResult } from '../types';

/**
 * Fixed-window rate limiter using kvlite
 * Based on the pattern from examples/rate_limiting/main.go
 */
export class RateLimiter {
  private client: KVLiteClient | null = null;

  /**
   * Check rate limit for an identifier (IP, API key, etc.)
   */
  async checkLimit(identifier: string): Promise<RateLimitResult> {
    if (!config.rateLimit.enabled) {
      return { allowed: true, remaining: Infinity, resetIn: 0 };
    }

    this.client = await getKVLiteClient();

    const windowSeconds = config.rateLimit.windowSeconds;
    const maxRequests = config.rateLimit.maxRequests;

    // Time-bucketed key for fixed window rate limiting
    const window = Math.floor(Date.now() / 1000 / windowSeconds);
    const key = `ratelimit:urlshorten:${identifier}:${window}`;

    try {
      // Atomic increment
      const count = await this.client.incr(key);

      // Set expiry on first request in window
      if (count === 1) {
        await this.client.expire(key, windowSeconds);
      }

      const allowed = count <= maxRequests;
      const remaining = Math.max(0, maxRequests - count);

      // Calculate seconds until window resets
      const currentSecond = Math.floor(Date.now() / 1000);
      const windowStart = window * windowSeconds;
      const resetIn = windowSeconds - (currentSecond - windowStart);

      return { allowed, remaining, resetIn };
    } catch (error) {
      // On error, allow request but log
      console.error('Rate limit check failed:', error);
      return { allowed: true, remaining: 0, resetIn: 0 };
    }
  }
}

export const rateLimiter = new RateLimiter();
