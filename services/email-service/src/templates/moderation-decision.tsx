import React from 'react';
import { Button, Heading, Text } from '@react-email/components';
import { BaseEmail } from './base';
import { config } from '../config';

export interface ModerationDecisionEmailProps {
  displayName:  string;
  contentType:  string;  // "review" | "upload" | "post"
  decision:     string;  // "approved" | "rejected" | "escalated"
  reason?:      string;
  contentId:    string;
}

export function ModerationDecisionEmail({
  displayName,
  contentType,
  decision,
  reason,
}: ModerationDecisionEmailProps) {
  const label = contentType.charAt(0).toUpperCase() + contentType.slice(1);

  const copy: Record<string, { heading: string; body: string; cta?: string }> = {
    approved: {
      heading: `Your ${label} has been approved ✅`,
      body: `Good news, ${displayName}! After review, your ${contentType} is now live on ViewAura.`,
      cta: 'View your content',
    },
    rejected: {
      heading: `Your ${label} was removed ❌`,
      body: `Hi ${displayName}, after review your ${contentType} was removed because it doesn't meet our community guidelines.${reason ? ` Reason: ${reason}.` : ''}`,
      cta: 'Submit an appeal',
    },
    escalated: {
      heading: `Your ${label} is under review 🔍`,
      body: `Hi ${displayName}, your ${contentType} has been escalated to a senior moderator for further review. We'll email you when a decision is made.`,
    },
  };

  const { heading, body, cta } = copy[decision] ?? copy.escalated;
  const ctaUrl = decision === 'rejected'
    ? `${config.APP_URL}/appeals/new`
    : `${config.APP_URL}/profile`;

  return (
    <BaseEmail preview={heading}>
      <Heading className="text-2xl font-bold text-gray-900">{heading}</Heading>
      <Text className="mt-4 text-gray-700">{body}</Text>
      {cta && (
        <Button
          href={ctaUrl}
          className="mt-6 rounded-md bg-indigo-600 px-6 py-3 text-white font-semibold"
        >
          {cta}
        </Button>
      )}
      <Text className="mt-6 text-sm text-gray-500">
        If you have questions, visit our{' '}
        <a href="https://viewaura.com/community-guidelines" className="text-indigo-600">
          Community Guidelines
        </a>.
      </Text>
    </BaseEmail>
  );
}