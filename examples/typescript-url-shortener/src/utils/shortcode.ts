import { customAlphabet } from 'nanoid';
import { config } from '../config';

// URL-safe alphabet without ambiguous characters (0/O, 1/l/I)
const alphabet = '23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz';
const nanoid = customAlphabet(alphabet, config.shortener.codeLength);

/**
 * Generate a random short code
 */
export function generateShortCode(): string {
  return nanoid();
}

/**
 * Validate a custom short code (3-20 alphanumeric characters)
 */
export function isValidShortCode(code: string): boolean {
  return /^[a-zA-Z0-9]{3,20}$/.test(code);
}

/**
 * Validate a URL (must be http or https)
 */
export function isValidUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}
