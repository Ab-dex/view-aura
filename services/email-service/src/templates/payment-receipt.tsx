import React from 'react';
import { Heading, Row, Column, Section, Text, Hr } from '@react-email/components';
import { BaseEmail } from './base';

export interface PaymentReceiptEmailProps {
  displayName:  string;
  plan:         string;   // "pro" | "studio"
  amountCents:  number;
  currency:     string;
  periodEnd:    string;   // ISO-8601 date
  invoiceId:    string;
}

function formatAmount(cents: number, currency: string): string {
  return new Intl.NumberFormat('en-US', {
    style:    'currency',
    currency: currency.toUpperCase(),
  }).format(cents / 100);
}

export function PaymentReceiptEmail({
  displayName,
  plan,
  amountCents,
  currency,
  periodEnd,
  invoiceId,
}: PaymentReceiptEmailProps) {
  const planLabel = plan === 'studio' ? 'Studio' : 'Pro';
  const amount    = formatAmount(amountCents, currency);
  const date      = new Date(periodEnd).toLocaleDateString('en-US', {
    year: 'numeric', month: 'long', day: 'numeric',
  });

  return (
    <BaseEmail preview={`Your ViewAura ${planLabel} receipt — ${amount}`}>
      <Heading className="text-2xl font-bold text-gray-900">
        Payment receipt
      </Heading>
      <Text className="mt-2 text-gray-600">
        Hi {displayName}, thanks for your payment. Here's your receipt.
      </Text>

      <Section className="mt-6 rounded-md bg-gray-50 p-6">
        <Row>
          <Column><Text className="text-sm text-gray-500">Plan</Text></Column>
          <Column className="text-right">
            <Text className="text-sm font-medium text-gray-900">ViewAura {planLabel}</Text>
          </Column>
        </Row>
        <Hr className="my-2 border-gray-200" />
        <Row>
          <Column><Text className="text-sm text-gray-500">Amount</Text></Column>
          <Column className="text-right">
            <Text className="text-sm font-medium text-gray-900">{amount}</Text>
          </Column>
        </Row>
        <Hr className="my-2 border-gray-200" />
        <Row>
          <Column><Text className="text-sm text-gray-500">Next renewal</Text></Column>
          <Column className="text-right">
            <Text className="text-sm font-medium text-gray-900">{date}</Text>
          </Column>
        </Row>
        <Hr className="my-2 border-gray-200" />
        <Row>
          <Column><Text className="text-sm text-gray-500">Invoice ID</Text></Column>
          <Column className="text-right">
            <Text className="font-mono text-xs text-gray-400">{invoiceId}</Text>
          </Column>
        </Row>
      </Section>

      <Text className="mt-6 text-sm text-gray-500">
        Questions about your bill? Reply to this email or visit our{' '}
        <a href="https://viewaura.com/support" className="text-indigo-600">
          Help Centre
        </a>.
      </Text>
    </BaseEmail>
  );
}