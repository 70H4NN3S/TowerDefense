/**
 * Attack targeting strategies: which monster a tower shoots at.
 * Pure functions, no I/O, no Pixi.
 */

import type { Monster, Tower, Vec2, TargetStrategy } from './types.ts';
import { tileCenter, vec2Dist, posAtProgress } from './geometry.ts';

/**
 * Returns the monster a tower should attack given a strategy, or null if
 * no monster is in range.
 */
export function pickTarget(
  tower: Tower,
  monsters: Monster[],
  waypoints: Vec2[],
  strategy: TargetStrategy,
): Monster | null {
  const towerPos = tileCenter(tower.tile);
  const inRange = monsters.filter(
    (m) =>
      m.alive &&
      vec2Dist(towerPos, posAtProgress(waypoints, m.progress)) <= tower.range,
  );
  if (inRange.length === 0) return null;

  switch (strategy) {
    case 'first':
      return pickFirst(inRange);
    case 'strongest':
      return pickStrongest(inRange);
    case 'closest':
      return pickClosest(inRange, towerPos, waypoints);
  }
}

/** Picks the monster furthest along the path (highest progress). */
function pickFirst(monsters: Monster[]): Monster {
  return monsters.reduce((best, m) => (m.progress > best.progress ? m : best));
}

/** Picks the monster with the most remaining HP. */
function pickStrongest(monsters: Monster[]): Monster {
  return monsters.reduce((best, m) => (m.hp > best.hp ? m : best));
}

/** Picks the monster closest to the tower's world-space position. */
function pickClosest(
  monsters: Monster[],
  towerPos: Vec2,
  waypoints: Vec2[],
): Monster {
  return monsters.reduce((best, m) => {
    const distBest = vec2Dist(
      towerPos,
      posAtProgress(waypoints, best.progress),
    );
    const distM = vec2Dist(towerPos, posAtProgress(waypoints, m.progress));
    return distM < distBest ? m : best;
  });
}
