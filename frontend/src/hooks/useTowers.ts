import { useState, useEffect, useCallback } from 'react';
import { getOwned, upgrade } from '@/api/endpoints/towers.ts';
import type { OwnedTower } from '@/api/types.ts';

export interface UseTowersResult {
  towers: OwnedTower[];
  isLoading: boolean;
  error: string | null;
  upgradeTower: (templateId: string) => Promise<void>;
  refresh: () => void;
}

/**
 * Fetches the current player's owned towers and exposes an upgrade action.
 * The upgrade action updates the local tower list optimistically so the UI
 * reflects the new stats immediately without a full re-fetch.
 */
export function useTowers(): UseTowersResult {
  const [towers, setTowers] = useState<OwnedTower[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let cancelled = false;
    getOwned()
      .then(({ towers: fetched }) => {
        if (!cancelled) {
          setTowers(fetched);
          setError(null);
          setIsLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load towers.');
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

  const upgradeTower = useCallback(async (templateId: string) => {
    const { tower } = await upgrade(templateId);
    setTowers((prev) => prev.map((t) => (t.template_id === templateId ? tower : t)));
  }, []);

  return { towers, isLoading, error, upgradeTower, refresh };
}
