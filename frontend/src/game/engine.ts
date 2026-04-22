/**
 * Pixi.js Application wrapper that owns the ticker, drives the simulation,
 * and delegates rendering to specialised renderers.
 *
 * Usage:
 *   const engine = new GameEngine();
 *   await engine.mount(divElement);
 *   engine.startGame('alpha', 200, 10);
 *   // later:
 *   engine.unmount();
 */

import { Application } from 'pixi.js';
import { step, initialState, isGameOver, isVictory } from './logic/sim.ts';
import { lookupMap } from './logic/mapdata.ts';
import { tileCenter, posAtProgress } from './logic/geometry.ts';
import type {
  GameState,
  PlaceTowerInput,
  Tower,
  Tile,
  GameMap,
  Wave,
} from './logic/types.ts';
import { MapRenderer, TILE_SIZE, pixelToTile } from './render/mapRenderer.ts';
import { MonsterRenderer } from './render/monsterRenderer.ts';
import { TowerRenderer } from './render/towerRenderer.ts';
import { ProjectileRenderer } from './render/projectileRenderer.ts';

export type GameOverReason = 'victory' | 'defeat';

export interface EngineCallbacks {
  /** Called when the simulation state changes (every tick). */
  onStateChange?: (state: Readonly<GameState>) => void;
  /** Called when the game ends. */
  onGameOver?: (reason: GameOverReason) => void;
  /**
   * Called when the player successfully places a tower.
   * Useful so the UI can queue a backend update.
   */
  onTowerPlaced?: (input: PlaceTowerInput) => void;
}

export class GameEngine {
  private _app: Application | null = null;
  private _state: GameState | null = null;
  private _pendingInputs: PlaceTowerInput[] = [];
  private _paused = false;
  private _callbacks: EngineCallbacks = {};

  // Renderers
  private _mapRenderer: MapRenderer | null = null;
  private _monsterRenderer: MonsterRenderer | null = null;
  private _towerRenderer: TowerRenderer | null = null;
  private _projectileRenderer: ProjectileRenderer | null = null;

  // Drag-and-drop state
  private _selectedTemplate: {
    id: string;
    damage: number;
    range: number;
    rate: number;
    goldCost: number;
  } | null = null;

  /** Mounts Pixi onto the given element and starts the render loop. */
  async mount(el: HTMLElement, callbacks: EngineCallbacks = {}): Promise<void> {
    this._callbacks = callbacks;

    const app = new Application();
    await app.init({
      width: el.clientWidth || 896,
      height: el.clientHeight || 576,
      backgroundColor: 0x1a1a2e,
      antialias: true,
      resolution: window.devicePixelRatio || 1,
      autoDensity: true,
    });
    el.appendChild(app.canvas);
    this._app = app;

    // Register the per-frame update.
    app.ticker.add(this._onTick.bind(this));

    // Pointer events for drag-and-drop.
    app.stage.eventMode = 'static';
    app.stage.hitArea = app.screen;
    app.stage.on('pointermove', this._onPointerMove.bind(this));
    app.stage.on('pointerup', this._onPointerUp.bind(this));
  }

  /** Tears down Pixi and all renderers. Must be called on React unmount. */
  unmount(): void {
    this._destroyRenderers();
    if (this._app) {
      this._app.destroy(true, { children: true, texture: true });
      this._app = null;
    }
    this._state = null;
    this._pendingInputs = [];
    this._selectedTemplate = null;
    this._callbacks = {};
  }

  /** Starts a new game on the given map. */
  startGame(mapId: string, gold: number, gateHp: number): void {
    const data = lookupMap(mapId);
    if (!data) {
      // Unknown map id — silently skip; callers should validate map ids.
      return;
    }
    this._startWithMapData(data.map, data.waves, gold, gateHp);
  }

  /** Pauses/resumes the simulation tick (rendering continues). */
  pause(): void {
    this._paused = true;
  }
  resume(): void {
    this._paused = false;
  }
  get isPaused(): boolean {
    return this._paused;
  }

  /** Read-only view of the current simulation state. */
  get state(): Readonly<GameState> | null {
    return this._state;
  }

  /**
   * Sets the tower template to place when the player taps a tile.
   * Pass null to cancel placement mode.
   */
  setSelectedTemplate(
    template: {
      id: string;
      damage: number;
      range: number;
      rate: number;
      goldCost: number;
    } | null,
  ): void {
    this._selectedTemplate = template;
    if (!template) {
      this._towerRenderer?.hideGhost();
    }
  }

  // ── private — tick ─────────────────────────────────────────────────────────

  private _onTick(ticker: { deltaMS: number }): void {
    if (!this._state || this._paused) return;

    const dt = ticker.deltaMS / 1000;

    // Record towers before step so we can detect new attacks.
    const prevTowers = this._state.towers;

    const input = {
      placeTowers: this._pendingInputs.splice(0),
    };
    const nextState = step(this._state, input, dt);

    // Spawn projectiles for any attack that fired this tick.
    this._spawnProjectiles(prevTowers, nextState);

    this._state = nextState;

    // Update renderers.
    if (this._state.map && this._monsterRenderer) {
      this._monsterRenderer.update(this._state.monsters, this._state.map.waypoints);
    }
    if (this._towerRenderer) {
      this._towerRenderer.update(this._state.towers);
    }
    if (this._projectileRenderer) {
      this._projectileRenderer.update(dt);
    }

    this._callbacks.onStateChange?.(this._state);

    // Check game over conditions.
    if (isGameOver(this._state)) {
      this._callbacks.onGameOver?.('defeat');
      this._paused = true;
    } else if (isVictory(this._state)) {
      this._callbacks.onGameOver?.('victory');
      this._paused = true;
    }
  }

