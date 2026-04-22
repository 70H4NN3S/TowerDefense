/**
 * Abstract token storage so that tests can inject an in-memory implementation
 * and native builds can swap in Capacitor Preferences (Phase 16).
 */

export interface TokenStore {
  getAccessToken(): string | null;
  getRefreshToken(): string | null;
  setTokens(access: string, refresh: string): void;
  clearTokens(): void;
}

const ACCESS_KEY = 'td_access_token';
const REFRESH_KEY = 'td_refresh_token';

/** Production implementation backed by localStorage. */
export function createLocalTokenStore(): TokenStore {
  return {
    getAccessToken() {
      return localStorage.getItem(ACCESS_KEY);
    },
    getRefreshToken() {
      return localStorage.getItem(REFRESH_KEY);
    },
    setTokens(access, refresh) {
      localStorage.setItem(ACCESS_KEY, access);
      localStorage.setItem(REFRESH_KEY, refresh);
    },
    clearTokens() {
      localStorage.removeItem(ACCESS_KEY);
      localStorage.removeItem(REFRESH_KEY);
    },
  };
}

/** In-memory implementation used in tests. */
export function createMemoryTokenStore(): TokenStore {
  let accessToken: string | null = null;
  let refreshToken: string | null = null;

  return {
    getAccessToken() {
      return accessToken;
    },
    getRefreshToken() {
      return refreshToken;
    },
    setTokens(access, refresh) {
      accessToken = access;
      refreshToken = refresh;
    },
    clearTokens() {
      accessToken = null;
      refreshToken = null;
    },
  };
}
