import { Kafka, Consumer, EachMessagePayload } from 'kafkajs';
import React from 'react';
import { config } from '../config';
import { logger } from '../logger';
import { sendEmail } from '../mailer';
import { WelcomeEmail }               from '../templates/welcome';
import { PaymentReceiptEmail }         from '../templates/payment-receipt';
import { PaymentFailedEmail }          from '../templates/payment-failed';
import { UploadCompleteEmail }         from '../templates/upload-complete';
import { SubscriptionCancelledEmail }  from '../templates/subscription-cancelled';
import { ModerationDecisionEmail }     from '../templates/moderation-decision';

// ─── Topic names (mirror internal/events/topics.go) ──────────────────────────

const TOPICS = [
  'user.events',
  'payment.events',
  'uploads.completed',
  'moderation.decided',
] as const;

// ─── Envelope (matches events.Envelope in Go) ─────────────────────────────────

interface Envelope {
  id:         string;
  event_type: string;
  payload:    Record<string, unknown>;
}

// ─── Event payload types ──────────────────────────────────────────────────────

interface UserRegisteredPayload {
  user_id:      string;
  email:        string;
  display_name: string;
}

interface PaymentSucceededPayload {
  invoice_id:      string;
  subscription_id: string;
  user_id:         string;
  amount_cents:    number;
  currency:        string;
  stripe_invoice_id: string;
  plan?:           string;
  period_end?:     string;
  // The email service needs the user's email address. Since the payment
  // event only carries user_id, production would look this up via the
  // Go API. For now the event is expected to include it via denorm.
  email?:          string;
  display_name?:   string;
}

interface PaymentFailedPayload {
  user_id:        string;
  amount_cents:   number;
  currency:       string;
  failure_reason: string;
  retry_at?:      string;
  email?:         string;
  display_name?:  string;
}

interface SubscriptionCancelledPayload {
  user_id:         string;
  plan:            string;
  access_until:    string;
  email?:          string;
  display_name?:   string;
}

interface AssetPublishedPayload {
  asset_id:  string;
  user_id:   string;
  stream_url: string;
  email?:    string;
  display_name?: string;
}

interface ModerationDecidedPayload {
  case_id:      string;
  content_type: string;
  content_id:   string;
  actor_id:     string;
  decision:     string;
  reason?:      string;
  email?:       string;
  display_name?: string;
}

// ─── Consumer ─────────────────────────────────────────────────────────────────

