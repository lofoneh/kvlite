import { Router, Request, Response, NextFunction } from 'express';
import { urlService } from '../services/url.service';
import { rateLimiter } from '../services/rate-limiter';
import { CreateUrlRequest } from '../types';

const router = Router();

/**
 * Rate limiting middleware for URL creation
 */
async function rateLimitMiddleware(
  req: Request,
  res: Response,
  next: NextFunction
): Promise<void> {
  const identifier = req.ip || 'unknown';
  const result = await rateLimiter.checkLimit(identifier);

  // Set rate limit headers
  res.setHeader('X-RateLimit-Limit', '100');
  res.setHeader('X-RateLimit-Remaining', result.remaining.toString());
  res.setHeader('X-RateLimit-Reset', result.resetIn.toString());

  if (!result.allowed) {
    res.status(429).json({
      error: 'Too Many Requests',
      message: 'Rate limit exceeded. Please try again later.',
      retryAfter: result.resetIn,
    });
    return;
  }

  next();
}

/**
 * POST /shorten - Create a short URL
 */
router.post('/shorten', rateLimitMiddleware, async (req: Request, res: Response) => {
  try {
    const { url, customCode, expiresIn } = req.body as CreateUrlRequest;

    if (!url) {
      res.status(400).json({ error: 'URL is required' });
      return;
    }

    const result = await urlService.createShortUrl({ url, customCode, expiresIn });
    res.status(201).json(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    res.status(400).json({ error: message });
  }
});

/**
 * GET /stats/:code - Get URL statistics
 */
router.get('/stats/:code', async (req: Request, res: Response) => {
  try {
    const { code } = req.params;
    const stats = await urlService.getStats(code);

    if (!stats) {
      res.status(404).json({ error: 'URL not found' });
      return;
    }

    res.json(stats);
  } catch (error) {
    res.status(500).json({ error: 'Internal server error' });
  }
});

/**
 * DELETE /:code - Delete a short URL
 */
router.delete('/:code', async (req: Request, res: Response) => {
  try {
    const { code } = req.params;
    const deleted = await urlService.deleteUrl(code);

    if (!deleted) {
      res.status(404).json({ error: 'URL not found' });
      return;
    }

    res.status(204).send();
  } catch (error) {
    res.status(500).json({ error: 'Internal server error' });
  }
});

/**
 * GET /:code - Redirect to long URL
 */
router.get('/:code', async (req: Request, res: Response) => {
  try {
    const { code } = req.params;
    const longUrl = await urlService.getLongUrl(code);

    if (!longUrl) {
      res.status(404).json({ error: 'URL not found or expired' });
      return;
    }

    res.redirect(302, longUrl);
  } catch (error) {
    res.status(500).json({ error: 'Internal server error' });
  }
});

export default router;
