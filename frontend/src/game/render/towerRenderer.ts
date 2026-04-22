/**
 * Renders placed towers and a ghost tile during drag-and-drop placement.
 */

import { Graphics, Container } from 'pixi.js';
import type { Tower, Tile } from '../logic/types.ts';
import { TILE_SIZE, pixelToTile } from './mapRenderer.ts';

const TOWER_COLOUR = 0x3388ff;
const TOWER_STROKE = 0xffffff;
const GHOST_VALID_COLOUR = 0x44aaff;
const GHOST_INVALID_COLOUR = 0xff4444;
const RANGE_COLOUR = 0xffffff;

/** A single rendered tower. */
interface TowerSprite {
  id: number;
  gfx: Graphics;
}

export class TowerRenderer {
  readonly container: Container;
  /** Semi-transparent preview shown while the player drags a tower. */
  private readonly _ghost: Graphics;
  private readonly _sprites = new Map<number, TowerSprite>();

  constructor() {
    this.container = new Container();
    this._ghost = new Graphics();
    this._ghost.visible = false;
    this.container.addChild(this._ghost);
  }

  /** Syncs rendered sprites with the current simulation state. */
  update(towers: Tower[]): void {
    const liveIds = new Set(towers.map((t) => t.id));

    // Add new towers.
    for (const tower of towers) {
      if (!this._sprites.has(tower.id)) {
        const gfx = this._drawTower(tower);
        this._sprites.set(tower.id, { id: tower.id, gfx });
        // Insert before the ghost so the ghost stays on top.
        this.container.addChildAt(gfx, this.container.children.indexOf(this._ghost));
      }
    }

    // Remove stale sprites.
    for (const [id, sprite] of this._sprites) {
      if (!liveIds.has(id)) {
        this.container.removeChild(sprite.gfx);
        sprite.gfx.destroy();
        this._sprites.delete(id);
      }
    }
  }

  /**
   * Shows the placement ghost at the tile under the pointer.
   * @param isValid - whether the tile is a valid (unoccupied, in-bounds) placement position
   * @param range - tower range in world units (drawn as circle)
   */
  showGhost(tile: Tile, isValid: boolean, range: number): void {
    const g = this._ghost;
    g.clear();
    g.visible = true;

    const x = tile.col * TILE_SIZE;
    const y = tile.row * TILE_SIZE;
    const pad = 6;
    const colour = isValid ? GHOST_VALID_COLOUR : GHOST_INVALID_COLOUR;

    // Tile highlight.
    g.rect(x + pad, y + pad, TILE_SIZE - pad * 2, TILE_SIZE - pad * 2);
    g.fill({ color: colour, alpha: 0.5 });
    g.setStrokeStyle({ color: colour, width: 2 });
    g.rect(x + pad, y + pad, TILE_SIZE - pad * 2, TILE_SIZE - pad * 2);
    g.stroke();

    // Range circle.
    const cx = (tile.col + 0.5) * TILE_SIZE;
    const cy = (tile.row + 0.5) * TILE_SIZE;
    const rangePx = range * TILE_SIZE;
    g.setStrokeStyle({ color: RANGE_COLOUR, width: 1, alpha: 0.4 });
    g.circle(cx, cy, rangePx);
    g.stroke();
  }

  /** Hides the placement ghost. */
  hideGhost(): void {
    this._ghost.visible = false;
    this._ghost.clear();
  }

  destroy(): void {
    this.container.destroy({ children: true });
    this._sprites.clear();
  }

  // ── private ─────────────────────────────────────────────────────────────────

  private _drawTower(tower: Tower): Graphics {
    const g = new Graphics();
    const x = tower.tile.col * TILE_SIZE;
    const y = tower.tile.row * TILE_SIZE;
    const pad = 8;

    // Body.
    g.rect(x + pad, y + pad, TILE_SIZE - pad * 2, TILE_SIZE - pad * 2);
    g.fill({ color: TOWER_COLOUR });
    g.setStrokeStyle({ color: TOWER_STROKE, width: 2 });
    g.rect(x + pad, y + pad, TILE_SIZE - pad * 2, TILE_SIZE - pad * 2);
    g.stroke();

    // Barrel (pointing up for now).
    const cx = x + TILE_SIZE / 2;
    g.rect(cx - 3, y + pad, 6, TILE_SIZE / 2 - pad);
    g.fill({ color: TOWER_STROKE });

    return g;
  }
}

export { pixelToTile };
