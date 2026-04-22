import { useState, useEffect, useCallback } from 'react';
import { getMyAlliance } from '@/api/endpoints/profile.ts';
import { getAlliance, getMembers, createAlliance } from '@/api/endpoints/alliances.ts';
import type { Alliance, AllianceMember } from '@/api/types.ts';

export interface UseAllianceResult {
  /** The player's alliance, or null if they aren't in one. */
  alliance: Alliance | null;
  members: AllianceMember[];
  isLoading: boolean;
  error: string | null;
  create: (name: string, tag: string, description: string) => Promise<void>;
  refresh: () => void;
}

/**
 * Fetches the current player's alliance membership and, if they are a
 * member, the full alliance detail and roster.
 */
export function useAlliance(): UseAllianceResult {
  const [alliance, setAlliance] = useState<Alliance | null>(null);
  const [members, setMembers] = useState<AllianceMember[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const { membership } = await getMyAlliance();
        if (cancelled) return;

        const [allianceData, membersData] = await Promise.all([
          getAlliance(membership.alliance_id),
          getMembers(membership.alliance_id),
        ]);
        if (cancelled) return;
        setAlliance(allianceData.alliance);
        setMembers(membersData.members);
        setError(null);
        setIsLoading(false);
      } catch (err: unknown) {
        if (cancelled) return;
        // A not_found response means the player is not in an alliance — valid state.
        const isNotFound =
          err instanceof Error &&
          (err.message.includes('not_found') || err.message.includes('404'));
        if (isNotFound) {
          setAlliance(null);
          setMembers([]);
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load alliance.');
        }
        setIsLoading(false);
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [revision]);

  const refresh = useCallback(() => {
    setIsLoading(true);
    setRevision((r) => r + 1);
  }, []);

  const create = useCallback(
    async (name: string, tag: string, description: string) => {
      await createAlliance({ name, tag, description });
      refresh();
    },
    [refresh],
  );

  return { alliance, members, isLoading, error, create, refresh };
}
