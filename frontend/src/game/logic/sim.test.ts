import { describe, it, expect } from 'vitest';
import { step, initialState, isGameOver, isVictory } from './sim.ts';
import type { GameState, GameMap, Wave, SimInput } from './types.ts';

// ── fixtures ──────────────────────────────────────────────────────────────────

const testMap: GameMap = {
  id: 'test',
  cols: 10,
  rows: 5,
  waypoints: [{ x: 0, y: 0 }, { x: 10, y: 0 }], // straight, length 10
  gate: { x: 10, y: 0 },
  tiles: [
    // A few placeable tiles away from the path (y=0 row)
    { col: 0, row: 2 },
    { col: 1, row: 2 },
    { col: 2, row: 2 },
  ],
};

const singleGroupWave: Wave = {
  number: 1,
  groups: [{ maxHp: 50, speed: 1, reward: 10, count: 2, interval: 2, delay: 0 }],
};

function makeState(overrides: Partial<GameState> = {}): GameState {
  return initialState(testMap, [singleGroupWave], 200, 10, ...[] as never[]);
}

// Helper that calls initialState with the test fixtures.
function freshState(): GameState {
  return initialState(testMap, [singleGroupWave], 200, 10);
}

const emptyInput: SimInput = { placeTowers: [] };

// ── initialState ──────────────────────────────────────────────────────────────

describe('initialState', () => {
  it('sets gold and gateHp', () => {
    const s = freshState();
    expect(s.gold).toBe(200);
    expect(s.gateHp).toBe(10);
  });
  it('starts with no towers or monsters', () => {
    const s = freshState();
    expect(s.towers).toHaveLength(0);
    expect(s.monsters).toHaveLength(0);
  });
  it('initialises spawn records for wave 0', () => {
    const s = freshState();
    expect(s.spawnRecords).toHaveLength(1);
    expect(s.spawnRecords[0].spawned).toBe(0);
  });
});

// ── step — spawning ───────────────────────────────────────────────────────────

describe('step — spawning', () => {
  it('spawns the first monster at delay=0 after a tiny tick', () => {
    const s = freshState();
    const s1 = step(s, emptyInput, 0.01);
    expect(s1.monsters.filter((m) => m.alive)).toHaveLength(1);
  });
  it('spawns the second monster after interval elapses', () => {
    let s = freshState();
    s = step(s, emptyInput, 0.01); // spawn first
    s = step(s, emptyInput, 2.0); // interval = 2s
    expect(s.monsters.filter((m) => m.alive)).toHaveLength(2);
  });
  it('does not spawn past group count', () => {
    let s = freshState();
    for (let i = 0; i < 20; i++) {
      s = step(s, emptyInput, 1.0);
    }
    // Group has count = 2; never exceed that.
    const everSpawned = s.monsters.length;
    expect(everSpawned).toBeLessThanOrEqual(2);
  });
});

// ── step — movement ───────────────────────────────────────────────────────────

describe('step — movement', () => {
  it('advances monster progress by speed * dt', () => {
    let s = freshState();
    s = step(s, emptyInput, 0.01); // spawn
    const before = s.monsters[0].progress;
    s = step(s, emptyInput, 1.0); // move 1 world-unit (speed=1)
    const after = s.monsters[0].progress;
    expect(after - before).toBeCloseTo(1.0, 2);
  });
  it('kills a monster and damages gate when it reaches the end', () => {
    let s = freshState();
    // Advance far enough for monster to reach progress=10 (end of path).
    s = step(s, emptyInput, 0.01); // spawn
    s = step(s, emptyInput, 15.0); // way past the end (length=10)
    const dead = s.monsters.find((m) => !m.alive);
    expect(dead).toBeDefined();
    expect(s.gateHp).toBeLessThan(10);
  });
});

// ── step — tower placement ────────────────────────────────────────────────────