export async function startConsumer(): Promise<Consumer | null> {
  if (!config.KAFKA_BROKERS) {
    logger.warn('KAFKA_BROKERS not set — email consumer disabled');
    return null;
  }

  const kafka = new Kafka({
    clientId: 'email-service',
    brokers:  config.KAFKA_BROKERS.split(',').map(b => b.trim()),
    retry: { retries: 10, initialRetryTime: 300 },
  });

  const consumer = kafka.consumer({
    groupId:       config.KAFKA_GROUP_ID,
    sessionTimeout: 30_000,
  });

  await consumer.connect();
  await consumer.subscribe({ topics: [...TOPICS], fromBeginning: false });

  logger.info({ topics: TOPICS }, 'kafka consumer connected');

  await consumer.run({
    eachMessage: async (payload: EachMessagePayload) => {
      const { topic, partition, message } = payload;
      const raw = message.value?.toString();
      if (!raw) return;

      let envelope: Envelope;
      try {
        envelope = JSON.parse(raw) as Envelope;
      } catch {
        logger.warn({ topic, partition, offset: message.offset }, 'non-JSON message — skipping');
        return;
      }

      try {
        await dispatch(envelope);
      } catch (err) {
        // Log and continue — a failed email send should not stall the consumer.
        logger.error(
          { err, event_type: envelope.event_type, envelope_id: envelope.id },
          'failed to process event'
        );
      }
    },
  });

  return consumer;
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

async function dispatch(envelope: Envelope): Promise<void> {
  logger.debug({ event_type: envelope.event_type }, 'dispatching event');

  switch (envelope.event_type) {
    case 'user.registered':
      return handleUserRegistered(envelope.payload as UserRegisteredPayload);

    case 'payment.payment_succeeded':
      return handlePaymentSucceeded(envelope.payload as PaymentSucceededPayload);

    case 'payment.payment_failed':
      return handlePaymentFailed(envelope.payload as PaymentFailedPayload);

    case 'payment.subscription_cancelled':
      return handleSubscriptionCancelled(envelope.payload as SubscriptionCancelledPayload);

    case 'upload.asset_published':
      return handleAssetPublished(envelope.payload as AssetPublishedPayload);

    case 'moderation.decided':
      return handleModerationDecided(envelope.payload as ModerationDecidedPayload);

    default:
      logger.debug({ event_type: envelope.event_type }, 'no email handler — ignoring');
  }
}

// ─── Event handlers ───────────────────────────────────────────────────────────

async function handleUserRegistered(p: UserRegisteredPayload) {
  await sendEmail({
    to:       p.email,
    subject:  'Welcome to ViewAura 🎬',
    template: React.createElement(WelcomeEmail, {
      displayName: p.display_name,
      email:       p.email,
    }),
  });
}

async function handlePaymentSucceeded(p: PaymentSucceededPayload) {
  if (!p.email) return;   // email address not denormalised into event — skip
  await sendEmail({
    to:      p.email,
    subject: `Your ViewAura receipt — ${formatAmount(p.amount_cents, p.currency)}`,
    template: React.createElement(PaymentReceiptEmail, {
      displayName:  p.display_name ?? 'there',
      plan:         p.plan ?? 'pro',
      amountCents:  p.amount_cents,
      currency:     p.currency,
      periodEnd:    p.period_end ?? new Date().toISOString(),
      invoiceId:    p.stripe_invoice_id,
    }),
  });
}

async function handlePaymentFailed(p: PaymentFailedPayload) {
  if (!p.email) return;
  await sendEmail({
    to:      p.email,
    subject: `Action required: payment of ${formatAmount(p.amount_cents, p.currency)} failed`,
    template: React.createElement(PaymentFailedEmail, {
      displayName:   p.display_name ?? 'there',
      amountCents:   p.amount_cents,
      currency:      p.currency,
      failureReason: p.failure_reason,
      retryAt:       p.retry_at,
    }),
  });
}

async function handleSubscriptionCancelled(p: SubscriptionCancelledPayload) {
  if (!p.email) return;
  await sendEmail({
    to:      p.email,
    subject: 'Your ViewAura subscription has been cancelled',
    template: React.createElement(SubscriptionCancelledEmail, {
      displayName: p.display_name ?? 'there',
      plan:        p.plan,
      accessUntil: p.access_until,
    }),
  });
}

async function handleAssetPublished(p: AssetPublishedPayload) {
  if (!p.email) return;
  await sendEmail({
    to:      p.email,
    subject: 'Your upload is live on ViewAura! 🎬',
    template: React.createElement(UploadCompleteEmail, {
      displayName: p.display_name ?? 'there',
      assetId:     p.asset_id,
      streamUrl:   p.stream_url,
    }),
  });
}

async function handleModerationDecided(p: ModerationDecidedPayload) {
  if (!p.email) return;
  const subjectMap: Record<string, string> = {
    approved:  `Your ${p.content_type} has been approved ✅`,
    rejected:  `Your ${p.content_type} has been removed`,
    escalated: `Your ${p.content_type} is under further review`,
  };
  await sendEmail({
    to:      p.email,
    subject: subjectMap[p.decision] ?? `Update on your ${p.content_type}`,
    template: React.createElement(ModerationDecisionEmail, {
      displayName:  p.display_name ?? 'there',
      contentType:  p.content_type,
      decision:     p.decision,
      reason:       p.reason,
      contentId:    p.content_id,
    }),
  });
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatAmount(cents: number, currency: string): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency', currency: currency.toUpperCase(),
  }).format(cents / 100);
}