/**
 * Built-in map definitions shared between client and server.
 * These mirror the maps registered in backend/internal/game/sim/maps.go.
 */

import type { GameMap, Wave } from './types.ts';
import { computePlacementTiles } from './geometry.ts';

// ── Map "alpha" ───────────────────────────────────────────────────────────────

const ALPHA_COLS = 14;
const ALPHA_ROWS = 9;

/**
 * Monster path for map "alpha".
 * Enters from the left at row 2, snakes right→down→left→down, then exits at
 * the bottom edge.
 */
const alphaWaypoints = [
  { x: 0, y: 2.5 },
  { x: 9.5, y: 2.5 },
  { x: 9.5, y: 6.5 },
  { x: 3.5, y: 6.5 },
  { x: 3.5, y: 9.0 },
];

function alphaWaves(): Wave[] {
  return [
    {
      number: 1,
      groups: [
        { maxHp: 50, speed: 2.0, reward: 10, count: 5, interval: 1.5, delay: 0 },
      ],
    },
    {
      number: 2,
      groups: [
        { maxHp: 60, speed: 2.5, reward: 12, count: 8, interval: 1.2, delay: 0 },
      ],
    },
    {
      number: 3,
      groups: [
        { maxHp: 40, speed: 4.0, reward: 15, count: 3, interval: 1.0, delay: 0 },
        { maxHp: 150, speed: 1.5, reward: 25, count: 3, interval: 2.0, delay: 4.0 },
      ],
    },
    {
      number: 4,
      groups: [
        { maxHp: 100, speed: 2.5, reward: 20, count: 10, interval: 1.0, delay: 0 },
      ],
    },
    {
      number: 5,
      groups: [
        { maxHp: 400, speed: 1.0, reward: 100, count: 2, interval: 5.0, delay: 0 },
        { maxHp: 80, speed: 3.0, reward: 20, count: 5, interval: 0.8, delay: 2.0 },
      ],
    },
  ];
}

function buildAlpha(): { map: GameMap; waves: Wave[] } {
  const map: GameMap = {
    id: 'alpha',
    cols: ALPHA_COLS,
    rows: ALPHA_ROWS,
    waypoints: alphaWaypoints,
    gate: { x: 3.5, y: 9.0 },
    tiles: computePlacementTiles(ALPHA_COLS, ALPHA_ROWS, alphaWaypoints, 0.75),
  };
  return { map, waves: alphaWaves() };
}

/** Registry of all built-in maps. */
const registry: Record<string, () => { map: GameMap; waves: Wave[] }> = {
  alpha: buildAlpha,
};

/**
 * Returns the map and wave schedule for the given id.
 * Returns null if the id is not registered.
 */
export function lookupMap(id: string): { map: GameMap; waves: Wave[] } | null {
  const fn = registry[id];
  return fn ? fn() : null;
}
