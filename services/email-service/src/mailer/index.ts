import sgMail from '@sendgrid/mail';
import { render } from '@react-email/render';
import React from 'react';
import { logger } from '../logger';
import { config } from '../config';

sgMail.setApiKey(config.SENDGRID_API_KEY);

export interface SendParams {
  to:       string;
  subject:  string;
  /** React Email component instance, e.g. <WelcomeEmail {...props} /> */
  template: React.ReactElement;
}

/**
 * Send a single transactional email via SendGrid.
 * Renders the React Email component to HTML + plain text before sending.
 * Throws on delivery failure so the Kafka consumer can log the error
 * and continue (at-least-once delivery — a failed email is not retried
 * to avoid duplicate sends; the loss is logged for manual follow-up).
 */
export async function sendEmail({ to, subject, template }: SendParams): Promise<void> {
  const html  = await render(template);
  const text  = await render(template, { plainText: true });

  await sgMail.send({
    to,
    from: { email: config.FROM_EMAIL, name: config.FROM_NAME },
    subject,
    html,
    text,
    trackingSettings: {
      clickTracking:     { enable: false },   // respect user privacy
      openTracking:      { enable: false },
      subscriptionTracking: { enable: true },
    },
  });

  logger.info({ to, subject }, 'email sent');
}