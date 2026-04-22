/**
 * Navigation context definition.
 * Kept separate from the provider (nav.tsx) so that nav.tsx can export
 * only the NavProvider component (satisfies react-refresh/only-export-components).
 */

import { createContext } from 'react';

export type Tab = 'shop' | 'towers' | 'main' | 'alliance' | 'events';

export interface NavState {
  activeTab: Tab;
  navigate: (tab: Tab) => void;
}

export const NavContext = createContext<NavState | null>(null);

const VALID_TABS = new Set<string>(['shop', 'towers', 'main', 'alliance', 'events']);

/** Type guard — checks a storage value against the known tab set. */
export function isTab(value: string | null): value is Tab {
  return value !== null && VALID_TABS.has(value);
}

/** Abstract storage so that tests can inject an in-memory implementation. */
export interface NavStorage {
  getActiveTab(): string | null;
  setActiveTab(tab: Tab): void;
}

const NAV_STORAGE_KEY = 'nav-active-tab';

/** Production implementation backed by localStorage. */
export function createLocalNavStorage(): NavStorage {
  return {
    getActiveTab() {
      try {
        return localStorage.getItem(NAV_STORAGE_KEY);
      } catch {
        return null;
      }
    },
    setActiveTab(tab) {
      try {
        localStorage.setItem(NAV_STORAGE_KEY, tab);
      } catch {
        // Private browsing or quota errors — ignore.
      }
    },
  };
}

/** In-memory implementation for tests. */
export function createMemoryNavStorage(initial?: Tab): NavStorage {
  let current: Tab | null = initial ?? null;
  return {
    getActiveTab() {
      return current;
    },
    setActiveTab(tab) {
      current = tab;
    },
  };
}
