import React from 'react';
import { Button, Heading, Text } from '@react-email/components';
import { BaseEmail } from './base';
import { config } from '../config';

export interface PaymentFailedEmailProps {
  displayName:   string;
  amountCents:   number;
  currency:      string;
  failureReason: string;
  retryAt?:      string;   // ISO-8601 datetime
}

function formatAmount(cents: number, currency: string): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency', currency: currency.toUpperCase(),
  }).format(cents / 100);
}

export function PaymentFailedEmail({
  displayName,
  amountCents,
  currency,
  failureReason,
  retryAt,
}: PaymentFailedEmailProps) {
  const amount = formatAmount(amountCents, currency);

  const reasonMap: Record<string, string> = {
    card_declined:        'Your card was declined.',
    insufficient_funds:   'Your card has insufficient funds.',
    expired_card:         'Your card has expired.',
    incorrect_cvc:        'The security code was incorrect.',
  };
  const reason = reasonMap[failureReason] ?? 'Your payment could not be processed.';

  const retryDate = retryAt
    ? new Date(retryAt).toLocaleDateString('en-US', { month: 'long', day: 'numeric' })
    : null;

  return (
    <BaseEmail preview={`Action required: ${amount} payment failed`}>
      <Heading className="text-2xl font-bold text-gray-900">
        Payment failed ⚠️
      </Heading>
      <Text className="mt-4 text-gray-700">
        Hi {displayName}, we couldn't charge {amount} to your card on file.
        {' '}{reason}
      </Text>
      {retryDate && (
        <Text className="text-gray-600">
          We'll automatically retry on <strong>{retryDate}</strong>. Update your
          payment method before then to avoid interruption.
        </Text>
      )}
      <Button
        href={`${config.APP_URL}/settings/billing`}
        className="mt-6 rounded-md bg-red-600 px-6 py-3 text-white font-semibold"
      >
        Update payment method
      </Button>
      <Text className="mt-6 text-sm text-gray-500">
        Your ViewAura access remains active until the retry window closes.
      </Text>
    </BaseEmail>
  );
}