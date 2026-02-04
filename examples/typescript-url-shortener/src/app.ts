import express, { Application, Request, Response } from 'express';
import urlRoutes from './routes/url.routes';
import { getKVLiteClient } from './kvlite-client';

export async function createApp(): Promise<Application> {
  const app: Application = express();

  // Middleware
  app.use(express.json());
  app.use(express.urlencoded({ extended: true }));

  // Trust proxy for accurate IP addresses
  app.set('trust proxy', 1);

  // Health check endpoint
  app.get('/health', async (_req: Request, res: Response) => {
    try {
      const client = await getKVLiteClient();
      const healthy = await client.ping();

      if (healthy) {
        res.json({ status: 'healthy', kvlite: 'connected' });
      } else {
        res.status(503).json({ status: 'unhealthy', kvlite: 'disconnected' });
      }
    } catch {
      res.status(503).json({ status: 'unhealthy', kvlite: 'error' });
    }
  });

  // URL shortener routes
  app.use('/', urlRoutes);

  // 404 handler
  app.use((_req: Request, res: Response) => {
    res.status(404).json({ error: 'Not Found' });
  });

  return app;
}
