import { api } from '../client.ts';
import type { OwnedTower } from '../types.ts';

/** List all towers the player owns with their current level stats. */
export function getOwned(): Promise<{ towers: OwnedTower[] }> {
  return api.get<{ towers: OwnedTower[] }>('/v1/towers');
}

/**
 * Upgrade an owned tower one level, spending the gold cost from its
 * current stats. `templateId` is the tower template UUID.
 */
export function upgrade(templateId: string): Promise<{ tower: OwnedTower }> {
  return api.post<{ tower: OwnedTower }>(`/v1/towers/${templateId}/upgrade`);
}
