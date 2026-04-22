import { useState, useCallback } from 'react';
import { GameCanvas } from '@/components/GameCanvas/index.tsx';
import { GameHud } from './GameHud.tsx';
import type { GameEngine } from '@/game/engine.ts';
import type { GameOverReason, EngineCallbacks } from '@/game/engine.ts';
import type { GameState } from '@/game/logic/types.ts';
import { useProfile } from '@/hooks/useProfile.ts';

interface GameScreenProps {
  mapId?: string;
  onExit: () => void;
}

type GamePhase = 'playing' | 'victory' | 'defeat';

/**
 * Full-screen game view.
 * Renders the Pixi canvas with the HUD overlay, and shows a result card on game over.
 */
export function GameScreen({ mapId = 'alpha', onExit }: GameScreenProps) {
  const { profile } = useProfile();
  // Store the live engine instance in state so the HUD can read isPaused reactively.
  const [engine, setEngine] = useState<GameEngine | null>(null);
  const [gameState, setGameState] = useState<Readonly<GameState> | null>(null);
  const [phase, setPhase] = useState<GamePhase>('playing');

  const onStateChange = useCallback((s: Readonly<GameState>) => {
    setGameState(s);
  }, []);

  const onGameOver = useCallback((reason: GameOverReason) => {
    setPhase(reason === 'victory' ? 'victory' : 'defeat');
  }, []);

  const onMount = useCallback((e: GameEngine) => {
    setEngine(e);
  }, []);

  const callbacks: EngineCallbacks = { onStateChange, onGameOver };

  if (!profile) return null;

  return (
    <div className="game-screen" style={{ position: 'relative', width: '100%', height: '100%' }}>
      <GameCanvas
        mapId={mapId}
        gold={profile.gold}
        gateHp={10}
        onMount={onMount}
        callbacks={callbacks}
      />

      {gameState && phase === 'playing' && (
        <div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            pointerEvents: 'none',
          }}
        >
          <div style={{ pointerEvents: 'auto' }}>
            <GameHud
              state={gameState}
              engine={engine}
              onExit={onExit}
            />
          </div>
        </div>
      )}

      {phase !== 'playing' && (
        <GameOverCard phase={phase} onExit={onExit} onRetry={onExit} />
      )}
    </div>
  );
}

// ── sub-component ─────────────────────────────────────────────────────────────

interface GameOverCardProps {
  phase: 'victory' | 'defeat';
  onExit: () => void;
  onRetry: () => void;
}

function GameOverCard({ phase, onExit, onRetry }: GameOverCardProps) {
  const isVictory = phase === 'victory';
  return (
    <div
      className="game-over-card"
      role="dialog"
      aria-label={isVictory ? 'Victory' : 'Defeat'}
      style={{
        position: 'absolute',
        inset: 0,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: 'rgba(0,0,0,0.75)',
        color: '#fff',
        gap: '1rem',
      }}
    >
      <h2 style={{ fontSize: '2rem' }}>{isVictory ? 'Victory!' : 'Defeat'}</h2>
      <p>{isVictory ? 'You defended the gate!' : 'Your gate was destroyed.'}</p>
      <div style={{ display: 'flex', gap: '1rem' }}>
        <button className="game-over-card__retry" onClick={onRetry}>
          Play Again
        </button>
        <button className="game-over-card__exit" onClick={onExit}>
          Back to Main
        </button>
      </div>
    </div>
  );
}
