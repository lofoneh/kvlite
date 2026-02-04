// URL mapping stored in kvlite
export interface UrlMapping {
  shortCode: string;
  longUrl: string;
  createdAt: string;
  expiresAt?: string;
}

// Response for URL creation
export interface CreateUrlResponse {
  shortCode: string;
  shortUrl: string;
  longUrl: string;
  expiresIn?: number;
}

// Response for URL stats
export interface UrlStats {
  shortCode: string;
  longUrl: string;
  clicks: number;
  createdAt: string;
  ttlRemaining: number;
}

// Request body for creating short URL
export interface CreateUrlRequest {
  url: string;
  customCode?: string;
  expiresIn?: number;
}

// Rate limit result
export interface RateLimitResult {
  allowed: boolean;
  remaining: number;
  resetIn: number;
}
