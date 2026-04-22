import { describe, it, expect } from 'vitest';
import { pickTarget } from './targeting.ts';
import type { Tower, Monster, Vec2 } from './types.ts';

// A straight horizontal path of length 10 for easy distance math.
const waypoints: Vec2[] = [{ x: 0, y: 0 }, { x: 10, y: 0 }];

function makeTower(overrides: Partial<Tower> = {}): Tower {
  return {
    id: 1,
    templateId: 'basic',
    tile: { col: 0, row: 1 }, // world-space centre (0.5, 1.5), range 5
    damage: 10,
    range: 5,
    rate: 1,
    cooldown: 0,
    ...overrides,
  };
}

function makeMonster(id: number, progress: number, hp: number, alive = true): Monster {
  return {
    id,
    maxHp: 100,
    hp,
    speed: 1,
    reward: 10,
    progress,
    alive,
  };
}

describe('pickTarget — no monsters', () => {
  it('returns null when monster list is empty', () => {
    expect(pickTarget(makeTower(), [], waypoints, 'first')).toBeNull();
  });
});

describe('pickTarget — range filtering', () => {
  it('returns null when all monsters are out of range', () => {
    // Tower at tile (0,1): centre (0.5, 1.5), range 5
    // Monster at progress 10 → pos (10, 0), dist ≈ 9.6 > 5
    const monsters = [makeMonster(1, 10, 100)];
    expect(pickTarget(makeTower(), monsters, waypoints, 'first')).toBeNull();
  });
  it('returns a monster that is within range', () => {
    // Monster at progress 1 → pos (1, 0), dist from (0.5,1.5) ≈ 1.58 < 5
    const monsters = [makeMonster(1, 1, 100)];
    expect(pickTarget(makeTower(), monsters, waypoints, 'first')).toBe(
      monsters[0],
    );
  });
  it('ignores dead monsters', () => {
    const monsters = [makeMonster(1, 1, 100, false)];
    expect(pickTarget(makeTower(), monsters, waypoints, 'first')).toBeNull();
  });
});

describe('pickTarget — first strategy', () => {
  it('picks the monster with the highest progress', () => {
    const m1 = makeMonster(1, 1, 100);
    const m2 = makeMonster(2, 3, 100); // further along
    const m3 = makeMonster(3, 2, 100);
    // All in range (progress ≤ 5 keeps distance roughly ≤ range 5)
    const tower = makeTower({ range: 10 });
    expect(pickTarget(tower, [m1, m2, m3], waypoints, 'first')).toBe(m2);
  });
});

describe('pickTarget — strongest strategy', () => {
  it('picks the monster with the most HP', () => {
    const m1 = makeMonster(1, 1, 40);
    const m2 = makeMonster(2, 2, 90); // highest HP
    const m3 = makeMonster(3, 3, 60);
    const tower = makeTower({ range: 10 });
    expect(pickTarget(tower, [m1, m2, m3], waypoints, 'strongest')).toBe(m2);
  });
});

describe('pickTarget — closest strategy', () => {
  it('picks the monster closest to the tower', () => {
    // Tower at tile (5,0): centre (5.5, 0.5), range 10
    const tower = makeTower({ tile: { col: 5, row: 0 }, range: 10 });
    const m1 = makeMonster(1, 2, 100); // pos (2, 0), dist from (5.5,0.5) ≈ 3.5
    const m2 = makeMonster(2, 5, 100); // pos (5, 0), dist from (5.5,0.5) ≈ 0.7  ← closest
    const m3 = makeMonster(3, 8, 100); // pos (8, 0), dist from (5.5,0.5) ≈ 2.6
    expect(pickTarget(tower, [m1, m2, m3], waypoints, 'closest')).toBe(m2);
  });
});

describe('pickTarget — mixed alive/dead', () => {
  it('only considers alive monsters', () => {
    const tower = makeTower({ range: 10 });
    const m1 = makeMonster(1, 3, 100, false); // dead, would be first
    const m2 = makeMonster(2, 1, 100, true);
    expect(pickTarget(tower, [m1, m2], waypoints, 'first')).toBe(m2);
  });
});
