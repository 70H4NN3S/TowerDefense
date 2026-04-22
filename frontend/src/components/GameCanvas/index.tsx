import { useEffect, useRef } from 'react';
import { GameEngine } from '@/game/engine.ts';
import type { EngineCallbacks } from '@/game/engine.ts';

interface GameCanvasProps {
  mapId: string;
  gold: number;
  gateHp: number;
  callbacks?: EngineCallbacks;
  /**
   * Called once the engine is mounted and ready.
   * Gives the parent access to engine methods without reading a ref during render.
   */
  onMount?: (engine: GameEngine) => void;
}

/**
 * Mounts the Pixi game canvas on a div and owns the engine lifecycle.
 * Cleans up fully when unmounted.
 */
export function GameCanvas({
  mapId,
  gold,
  gateHp,
  callbacks = {},
  onMount,
}: GameCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const engineRef = useRef<GameEngine | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const engine = new GameEngine();
    engineRef.current = engine;

    engine.mount(container, callbacks).then(() => {
      engine.startGame(mapId, gold, gateHp);
      onMount?.(engine);
    });

    return () => {
      engine.unmount();
      engineRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mapId]); // re-mount only when the map changes; callbacks are stable via useCallback

  return (
    <div
      ref={containerRef}
      className="game-canvas-container"
      style={{ width: '100%', height: '100%', overflow: 'hidden', touchAction: 'none' }}
    />
  );
}
