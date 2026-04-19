import { z } from 'zod';
import 'dotenv/config';

const schema = z.object({
  NODE_ENV:              z.enum(['local', 'staging', 'production']).default('local'),
  LOG_LEVEL:             z.string().default('info'),

  // Kafka — empty brokers string disables the consumer (local dev without Kafka)
  KAFKA_BROKERS:         z.string().default(''),
  KAFKA_GROUP_ID:        z.string().default('email-service'),
  KAFKA_AUTO_OFFSET_RESET: z.enum(['earliest', 'latest']).default('earliest'),

  SENDGRID_API_KEY:      z.string().min(1),
  FROM_EMAIL:            z.string().email().default('hello@viewaura.com'),
  FROM_NAME:             z.string().default('ViewAura'),

  APP_URL:               z.string().url().default('https://viewaura.com'),
  UNSUBSCRIBE_URL:       z.string().url().default('https://viewaura.com/unsubscribe'),
  LOGO_URL:              z.string().url().default('https://viewaura.com/logo.png'),
});

function load() {
  const result = schema.safeParse(process.env);
  if (!result.success) {
    console.error('❌ Invalid configuration:', result.error.flatten().fieldErrors);
    process.exit(1);
  }
  return result.data;
}

export const config = load();
export type Config = typeof config;