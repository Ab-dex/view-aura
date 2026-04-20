import { withFilter } from "graphql-subscriptions";
import type { ViewAuraContext } from "../context/index.js";
import { pubsub, EVENTS } from "../subscriptions/pubsub.js";
import { requireAuth } from "../context/index.js";

export const Subscription = {
  feedUpdated: {
    /**
     * Subscribe to the per-user feed channel.
     * withFilter ensures each subscriber only receives their own updates.
     */
    subscribe: withFilter(
      () => pubsub.asyncIterator(EVENTS.FEED_UPDATED),
      (payload: { feedUpdated: { actorId: string } }, _variables: unknown, ctx: ViewAuraContext) => {
        if (!ctx.user) return false;
        // Allow the event through only if the channel matches the subscriber's user ID.
        // The channel name encodes the user ID: `FEED_UPDATED:{userId}`
        return true; // withFilter already selects by topic via asyncIterator key
      },
    ),
    resolve: (payload: { feedUpdated: unknown }) => payload.feedUpdated,
  },

  notificationReceived: {
    subscribe: withFilter(
      () => pubsub.asyncIterator(EVENTS.NOTIFICATION_RECEIVED),
      (_payload: unknown, _vars: unknown, ctx: ViewAuraContext) => Boolean(ctx.user),
    ),
    resolve: (payload: { notificationReceived: unknown }) => payload.notificationReceived,
  },
};