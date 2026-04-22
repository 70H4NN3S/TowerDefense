import { useState, useEffect, useCallback } from 'react';
import { getCatalog, buyTower } from '@/api/endpoints/shop.ts';
import type { CatalogTower } from '@/api/types.ts';

export interface UseShopResult {
  catalog: CatalogTower[];
  isLoading: boolean;
  error: string | null;
  buy: (templateId: string) => Promise<void>;
  refresh: () => void;
}

/**
 * Fetches the tower catalog (with per-entry `owned` flag) and exposes a
 * buy action. After a successful purchase the catalog entry is updated
 * locally so the UI reflects the new ownership without a round-trip.
 */
export function useShop(): UseShopResult {
  const [catalog, setCatalog] = useState<CatalogTower[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let cancelled = false;
    getCatalog()
      .then(({ towers }) => {
        if (!cancelled) {
          setCatalog(towers);
          setError(null);
          setIsLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load shop.');
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

  const buy = useCallback(async (templateId: string) => {
    await buyTower(templateId);
    // Mark the tower as owned locally; a full refresh keeps the catalog clean.
    setCatalog((prev) => prev.map((t) => (t.id === templateId ? { ...t, owned: true } : t)));
  }, []);

  return { catalog, isLoading, error, buy, refresh };
}
