import { config } from './config';
import { logger }  from './logger';
import { startConsumer } from './consumer';

async function main() {
  logger.info({ env: config.NODE_ENV }, 'email-service starting');

  const consumer = await startConsumer();

  // ── Graceful shutdown ──────────────────────────────────────────────────────
  async function shutdown(signal: string) {
    logger.info({ signal }, 'shutdown signal received');
    if (consumer) {
      await consumer.disconnect();
      logger.info('kafka consumer disconnected');
    }
    process.exit(0);
  }

  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT',  () => shutdown('SIGINT'));
  process.on('uncaughtException', (err) => {
    logger.error({ err }, 'uncaught exception — exiting');
    process.exit(1);
  });
  process.on('unhandledRejection', (reason) => {
    logger.error({ reason }, 'unhandled rejection — exiting');
    process.exit(1);
  });

  logger.info('email-service ready');
}

main().catch((err) => {
  console.error('fatal startup error:', err);
  process.exit(1);
});