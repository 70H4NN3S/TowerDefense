/**
 * Shared data-transfer types mirroring the backend API contract.
 * Every type here corresponds 1-to-1 with a JSON shape documented in
 * docs/api-contract.md.
 */

/* ─── Auth ──────────────────────────────────────────────────────────────────── */

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  /** Seconds until the access token expires (always 3600). */
  expires_in: number;
}

/* ─── Profile ────────────────────────────────────────────────────────────────── */

export interface Profile {
  user_id: string;
  display_name: string;
  avatar_id: number;
  trophies: number;
  gold: number;
  diamonds: number;
  energy: number;
  energy_max: number;
  xp: number;
  level: number;
}

/* ─── Towers ─────────────────────────────────────────────────────────────────── */

export type Rarity = 'common' | 'rare' | 'epic' | 'legendary';

export interface TowerStats {
  level: number;
  /** Gold cost to upgrade to the next level; 0 at max level. */
  gold_cost: number;
  damage: number;
  range: number;
  rate: number;
}

export interface CatalogTower {
  id: string;
  name: string;
  rarity: Rarity;
  base_damage: number;
  base_range: number;
  base_rate: number;
  cost_diamonds: number;
  description: string;
  owned: boolean;
}

export interface OwnedTower {
  template_id: string;
  name: string;
  rarity: Rarity;
  cost_diamonds: number;
  description: string;
  current: TowerStats;
}

/* ─── Matches ────────────────────────────────────────────────────────────────── */

export interface Match {
  id: string;
  player_one: string;
  mode: string;
  map_id: string;
  seed: number;
  started_at: string;
  ended_at: string | null;
  winner: string | null;
}

export interface MatchResult {
  match: Match;
  gold_awarded: number;
  trophy_delta: number;
}

/* ─── Chat ───────────────────────────────────────────────────────────────────── */

export interface ChatMessage {
  id: string;
  channel_id: string;
  user_id: string;
  body: string;
  created_at: string;
}

/* ─── Alliances ──────────────────────────────────────────────────────────────── */

export type AllianceRole = 'leader' | 'officer' | 'member';

export interface Alliance {
  id: string;
  name: string;
  tag: string;
  description: string;
  leader_id: string;
  channel_id: string;
  created_at: string;
}

export interface AllianceMember {
  user_id: string;
  alliance_id: string;
  role: AllianceRole;
  joined_at: string;
}

export interface AllianceInvite {
  id: string;
  alliance_id: string;
  user_id: string;
  status: 'pending' | 'accepted' | 'declined';
  created_at: string;
}

/* ─── Leaderboard ────────────────────────────────────────────────────────────── */

export interface GlobalLeaderboardEntry {
  rank: number;
  user_id: string;
  trophies: number;
}

export interface AllianceLeaderboardEntry {
  alliance_id: string;
  alliance_name: string;
  alliance_tag: string;
  total_trophies: number;
  member_count: number;
}

export interface AllianceMemberLeaderboardEntry {
  rank: number;
  user_id: string;
  role: AllianceRole;
  trophies: number;
}

/* ─── Events ─────────────────────────────────────────────────────────────────── */

export interface GameEvent {
  id: string;
  kind: string;
  name: string;
  description: string;
  starts_at: string;
  ends_at: string;
  /** Opaque config object; shape depends on `kind`. */
  config: Record<string, unknown>;
}

export interface EventRewards {
  gold?: number;
  diamonds?: number;
  [key: string]: number | undefined;
}

/* ─── API errors ─────────────────────────────────────────────────────────────── */

export interface ValidationFieldError {
  field: string;
  reason: string;
}

export interface ApiErrorDetails {
  fields?: ValidationFieldError[];
}

export interface ApiError {
  code: string;
  message: string;
  details?: ApiErrorDetails;
}
