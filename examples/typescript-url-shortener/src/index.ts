import { createApp } from './app';
import { config } from './config';
import { getKVLiteClient } from './kvlite-client';

async function main() {
  try {
    // Connect to kvlite
    console.log(`Connecting to kvlite at ${config.kvlite.host}:${config.kvlite.port}...`);
    const client = await getKVLiteClient();

    if (await client.ping()) {
      console.log('Connected to kvlite successfully');
    }

    // Create and start Express app
    const app = await createApp();

    app.listen(config.port, () => {
      console.log(`URL Shortener API running at http://localhost:${config.port}`);
      console.log(`Base URL for short links: ${config.baseUrl}`);
      console.log('\nEndpoints:');
      console.log('  POST   /shorten      - Create short URL');
      console.log('  GET    /:code        - Redirect to long URL');
      console.log('  GET    /stats/:code  - Get URL statistics');
      console.log('  DELETE /:code        - Delete short URL');
      console.log('  GET    /health       - Health check');
    });

    // Graceful shutdown
    const shutdown = async () => {
      console.log('\nShutting down...');
      await client.close();
      process.exit(0);
    };

    process.on('SIGTERM', shutdown);
    process.on('SIGINT', shutdown);
  } catch (error) {
    console.error('Failed to start server:', error);
    process.exit(1);
  }
}

main();
