import { api } from '../client.ts';
import type { GlobalLeaderboardEntry, AllianceLeaderboardEntry } from '../types.ts';

interface GlobalParams {
  /** Exclusive rank cursor (omit for first page). */
  after_rank?: number;
  /** Page size (1–100, default 25). */
  limit?: number;
}

interface AllianceParams {
  /** Composite cursor part 1 (omit for first page). */
  after_trophies?: number;
  /** Composite cursor part 2 — last alliance UUID (omit for first page). */
  after_id?: string;
  limit?: number;
}

export interface GlobalLeaderboardResponse {
  entries: GlobalLeaderboardEntry[];
  next_cursor: number | null;
}

export interface AllianceLeaderboardResponse {
  entries: AllianceLeaderboardEntry[];
  next_cursor_trophies: number | null;
  next_cursor_id: string | null;
}

/** Global player trophy leaderboard with rank-based cursor pagination. */
export function getGlobal(params: GlobalParams = {}): Promise<GlobalLeaderboardResponse> {
  const query = new URLSearchParams();
  if (params.after_rank !== undefined) query.set('after_rank', String(params.after_rank));
  if (params.limit !== undefined) query.set('limit', String(params.limit));
  const qs = query.size > 0 ? `?${query.toString()}` : '';
  return api.get<GlobalLeaderboardResponse>(`/v1/leaderboard/global${qs}`);
}

/** Alliance leaderboard ranked by total member trophies. */
export function getAlliances(params: AllianceParams = {}): Promise<AllianceLeaderboardResponse> {
  const query = new URLSearchParams();
  if (params.after_trophies !== undefined)
    query.set('after_trophies', String(params.after_trophies));
  if (params.after_id !== undefined) query.set('after_id', params.after_id);
  if (params.limit !== undefined) query.set('limit', String(params.limit));
  const qs = query.size > 0 ? `?${query.toString()}` : '';
  return api.get<AllianceLeaderboardResponse>(`/v1/leaderboard/alliances${qs}`);
}
