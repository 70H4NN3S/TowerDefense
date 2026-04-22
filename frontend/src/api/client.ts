/**
 * Thin fetch wrapper for the Tower Defense backend.
 *
 * - Attaches Authorization header from the current access token.
 * - Normalizes non-2xx responses into ApiResponseError instances.
 * - Handles 204 No Content by returning undefined.
 */

import type { ApiError } from './types.ts';

const BASE_URL: string = (import.meta as { env: Record<string, string> }).env.VITE_API_URL ?? '';

/** Error thrown for any non-2xx HTTP response. */
export class ApiResponseError extends Error {
  readonly status: number;
  readonly error: ApiError;
  readonly requestId: string | undefined;

  constructor(status: number, error: ApiError, requestId?: string) {
    super(error.message);
    this.name = 'ApiResponseError';
    this.status = status;
    this.error = error;
    this.requestId = requestId;
  }
}

/** Module-level access token, set by the auth state on login/logout. */
let _accessToken: string | null = null;

/**
 * Update the token attached to all subsequent requests.
 * Called by AuthProvider on login, token refresh, and logout.
 */
export function setAccessToken(token: string | null): void {
  _accessToken = token;
}

interface RequestOptions {
  signal?: AbortSignal;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (_accessToken !== null) {
    headers['Authorization'] = `Bearer ${_accessToken}`;
  }

  const response = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal: options?.signal,
  });

  if (!response.ok) {
    const envelope = await response.json().catch(() => ({
      error: { code: 'unknown', message: 'An unexpected error occurred.' },
    }));
    const err = (envelope as { error?: ApiError }).error ?? {
      code: 'unknown',
      message: 'An unexpected error occurred.',
    };
    const requestId = (envelope as { request_id?: string }).request_id;
    throw new ApiResponseError(response.status, err, requestId);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

/** Typed HTTP helpers used by endpoint modules. */
export const api = {
  get<T>(path: string, options?: RequestOptions): Promise<T> {
    return request<T>('GET', path, undefined, options);
  },
  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('POST', path, body, options);
  },
  patch<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('PATCH', path, body, options);
  },
  delete<T>(path: string, options?: RequestOptions): Promise<T> {
    return request<T>('DELETE', path, undefined, options);
  },
};
