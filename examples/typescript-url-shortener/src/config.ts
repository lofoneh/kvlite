import dotenv from 'dotenv';

dotenv.config();

export const config = {
  // Server configuration
  port: parseInt(process.env.PORT || '3000', 10),
  baseUrl: process.env.BASE_URL || 'http://localhost:3000',

  // KVLite configuration
  kvlite: {
    host: process.env.KVLITE_HOST || 'localhost',
    port: parseInt(process.env.KVLITE_PORT || '6380', 10),
    connectionTimeout: 5000,
    commandTimeout: 3000,
  },

  // URL shortener settings
  shortener: {
    codeLength: 7,
    defaultTtl: 86400 * 30, // 30 days
    maxTtl: 86400 * 365,    // 1 year
  },

  // Rate limiting
  rateLimit: {
    enabled: process.env.RATE_LIMIT_ENABLED !== 'false',
    maxRequests: parseInt(process.env.RATE_LIMIT_MAX_REQUESTS || '100', 10),
    windowSeconds: parseInt(process.env.RATE_LIMIT_WINDOW_SECONDS || '60', 10),
  },
};
