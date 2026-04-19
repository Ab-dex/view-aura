import React from 'react';
import { Button, Heading, Text } from '@react-email/components';
import { BaseEmail } from './base';
import { config } from '../config';

export interface WelcomeEmailProps {
  displayName: string;
  email:       string;
}

export function WelcomeEmail({ displayName }: WelcomeEmailProps) {
  return (
    <BaseEmail preview={`Welcome to ViewAura, ${displayName}!`}>
      <Heading className="text-2xl font-bold text-gray-900">
        Welcome to ViewAura 🎬
      </Heading>
      <Text className="mt-4 text-gray-700">
        Hey {displayName}, your account is ready. Start discovering films,
        building your watchlist, and connecting with other cinephiles.
      </Text>
      <Button
        href={`${config.APP_URL}/discover`}
        className="mt-6 rounded-md bg-indigo-600 px-6 py-3 text-white font-semibold"
      >
        Explore ViewAura
      </Button>
      <Text className="mt-6 text-sm text-gray-500">
        If you didn't create this account, you can safely ignore this email.
      </Text>
    </BaseEmail>
  );
}