describe('step — tower placement', () => {
  it('places a tower and deducts gold', () => {
    const s = freshState();
    const input: SimInput = {
      placeTowers: [
        {
          templateId: 'basic',
          tile: { col: 0, row: 2 },
          damage: 10,
          range: 5,
          rate: 1,
          goldCost: 100,
        },
      ],
    };
    const s1 = step(s, input, 0);
    expect(s1.towers).toHaveLength(1);
    expect(s1.gold).toBe(100);
  });
  it('rejects tower on invalid tile', () => {
    const s = freshState();
    const input: SimInput = {
      placeTowers: [
        {
          templateId: 'basic',
          tile: { col: 5, row: 0 }, // on the path
          damage: 10,
          range: 5,
          rate: 1,
          goldCost: 50,
        },
      ],
    };
    const s1 = step(s, input, 0);
    expect(s1.towers).toHaveLength(0);
    expect(s1.gold).toBe(200); // no deduction
  });
  it('rejects tower when insufficient gold', () => {
    const s = freshState();
    const input: SimInput = {
      placeTowers: [
        {
          templateId: 'basic',
          tile: { col: 0, row: 2 },
          damage: 10,
          range: 5,
          rate: 1,
          goldCost: 999,
        },
      ],
    };
    const s1 = step(s, input, 0);
    expect(s1.towers).toHaveLength(0);
  });
  it('rejects tower on occupied tile', () => {
    let s = freshState();
    const input: SimInput = {
      placeTowers: [
        {
          templateId: 'basic',
          tile: { col: 0, row: 2 },
          damage: 10,
          range: 5,
          rate: 1,
          goldCost: 50,
        },
      ],
    };
    s = step(s, input, 0); // place first tower
    s = step(s, input, 0); // try again — should be rejected
    expect(s.towers).toHaveLength(1);
  });
});

// ── step — combat ─────────────────────────────────────────────────────────────

describe('step — combat', () => {
  it('tower kills a monster in range and awards gold', () => {
    let s = freshState();
    // Place a powerful tower (cost 100, gold = 100 after placement).
    // With range 15 and damage 999 it one-shots every monster.
    const input: SimInput = {
      placeTowers: [
        {
          templateId: 'basic',
          tile: { col: 0, row: 2 },
          damage: 999,
          range: 15,
          rate: 10,
          goldCost: 100,
        },
      ],
    };
    s = step(s, input, 0); // place tower
    // Run several ticks — monsters spawn and get killed, rewarding gold.
    for (let i = 0; i < 20; i++) {
      s = step(s, emptyInput, 0.5);
    }
    const killed = s.monsters.find((m) => !m.alive);
    expect(killed).toBeDefined();
    // After killing monsters (reward=10 each) gold should exceed post-placement gold (100).
    expect(s.gold).toBeGreaterThan(100);
  });
});

// ── step — wave advance ───────────────────────────────────────────────────────

describe('step — wave advance', () => {
  it('advances to the next wave after all monsters are cleared', () => {
    let s = freshState();
    // Place a powerful tower that will kill everything.
    const input: SimInput = {
      placeTowers: [
        {
          templateId: 'basic',
          tile: { col: 0, row: 2 },
          damage: 9999,
          range: 20,
          rate: 100,
          goldCost: 100,
        },
      ],
    };
    s = step(s, input, 0);
    // Run many ticks to spawn and kill all monsters.
    for (let i = 0; i < 100; i++) {
      s = step(s, emptyInput, 0.1);
    }
    // Wave 0 should be complete; no waves follow in this test setup, so
    // waveIdx should be >= 1.
    expect(s.waveIdx).toBeGreaterThanOrEqual(1);
  });
});

// ── isGameOver / isVictory ────────────────────────────────────────────────────

describe('isGameOver', () => {
  it('returns false when gate has hp', () => {
    expect(isGameOver(freshState())).toBe(false);
  });
  it('returns true when gate is destroyed', () => {
    const s = { ...freshState(), gateHp: 0 };
    expect(isGameOver(s)).toBe(true);
  });
});

describe('isVictory', () => {
  it('returns false during an active game', () => {
    expect(isVictory(freshState())).toBe(false);
  });
  it('returns true when all waves cleared and no alive monsters', () => {
    const s: GameState = {
      ...freshState(),
      waveIdx: 1, // past all waves (only 1 wave in test fixture)
      monsters: [{ id: 1, maxHp: 50, hp: 0, speed: 1, reward: 10, progress: 0, alive: false }],
    };
    expect(isVictory(s)).toBe(true);
  });
  it('returns false when gate is destroyed', () => {
    const s: GameState = {
      ...freshState(),
      waveIdx: 1,
      monsters: [],
      gateHp: 0,
    };
    expect(isVictory(s)).toBe(false);
  });
});

// ── immutability ──────────────────────────────────────────────────────────────

describe('step — immutability', () => {
  it('does not mutate the input state', () => {
    const s = freshState();
    const origGold = s.gold;
    const origTick = s.tick;
    step(s, emptyInput, 1.0);
    expect(s.gold).toBe(origGold);
    expect(s.tick).toBe(origTick);
  });
});
