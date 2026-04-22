import { useState, useEffect, useCallback } from 'react';
import { getActive, claimTier } from '@/api/endpoints/events.ts';
import type { GameEvent } from '@/api/types.ts';

export interface UseEventsResult {
  events: GameEvent[];
  isLoading: boolean;
  error: string | null;
  claim: (eventId: string, tier: number) => Promise<void>;
  refresh: () => void;
}

/**
 * Fetches currently active and upcoming events.
 * The `claim` action calls the backend and triggers a refresh so the
 * UI reflects any updated reward state.
 */
export function useEvents(): UseEventsResult {
  const [events, setEvents] = useState<GameEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let cancelled = false;
    getActive()
      .then(({ events: fetched }) => {
        if (!cancelled) {
          setEvents(fetched);
          setError(null);
          setIsLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load events.');
          setIsLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [revision]);

  const refresh = useCallback(() => {
    setIsLoading(true);
    setRevision((r) => r + 1);
  }, []);

  const claim = useCallback(
    async (eventId: string, tier: number) => {
      await claimTier(eventId, tier);
      refresh();
    },
    [refresh],
  );

  return { events, isLoading, error, claim, refresh };
}
