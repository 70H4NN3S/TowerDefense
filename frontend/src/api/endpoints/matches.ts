import { api } from '../client.ts';
import type { Match, MatchResult } from '../types.ts';

interface StartMatchRequest {
  map_id: string;
}

interface SubmitResultRequest {
  monsters_killed: number;
  waves_cleared: number;
  gate_hp: number;
  victory: boolean;
  gold_earned: number;
}

/** Start a solo match. Costs 1 energy. */
export function startMatch(body: StartMatchRequest): Promise<{ match: Match }> {
  return api.post<{ match: Match }>('/v1/matches', body);
}

/** Submit the result of a completed match for server-side validation. */
export function submitResult(matchId: string, body: SubmitResultRequest): Promise<MatchResult> {
  return api.post<MatchResult>(`/v1/matches/${matchId}/result`, body);
}

/** Join the matchmaking queue. */
export function joinMatchmaking(body: StartMatchRequest): Promise<{ status: string }> {
  return api.post<{ status: string }>('/v1/matchmaking/join', body);
}

/** Leave the matchmaking queue. */
export function leaveMatchmaking(): Promise<undefined> {
  return api.delete<undefined>('/v1/matchmaking/leave');
}
