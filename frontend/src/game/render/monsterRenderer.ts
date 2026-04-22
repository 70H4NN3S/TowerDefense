/**
 * Renders live monster sprites and their HP bars.
 * Updates positions every frame from the simulation state.
 */

import { Graphics, Container } from 'pixi.js';
import type { Monster, Vec2 } from '../logic/types.ts';
import { posAtProgress } from '../logic/geometry.ts';
import { TILE_SIZE } from './mapRenderer.ts';

const MONSTER_RADIUS = TILE_SIZE * 0.28;
const HP_BAR_WIDTH = TILE_SIZE * 0.6;
const HP_BAR_HEIGHT = 5;
const HP_BAR_Y_OFFSET = -MONSTER_RADIUS - 10;

const FILL_FULL = 0x44cc44;
const FILL_MID = 0xcccc44;
const FILL_LOW = 0xcc4444;
const BOSS_COLOUR = 0x8844cc;
const NORMAL_COLOUR = 0xee7722;

/** A single rendered monster entry. */
interface MonsterSprite {
  id: number;
  body: Graphics;
  hpBar: Graphics;
  container: Container;
}

export class MonsterRenderer {
  readonly container: Container;
  private readonly _sprites = new Map<number, MonsterSprite>();

  constructor() {
    this.container = new Container();
  }

  /**
   * Synchronises rendered sprites with the current simulation state.
   * Creates new sprites, updates existing ones, and removes dead ones.
   */
  update(monsters: Monster[], waypoints: Vec2[]): void {
    const liveIds = new Set<number>();

    for (const m of monsters) {
      if (!m.alive) continue;
      liveIds.add(m.id);

      const pos = posAtProgress(waypoints, m.progress);
      const px = pos.x * TILE_SIZE;
      const py = pos.y * TILE_SIZE;

      let sprite = this._sprites.get(m.id);
      if (!sprite) {
        sprite = this._createSprite(m);
        this._sprites.set(m.id, sprite);
        this.container.addChild(sprite.container);
      }

      sprite.container.x = px;
      sprite.container.y = py;
      this._updateHpBar(sprite.hpBar, m.hp, m.maxHp);
    }

    // Remove sprites for monsters that are no longer alive.
    for (const [id, sprite] of this._sprites) {
      if (!liveIds.has(id)) {
        this.container.removeChild(sprite.container);
        sprite.container.destroy({ children: true });
        this._sprites.delete(id);
      }
    }
  }

  destroy(): void {
    this.container.destroy({ children: true });
    this._sprites.clear();
  }

  // ── private ─────────────────────────────────────────────────────────────────

  private _createSprite(m: Monster): MonsterSprite {
    const container = new Container();

    const isBoss = m.maxHp >= 200;
    const colour = isBoss ? BOSS_COLOUR : NORMAL_COLOUR;

    const body = new Graphics();
    body.circle(0, 0, MONSTER_RADIUS);
    body.fill({ color: colour });
    body.setStrokeStyle({ color: 0xffffff, width: 1.5 });
    body.circle(0, 0, MONSTER_RADIUS);
    body.stroke();

    const hpBarBg = new Graphics();
    hpBarBg.rect(-HP_BAR_WIDTH / 2, HP_BAR_Y_OFFSET, HP_BAR_WIDTH, HP_BAR_HEIGHT);
    hpBarBg.fill({ color: 0x333333 });

    const hpBar = new Graphics();
    this._drawHpBar(hpBar, HP_BAR_WIDTH, 1.0);

    container.addChild(body);
    container.addChild(hpBarBg);
    container.addChild(hpBar);

    return { id: m.id, body, hpBar, container };
  }

  private _updateHpBar(bar: Graphics, hp: number, maxHp: number): void {
    const ratio = maxHp > 0 ? Math.max(0, hp / maxHp) : 0;
    this._drawHpBar(bar, HP_BAR_WIDTH * ratio, ratio);
  }

  private _drawHpBar(bar: Graphics, width: number, ratio: number): void {
    bar.clear();
    if (width <= 0) return;
    const colour = ratio > 0.6 ? FILL_FULL : ratio > 0.3 ? FILL_MID : FILL_LOW;
    bar.rect(-HP_BAR_WIDTH / 2, HP_BAR_Y_OFFSET, width, HP_BAR_HEIGHT);
    bar.fill({ color: colour });
  }
}
