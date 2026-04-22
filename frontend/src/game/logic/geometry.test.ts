import { describe, it, expect } from 'vitest';
import {
  vec2Add,
  vec2Sub,
  vec2Scale,
  vec2Len,
  vec2Dist,
  tileCenter,
  pathLength,
  posAtProgress,
  distToSegment,
  nearPath,
  computePlacementTiles,
} from './geometry.ts';
import type { Vec2 } from './types.ts';

// ── vec2Add ───────────────────────────────────────────────────────────────────

describe('vec2Add', () => {
  it('adds two vectors', () => {
    expect(vec2Add({ x: 1, y: 2 }, { x: 3, y: 4 })).toEqual({ x: 4, y: 6 });
  });
  it('handles negative components', () => {
    expect(vec2Add({ x: -1, y: 5 }, { x: 3, y: -2 })).toEqual({ x: 2, y: 3 });
  });
});

// ── vec2Sub ───────────────────────────────────────────────────────────────────

describe('vec2Sub', () => {
  it('subtracts two vectors', () => {
    expect(vec2Sub({ x: 5, y: 7 }, { x: 2, y: 3 })).toEqual({ x: 3, y: 4 });
  });
});

// ── vec2Scale ─────────────────────────────────────────────────────────────────

describe('vec2Scale', () => {
  it('scales a vector', () => {
    expect(vec2Scale({ x: 3, y: 4 }, 2)).toEqual({ x: 6, y: 8 });
  });
  it('scales by zero', () => {
    expect(vec2Scale({ x: 3, y: 4 }, 0)).toEqual({ x: 0, y: 0 });
  });
  it('scales by negative', () => {
    expect(vec2Scale({ x: 2, y: -1 }, -3)).toEqual({ x: -6, y: 3 });
  });
});

// ── vec2Len ───────────────────────────────────────────────────────────────────

describe('vec2Len', () => {
  it('returns zero for zero vector', () => {
    expect(vec2Len({ x: 0, y: 0 })).toBe(0);
  });
  it('3-4-5 triangle', () => {
    expect(vec2Len({ x: 3, y: 4 })).toBeCloseTo(5);
  });
  it('unit x', () => {
    expect(vec2Len({ x: 1, y: 0 })).toBeCloseTo(1);
  });
});

// ── vec2Dist ──────────────────────────────────────────────────────────────────

describe('vec2Dist', () => {
  it('returns 5 for a 3-4-5 triangle', () => {
    expect(vec2Dist({ x: 0, y: 0 }, { x: 3, y: 4 })).toBeCloseTo(5);
  });
  it('is symmetric', () => {
    const a: Vec2 = { x: 1, y: 2 };
    const b: Vec2 = { x: 4, y: 6 };
    expect(vec2Dist(a, b)).toBeCloseTo(vec2Dist(b, a));
  });
  it('returns 0 for the same point', () => {
    expect(vec2Dist({ x: 7, y: 3 }, { x: 7, y: 3 })).toBe(0);
  });
});

// ── tileCenter ────────────────────────────────────────────────────────────────

describe('tileCenter', () => {
  it('returns (0.5, 0.5) for tile (0, 0)', () => {
    expect(tileCenter({ col: 0, row: 0 })).toEqual({ x: 0.5, y: 0.5 });
  });
  it('returns (2.5, 3.5) for tile (2, 3)', () => {
    expect(tileCenter({ col: 2, row: 3 })).toEqual({ x: 2.5, y: 3.5 });
  });
  it('returns (9.5, 4.5) for tile (9, 4)', () => {
    expect(tileCenter({ col: 9, row: 4 })).toEqual({ x: 9.5, y: 4.5 });
  });
});

// ── pathLength ────────────────────────────────────────────────────────────────

describe('pathLength', () => {
  it('returns 0 for empty path', () => {
    expect(pathLength([])).toBe(0);
  });
  it('returns 0 for a single point', () => {
    expect(pathLength([{ x: 1, y: 2 }])).toBe(0);
  });
  it('returns segment length for two points', () => {
    expect(pathLength([{ x: 0, y: 0 }, { x: 5, y: 0 }])).toBeCloseTo(5);
  });
  it('L-shaped path (3 + 4 = 7)', () => {
    expect(
      pathLength([{ x: 0, y: 0 }, { x: 3, y: 0 }, { x: 3, y: 4 }]),
    ).toBeCloseTo(7);
  });
  it('3-4-5 hypotenuse', () => {
    expect(pathLength([{ x: 0, y: 0 }, { x: 3, y: 4 }])).toBeCloseTo(5);
  });
});

// ── posAtProgress ─────────────────────────────────────────────────────────────

