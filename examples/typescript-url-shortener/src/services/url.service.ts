import { getKVLiteClient, KVLiteClient } from '../kvlite-client';
import { config } from '../config';
import {
  UrlMapping,
  CreateUrlRequest,
  CreateUrlResponse,
  UrlStats,
} from '../types';
import { generateShortCode, isValidShortCode, isValidUrl } from '../utils/shortcode';

/**
 * URL shortening service
 * Demonstrates kvlite patterns: SETEX, GET, DELETE, EXISTS, INCR, TTL
 */
export class UrlService {
  private client: KVLiteClient | null = null;

  private async getClient(): Promise<KVLiteClient> {
    if (!this.client || !this.client.isConnected()) {
      this.client = await getKVLiteClient();
    }
    return this.client;
  }

  /**
   * Create a shortened URL
   */
  async createShortUrl(request: CreateUrlRequest): Promise<CreateUrlResponse> {
    const client = await this.getClient();

    // Validate URL
    if (!isValidUrl(request.url)) {
      throw new Error('Invalid URL. Must be http or https.');
    }

    // Generate or validate short code
    let shortCode: string;
    if (request.customCode) {
      if (!isValidShortCode(request.customCode)) {
        throw new Error('Invalid custom code. Must be 3-20 alphanumeric characters.');
      }
      // Check if custom code already exists
      const existing = await client.get(`url:${request.customCode}`);
      if (existing) {
        throw new Error('Custom code already in use');
      }
      shortCode = request.customCode;
    } else {
      // Generate unique code with collision check
      do {
        shortCode = generateShortCode();
      } while (await client.exists(`url:${shortCode}`));
    }

    // Prepare URL mapping
    const mapping: UrlMapping = {
      shortCode,
      longUrl: request.url,
      createdAt: new Date().toISOString(),
    };

    // Calculate TTL
    const ttl = request.expiresIn
      ? Math.min(request.expiresIn, config.shortener.maxTtl)
      : config.shortener.defaultTtl;

    if (ttl > 0) {
      mapping.expiresAt = new Date(Date.now() + ttl * 1000).toISOString();
    }

    // Store URL mapping with TTL using SETEX
    const urlKey = `url:${shortCode}`;
    const success = await client.setex(urlKey, ttl, JSON.stringify(mapping));

    if (!success) {
      throw new Error('Failed to store URL mapping');
    }

    // Initialize click counter with same TTL
    await client.set(`clicks:${shortCode}`, '0');
    await client.expire(`clicks:${shortCode}`, ttl);

    return {
      shortCode,
      shortUrl: `${config.baseUrl}/${shortCode}`,
      longUrl: request.url,
      expiresIn: ttl,
    };
  }

  /**
   * Get long URL by short code (for redirect)
   * Increments click counter atomically
   */
  async getLongUrl(shortCode: string): Promise<string | null> {
    const client = await this.getClient();

    const urlKey = `url:${shortCode}`;
    const data = await client.get(urlKey);

    if (!data) {
      return null;
    }

    try {
      const mapping: UrlMapping = JSON.parse(data);

      // Increment click counter (fire and forget)
      client.incr(`clicks:${shortCode}`).catch(() => {});

      return mapping.longUrl;
    } catch {
      return null;
    }
  }

  /**
   * Get statistics for a short URL
   */
  async getStats(shortCode: string): Promise<UrlStats | null> {
    const client = await this.getClient();

    const urlKey = `url:${shortCode}`;
    const data = await client.get(urlKey);

    if (!data) {
      return null;
    }

    try {
      const mapping: UrlMapping = JSON.parse(data);

      // Get click count
      const clicksStr = await client.get(`clicks:${shortCode}`);
      const clicks = clicksStr ? parseInt(clicksStr, 10) : 0;

      // Get TTL remaining
      const ttlRemaining = await client.ttl(urlKey);

      return {
        shortCode,
        longUrl: mapping.longUrl,
        clicks,
        createdAt: mapping.createdAt,
        ttlRemaining,
      };
    } catch {
      return null;
    }
  }

  /**
   * Delete a short URL
   */
  async deleteUrl(shortCode: string): Promise<boolean> {
    const client = await this.getClient();

    const urlKey = `url:${shortCode}`;
    const clicksKey = `clicks:${shortCode}`;

    // Check if URL exists
    const exists = await client.exists(urlKey);
    if (!exists) {
      return false;
    }

    // Delete both keys
    await client.delete(urlKey);
    await client.delete(clicksKey);

    return true;
  }
}

export const urlService = new UrlService();
