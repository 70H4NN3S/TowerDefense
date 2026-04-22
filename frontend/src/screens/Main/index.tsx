import { useState } from 'react';
import { useProfile } from '@/hooks/useProfile.ts';
import { ResourceHud } from '@/components/ResourceHud/index.tsx';
import { GameScreen } from '@/screens/Game/index.tsx';

function DailyRewardSlot() {
  return (
    <div className="daily-reward-slot">
      <span className="daily-reward-slot__title">Daily Reward</span>
      {/* TODO(claude, 2026-04-22): implement daily reward claim logic; see docs/followups.md#daily-reward */}
      <button className="daily-reward-slot__button" disabled>
        Come back tomorrow
      </button>
    </div>
  );
}

/**
 * Main screen — the game hub.
 * Shows the resource HUD, the player's trophy count, a Play button, and
 * the daily reward slot.
 *
 * When the Play button is pressed the GameScreen overlays the whole view.
 */
export function Main() {
  const { profile } = useProfile();
  const [isPlaying, setIsPlaying] = useState(false);

  if (profile === null) {
    return null;
  }

  if (isPlaying) {
    return (
      <div className="main-game-overlay" style={{ position: 'fixed', inset: 0, zIndex: 100 }}>
        <GameScreen onExit={() => setIsPlaying(false)} />
      </div>
    );
  }

  return (
    <div className="main-screen">
      <ResourceHud
        gold={profile.gold}
        diamonds={profile.diamonds}
        energy={profile.energy}
        energyMax={profile.energy_max}
      />

      <div className="main-trophies" aria-label="Trophy count">
        <span className="trophies-count">{profile.trophies}</span>
        <span className="trophies-label">Trophies</span>
      </div>

      <button
        className="main-play-button"
        aria-label="Play"
        onClick={() => setIsPlaying(true)}
      >
        Play
      </button>

      <DailyRewardSlot />
    </div>
  );
}
