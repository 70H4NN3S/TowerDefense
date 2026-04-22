import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { setAccessToken } from '@/api/client.ts';
import * as authApi from '@/api/endpoints/auth.ts';
import * as profileApi from '@/api/endpoints/profile.ts';
import { createLocalTokenStore, type TokenStore } from '@/lib/storage.ts';
import type { Profile } from '@/api/types.ts';
import { AuthContext } from '@/state/auth-context.ts';

interface AuthProviderProps {
  children: ReactNode;
  /**
   * Token storage backend.
   * Defaults to localStorage. Pass `createMemoryTokenStore()` in tests.
   */
  store?: TokenStore;
}

/**
 * AuthProvider manages the session lifecycle: restores tokens on mount,
 * exposes login / register / logout actions, and keeps the Profile in sync.
 */
export function AuthProvider({ children, store: storeProp }: AuthProviderProps) {
  // useState initializer runs exactly once — store reference is stable.
  const [store] = useState<TokenStore>(() => storeProp ?? createLocalTokenStore());

  const [user, setUser] = useState<Profile | null>(null);

  // isLoading starts true only when there is a stored token to validate.
  // Computing synchronously avoids calling setState inside the effect.
  const [isLoading, setIsLoading] = useState(() => store.getAccessToken() !== null);

  // Restore session from stored tokens on mount.
  useEffect(() => {
    const accessToken = store.getAccessToken();
    if (accessToken === null) {
      return;
    }
    setAccessToken(accessToken);
    profileApi
      .getMe()
      .then((profile) => {
        setUser(profile);
      })
      .catch(() => {
        // Stored token has expired or is invalid — clear it.
        store.clearTokens();
        setAccessToken(null);
      })
      .finally(() => {
        setIsLoading(false);
      });
  }, [store]);

  const login = useCallback(
    async (email: string, password: string) => {
      const tokens = await authApi.login({ email, password });
      store.setTokens(tokens.access_token, tokens.refresh_token);
      setAccessToken(tokens.access_token);
      const profile = await profileApi.getMe();
      setUser(profile);
    },
    [store],
  );

  const register = useCallback(
    async (email: string, username: string, password: string) => {
      const tokens = await authApi.register({ email, username, password });
      store.setTokens(tokens.access_token, tokens.refresh_token);
      setAccessToken(tokens.access_token);
      const profile = await profileApi.getMe();
      setUser(profile);
    },
    [store],
  );

  const logout = useCallback(() => {
    store.clearTokens();
    setAccessToken(null);
    setUser(null);
  }, [store]);

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: user !== null,
        isLoading,
        user,
        login,
        register,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
