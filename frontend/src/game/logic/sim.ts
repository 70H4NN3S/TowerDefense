/**
 * Client-side deterministic simulation step.
 * Mirrors backend/internal/game/sim/step.go.
 * Pure function: returns a new GameState without mutating its input.
 */

import type {
  GameState,
  SimInput,
  Tower,
  SpawnRecord,
  Wave,
} from './types.ts';
import { tileCenter, vec2Dist, posAtProgress, pathLength } from './geometry.ts';
import { pickTarget } from './targeting.ts';

const SPAWN_EPSILON = 1e-6;

/** Advances the simulation by dt seconds given player input. */
export function step(s: GameState, input: SimInput, dt: number): GameState {
  s = deepCopy(s);
  s = processInputs(s, input);
  s.waveTime += dt;
  s = spawnMonsters(s);
  s = moveMonsters(s, dt);
  s = towerAttacks(s, dt);
  s = advanceWave(s);
  s.tick++;
  return s;
}

/** Returns true if the game is over (gate destroyed or all waves cleared). */
export function isGameOver(s: GameState): boolean {
  return s.gateHp <= 0;
}

/** Returns true if the player has won (all waves cleared, no alive monsters). */
export function isVictory(s: GameState): boolean {
  return (
    s.waveIdx >= s.waves.length &&
    s.monsters.every((m) => !m.alive) &&
    s.gateHp > 0
  );
}

/**
 * Constructs the initial state for a match.
 * The caller sets gold and gateHp to game-design values.
 */
export function initialState(
  map: GameState['map'],
  waves: GameState['waves'],
  gold: number,
  gateHp: number,
): GameState {
  return {
    map,
    towers: [],
    monsters: [],
    waves,
    waveIdx: 0,
    waveTime: 0,
    spawnRecords: waves.length > 0 ? initSpawnRecords(waves[0]) : [],
    gold,
    gateHp,
    tick: 0,
    nextId: 0,
  };
}

// ── sub-steps ─────────────────────────────────────────────────────────────────

function processInputs(s: GameState, input: SimInput): GameState {
  for (const pt of input.placeTowers) {
    if (pt.goldCost <= 0 || pt.goldCost > s.gold) continue;
    if (pt.rate <= 0 || pt.damage < 0 || pt.range <= 0) continue;
    if (!tileIsValid(s.map.tiles, pt.tile)) continue;
    if (tileIsOccupied(s.towers, pt.tile)) continue;
    s.gold -= pt.goldCost;
    s.nextId++;
    s.towers = [
      ...s.towers,
      {
        id: s.nextId,
        templateId: pt.templateId,
        tile: pt.tile,
        damage: pt.damage,
        range: pt.range,
        rate: pt.rate,
        cooldown: 0,
      },
    ];
  }
  return s;
}

function tileIsValid(
  validTiles: GameState['map']['tiles'],
  tile: { col: number; row: number },
): boolean {
  return validTiles.some((t) => t.col === tile.col && t.row === tile.row);
}

function tileIsOccupied(towers: Tower[], tile: { col: number; row: number }): boolean {
  return towers.some((t) => t.tile.col === tile.col && t.tile.row === tile.row);
}

function spawnMonsters(s: GameState): GameState {
  if (s.waveIdx >= s.waves.length) return s;
  const wave = s.waves[s.waveIdx];
  const recs = [...s.spawnRecords];
  const monsters = [...s.monsters];
  let nextId = s.nextId;

  for (let i = 0; i < recs.length; i++) {
    const rec = { ...recs[i] };
    const group = wave.groups[rec.groupIdx];
    while (
      rec.spawned < group.count &&
      s.waveTime + SPAWN_EPSILON >= rec.nextSpawnT
    ) {
      nextId++;
      monsters.push({
        id: nextId,
        maxHp: group.maxHp,
        hp: group.maxHp,
        speed: group.speed,
        reward: group.reward,
        progress: 0,
        alive: true,
      });
      rec.spawned++;
      rec.nextSpawnT += group.interval;
    }
    recs[i] = rec;
  }
  return { ...s, spawnRecords: recs, monsters, nextId };
}

function moveMonsters(s: GameState, dt: number): GameState {
  const total = pathLength(s.map.waypoints);
  const monsters = s.monsters.map((m) => {
    if (!m.alive) return m;
    const newProgress = m.progress + m.speed * dt;
    if (newProgress >= total) {
      // Reached the gate.
      return { ...m, alive: false, progress: total };
    }
    return { ...m, progress: newProgress };
  });

  // Compute gate damage: count newly-dead monsters that reached the gate.
  let gateDamage = 0;
  for (let i = 0; i < monsters.length; i++) {
    if (s.monsters[i].alive && !monsters[i].alive) {
      gateDamage++;
    }
  }

  return {
    ...s,
    monsters,
    gateHp: Math.max(0, s.gateHp - gateDamage),
  };
}

function towerAttacks(s: GameState, dt: number): GameState {
  const towers = s.towers.map((t) => ({ ...t }));
  const monsters = s.monsters.map((m) => ({ ...m }));
  let gold = s.gold;

  for (const tower of towers) {
    tower.cooldown = Math.max(0, tower.cooldown - dt);
    if (tower.cooldown > 0) continue;

    const target = pickTarget(tower, monsters, s.map.waypoints, 'first');
    if (!target) continue;

    // Find and damage the monster in our mutable copy.
    const idx = monsters.findIndex((m) => m.id === target.id);
    if (idx === -1) continue;
    const m = monsters[idx];
    m.hp -= tower.damage;
    if (m.hp <= 0) {
      m.hp = 0;
      m.alive = false;
      gold += m.reward;
    }
    tower.cooldown = 1 / tower.rate;
  }

  return { ...s, towers, monsters, gold };
}

function advanceWave(s: GameState): GameState {
  if (s.waveIdx >= s.waves.length) return s;
  const wave = s.waves[s.waveIdx];

  // All groups fully spawned?
  const allSpawned = s.spawnRecords.every(
    (rec) => rec.spawned >= wave.groups[rec.groupIdx].count,
  );
  if (!allSpawned) return s;

  // All spawned monsters dead?
  const anyAlive = s.monsters.some((m) => m.alive);
  if (anyAlive) return s;

  // Advance to the next wave.
  const nextIdx = s.waveIdx + 1;
  return {
    ...s,
    waveIdx: nextIdx,
    waveTime: 0,
    spawnRecords:
      nextIdx < s.waves.length ? initSpawnRecords(s.waves[nextIdx]) : [],
  };
}

// ── helpers ───────────────────────────────────────────────────────────────────

function initSpawnRecords(wave: Wave): SpawnRecord[] {
  return wave.groups.map((g, i) => ({
    groupIdx: i,
    spawned: 0,
    nextSpawnT: g.delay,
  }));
}

/** Deep copies a GameState so sub-steps can mutate freely. */
function deepCopy(s: GameState): GameState {
  return {
    ...s,
    map: {
      ...s.map,
      waypoints: [...s.map.waypoints],
      tiles: [...s.map.tiles],
    },
    towers: s.towers.map((t) => ({ ...t })),
    monsters: s.monsters.map((m) => ({ ...m })),
    waves: s.waves,
    spawnRecords: s.spawnRecords.map((r) => ({ ...r })),
  };
}

// Export tileCenter for use by the engine when computing tower world positions.
export { tileCenter, vec2Dist, posAtProgress };
