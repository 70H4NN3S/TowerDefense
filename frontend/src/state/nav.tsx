import { useState, useCallback, type ReactNode } from 'react';
import {
  NavContext,
  isTab,
  createLocalNavStorage,
  type Tab,
  type NavStorage,
} from '@/state/nav.ts';

interface NavProviderProps {
  children: ReactNode;
  /**
   * Storage backend. Defaults to localStorage.
   * Pass `createMemoryNavStorage()` in tests.
   */
  storage?: NavStorage;
}

/**
 * NavProvider manages which tab is currently visible.
 * The active tab is persisted via the storage backend so that a page
 * reload returns the user to the same section.
 */
export function NavProvider({ children, storage: storageProp }: NavProviderProps) {
  // useState initializer runs exactly once — stable reference.
  const [storage] = useState<NavStorage>(() => storageProp ?? createLocalNavStorage());

  const [activeTab, setActiveTab] = useState<Tab>(() => {
    const stored = storage.getActiveTab();
    return isTab(stored) ? stored : 'main';
  });

  const navigate = useCallback(
    (tab: Tab) => {
      setActiveTab(tab);
      storage.setActiveTab(tab);
    },
    [storage],
  );

  return <NavContext.Provider value={{ activeTab, navigate }}>{children}</NavContext.Provider>;
}
