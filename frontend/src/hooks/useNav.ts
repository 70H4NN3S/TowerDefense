import { useContext } from 'react';
import { NavContext, type NavState } from '@/state/nav.ts';

/**
 * Returns the current navigation state and a `navigate` action.
 * Must be called inside a NavProvider.
 */
export function useNav(): NavState {
  const ctx = useContext(NavContext);
  if (ctx === null) {
    throw new Error('useNav must be called inside <NavProvider>');
  }
  return ctx;
}
