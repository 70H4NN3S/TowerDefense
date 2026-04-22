/**
 * Renders the static map: placement tiles, path, and gate.
 * Must be destroyed when the game ends to release GPU resources.
 */

import { Graphics, Container } from 'pixi.js';
import type { GameMap } from '../logic/types.ts';
import { tileCenter } from '../logic/geometry.ts';

const TILE_SIZE = 64; // pixels per world unit

// Colour palette
const TILE_COLOUR = 0x3a5a2a; // dark green — placeable tile
const PATH_COLOUR = 0xc8a050; // sandy brown — path background
const GATE_COLOUR = 0xdd4444; // red — gate marker
const BORDER_COLOUR = 0x2a3a1a; // darker green — grid lines

export class MapRenderer {
  readonly container: Container;
  private readonly _gfx: Graphics;

  constructor() {
    this.container = new Container();
    this._gfx = new Graphics();
    this.container.addChild(this._gfx);
  }

  /**
   * Draws the map once. Call whenever a new map is loaded.
   * The canvas coordinate origin is the top-left corner of tile (0,0).
   */
  draw(map: GameMap): void {
    const g = this._gfx;
    g.clear();

    this._drawGrid(map);
    this._drawPath(map);
    this._drawGate(map);
    this._drawPlacementTiles(map);
  }

  destroy(): void {
    this.container.destroy({ children: true });
  }

  // ── private ─────────────────────────────────────────────────────────────────

  private _drawGrid(map: GameMap): void {
    const g = this._gfx;
    // Background for the whole map area.
    g.rect(0, 0, map.cols * TILE_SIZE, map.rows * TILE_SIZE);
    g.fill({ color: 0x2a2a2a });

    // Grid lines.
    g.setStrokeStyle({ color: BORDER_COLOUR, width: 1, alpha: 0.4 });
    for (let col = 0; col <= map.cols; col++) {
      g.moveTo(col * TILE_SIZE, 0);
      g.lineTo(col * TILE_SIZE, map.rows * TILE_SIZE);
    }
    for (let row = 0; row <= map.rows; row++) {
      g.moveTo(0, row * TILE_SIZE);
      g.lineTo(map.cols * TILE_SIZE, row * TILE_SIZE);
    }
    g.stroke();
  }

  private _drawPath(map: GameMap): void {
    const g = this._gfx;
    if (map.waypoints.length < 2) return;

    // Thick path band connecting waypoints.
    const half = TILE_SIZE * 0.5;
    g.setStrokeStyle({ color: PATH_COLOUR, width: TILE_SIZE, cap: 'square', join: 'miter' });
    const [first, ...rest] = map.waypoints;
    g.moveTo(first.x * TILE_SIZE, first.y * TILE_SIZE);
    for (const pt of rest) {
      g.lineTo(pt.x * TILE_SIZE, pt.y * TILE_SIZE);
    }
    g.stroke();
    void half; // used conceptually for path width
  }

  private _drawGate(map: GameMap): void {
    const g = this._gfx;
    const cx = map.gate.x * TILE_SIZE;
    const cy = map.gate.y * TILE_SIZE;
    const r = TILE_SIZE * 0.4;
    g.rect(cx - r, cy - r, r * 2, r * 2);
    g.fill({ color: GATE_COLOUR });
    g.setStrokeStyle({ color: 0xffffff, width: 2 });
    g.rect(cx - r, cy - r, r * 2, r * 2);
    g.stroke();
  }

  private _drawPlacementTiles(map: GameMap): void {
    const g = this._gfx;
    for (const tile of map.tiles) {
      const x = tile.col * TILE_SIZE;
      const y = tile.row * TILE_SIZE;
      const pad = 3;
      g.rect(x + pad, y + pad, TILE_SIZE - pad * 2, TILE_SIZE - pad * 2);
      g.fill({ color: TILE_COLOUR, alpha: 0.5 });
    }
  }
}

/** Converts world-space coordinates to pixel coordinates. */
export function worldToPixel(wx: number, wy: number): { px: number; py: number } {
  return { px: wx * TILE_SIZE, py: wy * TILE_SIZE };
}

/** Converts pixel coordinates to tile col/row (floored). */
export function pixelToTile(px: number, py: number): { col: number; row: number } {
  return { col: Math.floor(px / TILE_SIZE), row: Math.floor(py / TILE_SIZE) };
}

export { TILE_SIZE, tileCenter };
