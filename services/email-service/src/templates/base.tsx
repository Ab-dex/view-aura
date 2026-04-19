import React from 'react';
import {
  Body, Container, Head, Html, Img, Link,
  Preview, Section, Tailwind, Text,
} from '@react-email/components';
import { config } from '../config';

interface BaseEmailProps {
  preview:  string;
  children: React.ReactNode;
}

/** Shared layout: ViewAura logo header + content area + footer */
export function BaseEmail({ preview, children }: BaseEmailProps) {
  return (
    <Html lang="en">
      <Head />
      <Preview>{preview}</Preview>
      <Tailwind>
        <Body className="bg-gray-50 font-sans">
          <Container className="mx-auto my-10 max-w-lg rounded-lg bg-white p-8 shadow-sm">

            {/* Logo */}
            <Section className="mb-8 text-center">
              <Img
                src={config.LOGO_URL}
                alt="ViewAura"
                width={140}
                height={32}
                className="mx-auto"
              />
            </Section>

            {/* Content slot */}
            {children}

            {/* Footer */}
            <Section className="mt-10 border-t border-gray-100 pt-6 text-center">
              <Text className="text-xs text-gray-400">
                © {new Date().getFullYear()} ViewAura. All rights reserved.
              </Text>
              <Text className="text-xs text-gray-400">
                <Link href={config.UNSUBSCRIBE_URL} className="text-gray-400 underline">
                  Unsubscribe
                </Link>{' '}
                · 123 Film Row, San Francisco, CA 94107
              </Text>
            </Section>

          </Container>
        </Body>
      </Tailwind>
    </Html>
  );
}