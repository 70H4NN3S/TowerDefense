/**
 * Auth context definition — kept separate from the provider so that
 * auth.tsx can export only the AuthProvider component (required by
 * react-refresh/only-export-components).
 */

import { createContext } from 'react';
import type { Profile } from '@/api/types.ts';

export interface AuthState {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: Profile | null;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, username: string, password: string) => Promise<void>;
  logout: () => void;
}

export const AuthContext = createContext<AuthState | null>(null);
