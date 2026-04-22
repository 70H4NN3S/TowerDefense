/**
 * Pure geometry helpers for path traversal and tile layout.
 * No I/O, no Pixi, no React — safe to test in Node/Vitest.
 *
 * Mirrors the functions in backend/internal/game/sim/geometry.go.
 */

import type { Vec2, Tile } from './types.ts';

/** Returns v + w. */
export function vec2Add(v: Vec2, w: Vec2): Vec2 {
  return { x: v.x + w.x, y: v.y + w.y };
}

/** Returns v - w. */
export function vec2Sub(v: Vec2, w: Vec2): Vec2 {
  return { x: v.x - w.x, y: v.y - w.y };
}

/** Returns v * s. */
export function vec2Scale(v: Vec2, s: number): Vec2 {
  return { x: v.x * s, y: v.y * s };
}

/** Returns the Euclidean length of v. */
export function vec2Len(v: Vec2): number {
  return Math.sqrt(v.x * v.x + v.y * v.y);
}

/** Returns the Euclidean distance from v to w. */
export function vec2Dist(v: Vec2, w: Vec2): number {
  return vec2Len(vec2Sub(v, w));
}

/**
 * Returns the world-space centre of tile t.
 * Each tile occupies a 1×1 cell at (col, row), so its centre is at
 * (col+0.5, row+0.5).
 */
export function tileCenter(t: Tile): Vec2 {
  return { x: t.col + 0.5, y: t.row + 0.5 };
}

/**
 * Returns the total arc-length of the polyline defined by waypoints.
 * Returns 0 for fewer than two waypoints.
 */
export function pathLength(waypoints: Vec2[]): number {
  let total = 0;
  for (let i = 1; i < waypoints.length; i++) {
    total += vec2Dist(waypoints[i - 1], waypoints[i]);
  }
  return total;
}

/**
 * Returns the world-space position at distance d along the polyline.
 * d is clamped to [0, pathLength(waypoints)].
 */
export function posAtProgress(waypoints: Vec2[], d: number): Vec2 {
  if (waypoints.length === 0) return { x: 0, y: 0 };
  if (waypoints.length === 1 || d <= 0) return waypoints[0];

  let remaining = d;
  for (let i = 1; i < waypoints.length; i++) {
    const seg = vec2Dist(waypoints[i - 1], waypoints[i]);
    if (remaining <= seg) {
      if (seg === 0) return waypoints[i - 1];
      const t = remaining / seg;
      return vec2Add(
        waypoints[i - 1],
        vec2Scale(vec2Sub(waypoints[i], waypoints[i - 1]), t),
      );
    }
    remaining -= seg;
  }
  // d ≥ pathLength: clamp to last waypoint.
  return waypoints[waypoints.length - 1];
}

/** Returns the shortest distance from point p to segment ab. */
export function distToSegment(a: Vec2, b: Vec2, p: Vec2): number {
  const ab = vec2Sub(b, a);
  const ap = vec2Sub(p, a);
  const lenSq = ab.x * ab.x + ab.y * ab.y;
  if (lenSq === 0) return vec2Len(ap);
  let t = (ap.x * ab.x + ap.y * ab.y) / lenSq;
  t = Math.max(0, Math.min(1, t));
  const proj = vec2Add(a, vec2Scale(ab, t));
  return vec2Dist(p, proj);
}

/**
 * Reports whether point p is within radius of any segment of the path.
 * Used to determine which tiles are valid placement positions.
 */
export function nearPath(waypoints: Vec2[], p: Vec2, radius: number): boolean {
  for (let i = 1; i < waypoints.length; i++) {
    if (distToSegment(waypoints[i - 1], waypoints[i], p) <= radius) {
      return true;
    }
  }
  return false;
}

/**
 * Computes valid tower-placement tiles for a map.
 * A tile is valid if its centre is more than radius world-units from every
 * path segment.
 */
export function computePlacementTiles(
  cols: number,
  rows: number,
  waypoints: Vec2[],
  radius: number,
): Tile[] {
  const tiles: Tile[] = [];
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      const t: Tile = { col, row };
      if (!nearPath(waypoints, tileCenter(t), radius)) {
        tiles.push(t);
      }
    }
  }
  return tiles;
}
