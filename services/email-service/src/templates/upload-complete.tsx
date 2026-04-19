import React from 'react';
import { Button, Heading, Text } from '@react-email/components';
import { BaseEmail } from './base';
import { config } from '../config';

export interface UploadCompleteEmailProps {
  displayName: string;
  assetId:     string;
  streamUrl:   string;
}

export function UploadCompleteEmail({
  displayName,
  streamUrl,
}: UploadCompleteEmailProps) {
  return (
    <BaseEmail preview="Your upload is live on ViewAura! 🎬">
      <Heading className="text-2xl font-bold text-gray-900">
        Your video is live! 🎬
      </Heading>
      <Text className="mt-4 text-gray-700">
        Great news, {displayName}! Your upload has been processed and is now
        available on ViewAura. Transcoding, moderation, and CDN publishing are
        all complete.
      </Text>
      <Button
        href={streamUrl}
        className="mt-6 rounded-md bg-indigo-600 px-6 py-3 text-white font-semibold"
      >
        Watch your video
      </Button>
      <Text className="mt-6 text-sm text-gray-500">
        It may take a few minutes for the video to appear in search results.
      </Text>
    </BaseEmail>
  );
}