  // ── private — rendering setup ──────────────────────────────────────────────

  private _startWithMapData(map: GameMap, waves: Wave[], gold: number, gateHp: number): void {
    if (!this._app) return;

    this._destroyRenderers();
    this._state = initialState(map, waves, gold, gateHp);
    this._paused = false;

    const stage = this._app.stage;

    // Map layer (static, drawn once).
    this._mapRenderer = new MapRenderer();
    this._mapRenderer.draw(map);
    stage.addChild(this._mapRenderer.container);

    // Tower layer.
    this._towerRenderer = new TowerRenderer();
    stage.addChild(this._towerRenderer.container);

    // Monster layer.
    this._monsterRenderer = new MonsterRenderer();
    stage.addChild(this._monsterRenderer.container);

    // Projectile layer.
    this._projectileRenderer = new ProjectileRenderer();
    stage.addChild(this._projectileRenderer.container);

    // Resize canvas to fit the map.
    this._app.renderer.resize(map.cols * TILE_SIZE, map.rows * TILE_SIZE);

    this._callbacks.onStateChange?.(this._state);
  }

  private _destroyRenderers(): void {
    this._mapRenderer?.destroy();
    this._monsterRenderer?.destroy();
    this._towerRenderer?.destroy();
    this._projectileRenderer?.destroy();
    this._mapRenderer = null;
    this._monsterRenderer = null;
    this._towerRenderer = null;
    this._projectileRenderer = null;
    if (this._app) {
      // Remove all children from stage before adding fresh renderers.
      this._app.stage.removeChildren();
    }
  }

  // ── private — input handling ───────────────────────────────────────────────

  private _onPointerMove(e: { global: { x: number; y: number } }): void {
    if (!this._selectedTemplate || !this._state || !this._towerRenderer) return;

    const tile = pixelToTile(e.global.x, e.global.y);
    const isValid = this._isTileValid(tile);
    this._towerRenderer.showGhost(tile, isValid, this._selectedTemplate.range);
  }

  private _onPointerUp(e: { global: { x: number; y: number } }): void {
    if (!this._selectedTemplate || !this._state) return;

    const tile = pixelToTile(e.global.x, e.global.y);
    if (!this._isTileValid(tile)) return;

    const input: PlaceTowerInput = {
      templateId: this._selectedTemplate.id,
      tile,
      damage: this._selectedTemplate.damage,
      range: this._selectedTemplate.range,
      rate: this._selectedTemplate.rate,
      goldCost: this._selectedTemplate.goldCost,
    };

    // Check gold before queuing (optimistic check; sim validates authoritatively).
    if (this._state.gold < input.goldCost) return;

    this._pendingInputs.push(input);
    this._callbacks.onTowerPlaced?.(input);
    // Deselect after placement.
    this._selectedTemplate = null;
    this._towerRenderer?.hideGhost();
  }

  private _isTileValid(tile: Tile): boolean {
    if (!this._state) return false;
    const onMap =
      tile.col >= 0 &&
      tile.row >= 0 &&
      tile.col < this._state.map.cols &&
      tile.row < this._state.map.rows;
    if (!onMap) return false;
    const isPlaceable = this._state.map.tiles.some(
      (t) => t.col === tile.col && t.row === tile.row,
    );
    const isOccupied = this._state.towers.some(
      (t) => t.tile.col === tile.col && t.tile.row === tile.row,
    );
    return isPlaceable && !isOccupied;
  }

  // ── private — projectile spawning ─────────────────────────────────────────

  private _spawnProjectiles(prevTowers: Tower[], nextState: GameState): void {
    if (!this._projectileRenderer || !this._state) return;

    for (let i = 0; i < nextState.towers.length; i++) {
      const nextTower = nextState.towers[i];
      const prevTower = prevTowers.find((t) => t.id === nextTower.id);
      if (!prevTower) continue;

      // A tower fired if its cooldown reset (went from ~0 to 1/rate).
      const justFired = prevTower.cooldown <= 0 && nextTower.cooldown > 0;
      if (!justFired) continue;

      // Find the closest alive monster in the next state (approximate target).
      const towerPos = tileCenter(nextTower.tile);
      const spx = towerPos.x * TILE_SIZE;
      const spy = towerPos.y * TILE_SIZE;

      const target = nextState.monsters
        .filter((m) => m.alive)
        .sort((a, b) => b.progress - a.progress)[0]; // first strategy

      if (!target) continue;
      const tp = posAtProgress(nextState.map.waypoints, target.progress);
      this._projectileRenderer.spawn(spx, spy, tp.x * TILE_SIZE, tp.y * TILE_SIZE);
    }
  }
}
