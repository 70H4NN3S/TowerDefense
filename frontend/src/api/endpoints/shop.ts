import { api } from '../client.ts';
import type { CatalogTower, OwnedTower } from '../types.ts';

/** Fetch the full tower catalog with an `owned` flag per entry. */
export function getCatalog(): Promise<{ towers: CatalogTower[] }> {
  return api.get<{ towers: CatalogTower[] }>('/v1/shop/towers');
}

/** Purchase a tower template for its diamond cost. */
export function buyTower(templateId: string): Promise<{ tower: OwnedTower }> {
  return api.post<{ tower: OwnedTower }>(`/v1/shop/towers/${templateId}/buy`);
}
