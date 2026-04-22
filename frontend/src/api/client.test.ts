import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api, ApiResponseError, setAccessToken } from './client.ts';

function mockFetch(status: number, body: unknown): ReturnType<typeof vi.spyOn> {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    new Response(body !== null ? JSON.stringify(body) : null, {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
}

describe('api client', () => {
  beforeEach(() => {
    setAccessToken(null);
    vi.restoreAllMocks();
  });

  describe('successful responses', () => {
    it('returns the parsed JSON body on 200', async () => {
      mockFetch(200, { hello: 'world' });
      const result = await api.get<{ hello: string }>('/test');
      expect(result).toEqual({ hello: 'world' });
    });

    it('returns the parsed JSON body on 201', async () => {
      mockFetch(201, { id: 'abc' });
      const result = await api.post<{ id: string }>('/test', { name: 'x' });
      expect(result).toEqual({ id: 'abc' });
    });

    it('returns undefined for 204 No Content', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(null, { status: 204 }));
      const result = await api.delete<undefined>('/test');
      expect(result).toBeUndefined();
    });
  });

  describe('error responses', () => {
    it('throws ApiResponseError on 4xx with error envelope', async () => {
      mockFetch(404, {
        error: { code: 'not_found', message: 'Not found.' },
        request_id: 'req-123',
      });
      await expect(api.get('/test')).rejects.toThrow(ApiResponseError);
    });

    it('populates code, message, status, and requestId from the envelope', async () => {
      mockFetch(409, {
        error: { code: 'already_exists', message: 'Already exists.' },
        request_id: 'req-456',
      });
      let caught: unknown;
      try {
        await api.post('/test', {});
      } catch (err) {
        caught = err;
      }
      expect(caught).toBeInstanceOf(ApiResponseError);
      const apiErr = caught as ApiResponseError;
      expect(apiErr.status).toBe(409);
      expect(apiErr.error.code).toBe('already_exists');
      expect(apiErr.message).toBe('Already exists.');
      expect(apiErr.requestId).toBe('req-456');
    });

    it('throws ApiResponseError on 5xx', async () => {
      mockFetch(500, {
        error: { code: 'internal', message: 'Something went wrong.' },
      });
      await expect(api.get('/test')).rejects.toThrow(ApiResponseError);
    });

    it('falls back to a generic error if the body is not valid JSON', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
        new Response('not json', { status: 503 }),
      );
      let caught: unknown;
      try {
        await api.get('/test');
      } catch (err) {
        caught = err;
      }
      expect(caught).toBeInstanceOf(ApiResponseError);
      const apiErr = caught as ApiResponseError;
      expect(apiErr.status).toBe(503);
      expect(apiErr.error.code).toBe('unknown');
    });
  });

  describe('authorization header', () => {
    it('attaches Authorization header when a token is set', async () => {
      const spy = mockFetch(200, {});
      setAccessToken('my-access-token');
      await api.get('/test');
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer my-access-token');
    });

    it('omits Authorization header when no token is set', async () => {
      const spy = mockFetch(200, {});
      await api.get('/test');
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBeUndefined();
    });

    it('clears the token after setAccessToken(null)', async () => {
      setAccessToken('old-token');
      setAccessToken(null);
      const spy = mockFetch(200, {});
      await api.get('/test');
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBeUndefined();
    });
  });

  describe('http methods', () => {
    it('uses GET method', async () => {
      const spy = mockFetch(200, {});
      await api.get('/test');
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      expect(init.method).toBe('GET');
    });

    it('uses POST method and serializes body', async () => {
      const spy = mockFetch(200, {});
      await api.post('/test', { key: 'value' });
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      expect(init.method).toBe('POST');
      expect(init.body).toBe('{"key":"value"}');
    });

    it('uses PATCH method and serializes body', async () => {
      const spy = mockFetch(200, {});
      await api.patch('/test', { key: 'value' });
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      expect(init.method).toBe('PATCH');
    });

    it('uses DELETE method with no body', async () => {
      const spy = mockFetch(204, null);
      await api.delete('/test');
      const [, init] = spy.mock.calls[0] as [string, RequestInit];
      expect(init.method).toBe('DELETE');
      expect(init.body).toBeUndefined();
    });
  });
});
