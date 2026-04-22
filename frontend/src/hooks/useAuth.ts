import { useContext } from 'react';
import { AuthContext, type AuthState } from '@/state/auth-context.ts';

/**
 * Returns the current auth state and actions (login, register, logout).
 * Must be called inside an AuthProvider.
 */
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (ctx === null) {
    throw new Error('useAuth must be called inside <AuthProvider>');
  }
  return ctx;
}
