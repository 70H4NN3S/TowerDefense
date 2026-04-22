import { api } from '../client.ts';
import type { AuthTokens } from '../types.ts';

interface RegisterRequest {
  email: string;
  username: string;
  password: string;
}

interface LoginRequest {
  email: string;
  password: string;
}

interface RefreshRequest {
  refresh_token: string;
}

/** Register a new account. Rate-limited: 10 req/min per IP. */
export function register(body: RegisterRequest): Promise<AuthTokens> {
  return api.post<AuthTokens>('/v1/auth/register', body);
}

/** Log in with email and password. Rate-limited: 10 req/min per IP. */
export function login(body: LoginRequest): Promise<AuthTokens> {
  return api.post<AuthTokens>('/v1/auth/login', body);
}

/**
 * Exchange a refresh token for a new token pair.
 * The old refresh token is invalidated on use — store the returned one.
 */
export function refresh(body: RefreshRequest): Promise<AuthTokens> {
  return api.post<AuthTokens>('/v1/auth/refresh', body);
}
