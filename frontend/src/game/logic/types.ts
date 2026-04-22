/**
 * TypeScript equivalents of the Go sim types in backend/internal/game/sim/.
 * These are pure value types with no I/O, no Pixi, and no React.
 */

/** Two-dimensional world-space position. One unit equals one tile width. */
export interface Vec2 {
  x: number;
  y: number;
}

/** Grid cell (zero-based col, row) where a tower may be placed. */
export interface Tile {
  col: number;
  row: number;
}

/** Static layout of a level: path, gate, and placeable tiles. */
export interface GameMap {
  id: string;
  cols: number;
  rows: number;
  /** Ordered waypoints; monsters travel [0] → [len-1]. */
  waypoints: Vec2[];
  /** World-space centre of the gate. */
  gate: Vec2;
  /** Valid tower-placement positions. */
  tiles: Tile[];
}

/** Live state of one enemy unit. */
export interface Monster {
  id: number;
  maxHp: number;
  hp: number;
  /** World units per second. */
  speed: number;
  /** Gold awarded on kill. */
  reward: number;
  /** Total distance travelled along the path. */
  progress: number;
  alive: boolean;
}

/** Live state of one defence structure. */
export interface Tower {
  id: number;
  templateId: string;
  tile: Tile;
  damage: number;
  /** World units. */
  range: number;
  /** Attacks per second (must be > 0). */
  rate: number;
  /** Seconds until next attack (0 = ready). */
  cooldown: number;
}

/** Batch of identical monsters within a wave. */
export interface SpawnGroup {
  maxHp: number;
  speed: number;
  reward: number;
  count: number;
  /** Seconds between spawns within the group. */
  interval: number;
  /** Seconds after wave start before first spawn. */
  delay: number;
}

/** Spawn schedule for one enemy wave. */
export interface Wave {
  number: number;
  groups: SpawnGroup[];
}

/** Player command to build a tower. */
export interface PlaceTowerInput {
  templateId: string;
  tile: Tile;
  damage: number;
  range: number;
  rate: number;
  goldCost: number;
}

/** All player actions for one simulation tick. */
export interface SimInput {
  placeTowers: PlaceTowerInput[];
}

/** Per-group spawn progress within the current wave. */
export interface SpawnRecord {
  groupIdx: number;
  spawned: number;
  /** Wave time at which the next spawn fires. */
  nextSpawnT: number;
}

/** Complete serialisable snapshot of an in-progress match. */
export interface GameState {
  map: GameMap;
  towers: Tower[];
  monsters: Monster[];
  waves: Wave[];
  waveIdx: number;
  waveTime: number;
  spawnRecords: SpawnRecord[];
  gold: number;
  gateHp: number;
  tick: number;
  nextId: number;
}

/** Targeting strategy for tower attacks. */
export type TargetStrategy = 'first' | 'strongest' | 'closest';
