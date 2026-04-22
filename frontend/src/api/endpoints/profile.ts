import { api } from '../client.ts';
import type { Profile, AllianceMember } from '../types.ts';

interface UpdateProfileRequest {
  display_name?: string;
  avatar_id?: number;
}

/**
 * Fetch the current player's profile.
 * Creates the profile row on first call (returns 201 which is still OK).
 */
export function getMe(): Promise<Profile> {
  return api.get<Profile>('/v1/me');
}

/**
 * Update display name and/or avatar.
 * Both fields are optional; omit to leave unchanged.
 */
export function updateMe(body: UpdateProfileRequest): Promise<Profile> {
  return api.patch<Profile>('/v1/me', body);
}

/** Get the current user's alliance membership, if any. */
export function getMyAlliance(): Promise<{ membership: AllianceMember }> {
  return api.get<{ membership: AllianceMember }>('/v1/me/alliance');
}

/** Leave the current alliance. */
export function leaveAlliance(): Promise<undefined> {
  return api.post<undefined>('/v1/me/alliance/leave');
}
