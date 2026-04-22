import { api } from '../client.ts';
import type {
  Alliance,
  AllianceMember,
  AllianceMemberLeaderboardEntry,
  AllianceInvite,
} from '../types.ts';

interface CreateAllianceRequest {
  name: string;
  tag: string;
  description: string;
}

interface InviteRequest {
  user_id: string;
}

/** Create a new alliance. The requester becomes its leader. */
export function createAlliance(body: CreateAllianceRequest): Promise<{ alliance: Alliance }> {
  return api.post<{ alliance: Alliance }>('/v1/alliances', body);
}

/** Fetch alliance details. */
export function getAlliance(id: string): Promise<{ alliance: Alliance }> {
  return api.get<{ alliance: Alliance }>(`/v1/alliances/${id}`);
}

/** Disband an alliance. Leader only. */
export function disbandAlliance(id: string): Promise<undefined> {
  return api.delete<undefined>(`/v1/alliances/${id}`);
}

/** List all members of an alliance. */
export function getMembers(id: string): Promise<{ members: AllianceMember[] }> {
  return api.get<{ members: AllianceMember[] }>(`/v1/alliances/${id}/members`);
}

/** Kick a member. Leader or officer only. */
export function kickMember(allianceId: string, userId: string): Promise<undefined> {
  return api.delete<undefined>(`/v1/alliances/${allianceId}/members/${userId}`);
}

/** Promote a member to officer. Leader only. */
export function promoteMember(allianceId: string, userId: string): Promise<undefined> {
  return api.post<undefined>(`/v1/alliances/${allianceId}/members/${userId}/promote`);
}

/** Demote an officer back to member. Leader only. */
export function demoteMember(allianceId: string, userId: string): Promise<undefined> {
  return api.post<undefined>(`/v1/alliances/${allianceId}/members/${userId}/demote`);
}

/** Send an alliance invite. Leader or officer only. */
export function sendInvite(
  allianceId: string,
  body: InviteRequest,
): Promise<{ invite: AllianceInvite }> {
  return api.post<{ invite: AllianceInvite }>(`/v1/alliances/${allianceId}/invites`, body);
}

/** Accept a pending invite addressed to the current user. */
export function acceptInvite(inviteId: string): Promise<undefined> {
  return api.post<undefined>(`/v1/invites/${inviteId}/accept`);
}

/** Decline a pending invite. */
export function declineInvite(inviteId: string): Promise<undefined> {
  return api.post<undefined>(`/v1/invites/${inviteId}/decline`);
}

/** Per-alliance member trophy leaderboard. */
export function getAllianceLeaderboard(
  id: string,
): Promise<{ entries: AllianceMemberLeaderboardEntry[] }> {
  return api.get<{ entries: AllianceMemberLeaderboardEntry[] }>(`/v1/alliances/${id}/leaderboard`);
}
