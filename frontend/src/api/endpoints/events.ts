import { api } from '../client.ts';
import type { GameEvent, EventRewards } from '../types.ts';

/** Fetch events that are currently active or start within the next 7 days. */
export function getActive(): Promise<{ events: GameEvent[] }> {
  return api.get<{ events: GameEvent[] }>('/v1/events');
}

/**
 * Claim a reward tier for an event.
 * `tier` is zero-based.
 */
export function claimTier(eventId: string, tier: number): Promise<{ rewards: EventRewards }> {
  return api.post<{ rewards: EventRewards }>(`/v1/events/${eventId}/claim`, {
    tier,
  });
}
