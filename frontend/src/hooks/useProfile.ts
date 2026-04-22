import { useAuth } from '@/hooks/useAuth.ts';
import type { Profile } from '@/api/types.ts';

export interface UseProfileResult {
  profile: Profile | null;
  isLoading: boolean;
  /** Re-fetch the profile from the server and update the shared auth state. */
  refresh: () => Promise<void>;
}

/**
 * Exposes the current user's profile and a refresh action.
 * The profile lives in the auth context so all screens share the same data
 * without duplicate fetches.
 */
export function useProfile(): UseProfileResult {
  const { user, isLoading, refreshProfile } = useAuth();
  return { profile: user, isLoading, refresh: refreshProfile };
}
