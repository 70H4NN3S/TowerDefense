import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi, describe, it, expect } from 'vitest';
import { GameHud } from './GameHud.tsx';
import type { GameState } from '@/game/logic/types.ts';
import type { GameEngine } from '@/game/engine.ts';

// ── fixture ───────────────────────────────────────────────────────────────────

function makeState(overrides: Partial<GameState> = {}): GameState {
  return {
    map: {
      id: 'test',
      cols: 10,
      rows: 5,
      waypoints: [],
      gate: { x: 10, y: 0 },
      tiles: [],
    },
    towers: [],
    monsters: [],
    waves: [
      { number: 1, groups: [] },
      { number: 2, groups: [] },
    ],
    waveIdx: 0,
    waveTime: 0,
    spawnRecords: [],
    gold: 150,
    gateHp: 8,
    tick: 0,
    nextId: 0,
    ...overrides,
  } as GameState;
}

function makeEngine(isPaused = false): GameEngine {
  return {
    isPaused,
    pause: vi.fn(),
    resume: vi.fn(),
  } as unknown as GameEngine;
}

// ── tests ─────────────────────────────────────────────────────────────────────

describe('GameHud', () => {
  it('renders gold amount', () => {
    render(<GameHud state={makeState({ gold: 250 })} engine={null} onExit={() => {}} />);
    expect(screen.getByLabelText('Gold: 250')).toBeInTheDocument();
  });

  it('renders wave counter', () => {
    render(<GameHud state={makeState({ waveIdx: 0 })} engine={null} onExit={() => {}} />);
    expect(screen.getByLabelText('Wave 1 of 2')).toBeInTheDocument();
  });

  it('renders gate HP bar', () => {
    render(<GameHud state={makeState({ gateHp: 5 })} engine={null} onExit={() => {}} />);
    expect(screen.getByLabelText('Gate HP: 5')).toBeInTheDocument();
  });

  it('shows gate HP progressbar with correct value', () => {
    render(<GameHud state={makeState({ gateHp: 7 })} engine={null} onExit={() => {}} />);
    const bar = screen.getByRole('progressbar');
    expect(bar).toHaveAttribute('aria-valuenow', '70');
  });

  it('calls engine.pause when pause button clicked', async () => {
    const engine = makeEngine(false);
    render(<GameHud state={makeState()} engine={engine} onExit={() => {}} />);
    await userEvent.click(screen.getByLabelText('Pause'));
    expect(engine.pause).toHaveBeenCalled();
  });

  it('calls engine.resume when resume button clicked while paused', async () => {
    const engine = makeEngine(true);
    render(<GameHud state={makeState()} engine={engine} onExit={() => {}} />);
    await userEvent.click(screen.getByLabelText('Resume'));
    expect(engine.resume).toHaveBeenCalled();
  });

  it('shows pause overlay when engine is paused', () => {
    const engine = makeEngine(true);
    render(<GameHud state={makeState()} engine={engine} onExit={() => {}} />);
    expect(screen.getByRole('status')).toHaveTextContent('Paused');
  });

  it('hides pause overlay when not paused', () => {
    const engine = makeEngine(false);
    render(<GameHud state={makeState()} engine={engine} onExit={() => {}} />);
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('calls onExit when exit button is clicked', async () => {
    const onExit = vi.fn();
    render(<GameHud state={makeState()} engine={null} onExit={onExit} />);
    await userEvent.click(screen.getByLabelText('Exit game'));
    expect(onExit).toHaveBeenCalled();
  });
});
