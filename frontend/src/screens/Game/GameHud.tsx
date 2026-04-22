import type { GameState } from '@/game/logic/types.ts';
import type { GameEngine } from '@/game/engine.ts';

interface GameHudProps {
  state: Readonly<GameState>;
  engine: GameEngine | null;
  onExit: () => void;
}

/**
 * HUD overlay rendered as React components above the Pixi canvas.
 * Contains energy bar, wave counter, gate HP, and pause/exit controls.
 */
export function GameHud({ state, engine, onExit }: GameHudProps) {
  const isPaused = engine?.isPaused ?? false;
  const waveNumber = Math.min(state.waveIdx + 1, state.waves.length);
  const totalWaves = state.waves.length;
  const gateHpPct = state.gateHp > 0 ? Math.min(100, state.gateHp * 10) : 0;

  function handlePause() {
    if (!engine) return;
    if (isPaused) {
      engine.resume();
    } else {
      engine.pause();
    }
  }

  return (
    <div className="game-hud" aria-label="Game HUD">
      {/* Top bar */}
      <div className="game-hud__top">
        <GoldDisplay gold={state.gold} />
        <WaveCounter current={waveNumber} total={totalWaves} />
        <div className="game-hud__controls">
          <button
            className="game-hud__pause-btn"
            aria-label={isPaused ? 'Resume' : 'Pause'}
            onClick={handlePause}
          >
            {isPaused ? '▶' : '⏸'}
          </button>
          <button
            className="game-hud__exit-btn"
            aria-label="Exit game"
            onClick={onExit}
          >
            ✕
          </button>
        </div>
      </div>

      {/* Gate HP bar */}
      <GateHpBar hp={state.gateHp} pct={gateHpPct} />

      {/* Pause overlay */}
      {isPaused && (
        <div className="game-hud__pause-overlay" role="status" aria-live="polite">
          <span>Paused</span>
          <button
            className="game-hud__resume-btn"
            onClick={handlePause}
          >
            Resume
          </button>
        </div>
      )}
    </div>
  );
}

// ── sub-components ────────────────────────────────────────────────────────────

interface GoldDisplayProps {
  gold: number;
}

function GoldDisplay({ gold }: GoldDisplayProps) {
  return (
    <div className="game-hud__gold" aria-label={`Gold: ${gold}`}>
      <span className="game-hud__gold-icon">🪙</span>
      <span className="game-hud__gold-value">{gold}</span>
    </div>
  );
}

interface WaveCounterProps {
  current: number;
  total: number;
}

function WaveCounter({ current, total }: WaveCounterProps) {
  return (
    <div className="game-hud__wave" aria-label={`Wave ${current} of ${total}`}>
      <span>Wave {current} / {total}</span>
    </div>
  );
}

interface GateHpBarProps {
  hp: number;
  pct: number;
}

function GateHpBar({ hp, pct }: GateHpBarProps) {
  const colour = pct > 60 ? '#44cc44' : pct > 30 ? '#cccc44' : '#cc4444';
  return (
    <div className="game-hud__gate-bar" aria-label={`Gate HP: ${hp}`}>
      <span className="game-hud__gate-label">Gate</span>
      <div className="game-hud__gate-track" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
        <div
          className="game-hud__gate-fill"
          style={{ width: `${pct}%`, backgroundColor: colour }}
        />
      </div>
      <span className="game-hud__gate-hp">{hp}</span>
    </div>
  );
}
