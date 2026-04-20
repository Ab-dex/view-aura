/**
 * PubSub engine for GraphQL subscriptions.
 *
 * Current backend: in-process PubSubEngine from graphql-subscriptions.
 * This works for a single pod but does NOT fan out across multiple BFF pods.
 *
 * Production upgrade path: replace with a Redis-backed engine
 * (graphql-redis-subscriptions) using the same SUBSCRIBE/PUBLISH pattern
 * so the Go API can trigger subscriptions by publishing to Redis channels.
 * The channel naming convention is already designed for that:
 *   FEED_UPDATED:{userId}
 *   NOTIFICATION:{userId}
 */
import { PubSub } from "graphql-subscriptions";

export const pubsub = new PubSub();

export const EVENTS = {
  FEED_UPDATED:          "FEED_UPDATED",
  NOTIFICATION_RECEIVED: "NOTIFICATION_RECEIVED",
} as const;

export type EventName = (typeof EVENTS)[keyof typeof EVENTS];

/** Publish a feed update for a specific user. */
export function publishFeedUpdate(userId: string, item: unknown): void {
  pubsub.publish(`${EVENTS.FEED_UPDATED}:${userId}`, { feedUpdated: item });
}

/** Publish a notification for a specific user. */
export function publishNotification(userId: string, notification: unknown): void {
  pubsub.publish(`${EVENTS.NOTIFICATION_RECEIVED}:${userId}`, { notificationReceived: notification });
}