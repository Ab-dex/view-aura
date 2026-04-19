import React from 'react';
import { Button, Heading, Text } from '@react-email/components';
import { BaseEmail } from './base';
import { config } from '../config';

export interface SubscriptionCancelledEmailProps {
  displayName: string;
  plan:        string;
  accessUntil: string; // ISO-8601
}

export function SubscriptionCancelledEmail({
  displayName,
  plan,
  accessUntil,
}: SubscriptionCancelledEmailProps) {
  const planLabel = plan === 'studio' ? 'Studio' : 'Pro';
  const date = new Date(accessUntil).toLocaleDateString('en-US', {
    year: 'numeric', month: 'long', day: 'numeric',
  });

  return (
    <BaseEmail preview={`Your ViewAura ${planLabel} subscription has been cancelled`}>
      <Heading className="text-2xl font-bold text-gray-900">
        Subscription cancelled
      </Heading>
      <Text className="mt-4 text-gray-700">
        Hi {displayName}, we've cancelled your ViewAura {planLabel} subscription
        as requested.
      </Text>
      <Text className="text-gray-700">
        Your {planLabel} access continues until{' '}
        <strong>{date}</strong>. After that date, your account will revert to
        the free plan.
      </Text>
      <Button
        href={`${config.APP_URL}/settings/billing`}
        className="mt-6 rounded-md bg-indigo-600 px-6 py-3 text-white font-semibold"
      >
        Reactivate subscription
      </Button>
      <Text className="mt-6 text-sm text-gray-500">
        Changed your mind? You can reactivate at any time before {date}.
      </Text>
    </BaseEmail>
  );
}