describe('posAtProgress', () => {
  it('returns start for d = 0', () => {
    expect(posAtProgress([{ x: 1, y: 2 }, { x: 4, y: 2 }], 0)).toEqual({
      x: 1,
      y: 2,
    });
  });
  it('returns end for d = pathLength', () => {
    const pts: Vec2[] = [{ x: 0, y: 0 }, { x: 6, y: 0 }];
    expect(posAtProgress(pts, 6)).toEqual({ x: 6, y: 0 });
  });
  it('clamps d > pathLength to last waypoint', () => {
    const pts: Vec2[] = [{ x: 0, y: 0 }, { x: 4, y: 0 }];
    expect(posAtProgress(pts, 100)).toEqual({ x: 4, y: 0 });
  });
  it('clamps negative d to first waypoint', () => {
    const pts: Vec2[] = [{ x: 2, y: 3 }, { x: 5, y: 3 }];
    expect(posAtProgress(pts, -5)).toEqual({ x: 2, y: 3 });
  });
  it('returns midpoint for d = half length', () => {
    const pts: Vec2[] = [{ x: 0, y: 0 }, { x: 10, y: 0 }];
    const mid = posAtProgress(pts, 5);
    expect(mid.x).toBeCloseTo(5);
    expect(mid.y).toBeCloseTo(0);
  });
  it('handles multi-segment path correctly', () => {
    // L-shape: (0,0)->(4,0)->(4,3), total length 7
    const pts: Vec2[] = [{ x: 0, y: 0 }, { x: 4, y: 0 }, { x: 4, y: 3 }];
    // At d=4, we should be at the corner (4,0)
    const corner = posAtProgress(pts, 4);
    expect(corner.x).toBeCloseTo(4);
    expect(corner.y).toBeCloseTo(0);
    // At d=4+1.5=5.5, we should be at (4, 1.5)
    const mid = posAtProgress(pts, 5.5);
    expect(mid.x).toBeCloseTo(4);
    expect(mid.y).toBeCloseTo(1.5);
  });
  it('returns zero for empty waypoints', () => {
    expect(posAtProgress([], 5)).toEqual({ x: 0, y: 0 });
  });
  it('returns single waypoint regardless of d', () => {
    expect(posAtProgress([{ x: 3, y: 7 }], 100)).toEqual({ x: 3, y: 7 });
  });
});

// ── distToSegment ─────────────────────────────────────────────────────────────

describe('distToSegment', () => {
  it('returns 0 when point is on segment', () => {
    expect(
      distToSegment({ x: 0, y: 0 }, { x: 10, y: 0 }, { x: 5, y: 0 }),
    ).toBeCloseTo(0);
  });
  it('returns perpendicular distance when point projects inside segment', () => {
    expect(
      distToSegment({ x: 0, y: 0 }, { x: 10, y: 0 }, { x: 5, y: 3 }),
    ).toBeCloseTo(3);
  });
  it('returns distance to start for point before segment', () => {
    expect(
      distToSegment({ x: 2, y: 0 }, { x: 8, y: 0 }, { x: 0, y: 0 }),
    ).toBeCloseTo(2);
  });
  it('returns distance to end for point after segment', () => {
    expect(
      distToSegment({ x: 0, y: 0 }, { x: 4, y: 0 }, { x: 7, y: 0 }),
    ).toBeCloseTo(3);
  });
  it('handles degenerate segment (a === b)', () => {
    expect(
      distToSegment({ x: 5, y: 5 }, { x: 5, y: 5 }, { x: 8, y: 9 }),
    ).toBeCloseTo(5); // distance from (5,5) to (8,9) = 5
  });
});

// ── nearPath ──────────────────────────────────────────────────────────────────

describe('nearPath', () => {
  const pts: Vec2[] = [{ x: 0, y: 0 }, { x: 10, y: 0 }];
  it('returns true when point is on path', () => {
    expect(nearPath(pts, { x: 5, y: 0 }, 1)).toBe(true);
  });
  it('returns true when point is within radius', () => {
    expect(nearPath(pts, { x: 5, y: 0.8 }, 1)).toBe(true);
  });
  it('returns false when point is beyond radius', () => {
    expect(nearPath(pts, { x: 5, y: 2 }, 1)).toBe(false);
  });
  it('returns false for empty waypoints', () => {
    expect(nearPath([], { x: 0, y: 0 }, 1)).toBe(false);
  });
});

// ── computePlacementTiles ─────────────────────────────────────────────────────

describe('computePlacementTiles', () => {
  it('returns all tiles when path is empty', () => {
    const tiles = computePlacementTiles(2, 2, [], 0.75);
    expect(tiles).toHaveLength(4);
  });
  it('excludes tiles too close to path', () => {
    // Horizontal path along y=2.5
    const waypoints: Vec2[] = [{ x: 0, y: 2.5 }, { x: 10, y: 2.5 }];
    const tiles = computePlacementTiles(10, 5, waypoints, 0.75);
    // Tiles with row 2 have centres at y=2.5, which are on the path — excluded
    // Tiles with row 1 have centres at y=1.5, distance to path = 1.0 > 0.75 — included
    const rowTwoTiles = tiles.filter((t) => t.row === 2);
    expect(rowTwoTiles).toHaveLength(0);
  });
  it('includes tiles far from path', () => {
    const waypoints: Vec2[] = [{ x: 0, y: 0 }, { x: 3, y: 0 }];
    const tiles = computePlacementTiles(4, 4, waypoints, 0.75);
    // Row 1 and below have centres y >= 1.5, distance > 0.75 from y=0
    const farTiles = tiles.filter((t) => t.row >= 2);
    expect(farTiles.length).toBeGreaterThan(0);
  });
});
