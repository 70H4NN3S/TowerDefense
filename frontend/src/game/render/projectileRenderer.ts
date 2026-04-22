/**
 * Renders short-lived projectile animations.
 * A projectile travels from a tower to its target over a fixed duration,
 * then removes itself.
 */

import { Graphics, Container } from 'pixi.js';

const PROJECTILE_DURATION = 0.12; // seconds
const PROJECTILE_RADIUS = 4;
const PROJECTILE_COLOUR = 0xffee44;

interface Projectile {
  gfx: Graphics;
  /** Start pixel position. */
  sx: number;
  sy: number;
  /** End pixel position. */
  ex: number;
  ey: number;
  elapsed: number;
}

export class ProjectileRenderer {
  readonly container: Container;
  private readonly _projectiles: Projectile[] = [];

  constructor() {
    this.container = new Container();
  }

  /**
   * Spawns a new projectile travelling from (sx, sy) to (ex, ey) in pixels.
   */
  spawn(sx: number, sy: number, ex: number, ey: number): void {
    const gfx = new Graphics();
    gfx.circle(0, 0, PROJECTILE_RADIUS);
    gfx.fill({ color: PROJECTILE_COLOUR });
    gfx.x = sx;
    gfx.y = sy;
    this.container.addChild(gfx);
    this._projectiles.push({ gfx, sx, sy, ex, ey, elapsed: 0 });
  }

  /**
   * Advances all active projectiles by dt seconds.
   * Dead projectiles are removed.
   */
  update(dt: number): void {
    let i = 0;
    while (i < this._projectiles.length) {
      const p = this._projectiles[i];
      p.elapsed += dt;
      const t = Math.min(1, p.elapsed / PROJECTILE_DURATION);
      p.gfx.x = p.sx + (p.ex - p.sx) * t;
      p.gfx.y = p.sy + (p.ey - p.sy) * t;
      if (t >= 1) {
        this.container.removeChild(p.gfx);
        p.gfx.destroy();
        this._projectiles.splice(i, 1);
      } else {
        i++;
      }
    }
  }

  destroy(): void {
    this.container.destroy({ children: true });
    this._projectiles.length = 0;
  }
}
