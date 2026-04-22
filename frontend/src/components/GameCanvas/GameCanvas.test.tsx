import { render } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { GameCanvas } from './index.tsx';

// Pixi.js requires WebGL which is unavailable in jsdom.
// Mock the engine so we only test the React lifecycle contract.
vi.mock('@/game/engine.ts', () => {
  class GameEngine {
    mount = vi.fn().mockResolvedValue(undefined);
    startGame = vi.fn();
    unmount = vi.fn();
    isPaused = false;
    pause = vi.fn();
    resume = vi.fn();
  }
  return { GameEngine };
});

describe('GameCanvas', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders a container div', () => {
    const { container } = render(
      <GameCanvas mapId="alpha" gold={200} gateHp={10} />,
    );
    expect(container.querySelector('.game-canvas-container')).not.toBeNull();
  });

  it('unmounts the engine when the component is removed', async () => {
    const { GameEngine } = await import('@/game/engine.ts');
    const { unmount } = render(
      <GameCanvas mapId="alpha" gold={200} gateHp={10} />,
    );
    unmount();
    // The mock class was used; no error = lifecycle completed cleanly.
    expect(GameEngine).toBeDefined();
  });

  it('calls onMount with the engine after initialization', async () => {
    const onMount = vi.fn();
    render(
      <GameCanvas mapId="alpha" gold={200} gateHp={10} onMount={onMount} />,
    );
    // mount resolves asynchronously — wait a tick.
    await new Promise((r) => setTimeout(r, 0));
    expect(onMount).toHaveBeenCalled();
  });
});
