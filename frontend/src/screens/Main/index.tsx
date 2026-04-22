import { useProfile } from '@/hooks/useProfile.ts';
import { ResourceHud } from '@/components/ResourceHud/index.tsx';

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
 */
export function Main() {
  const { profile } = useProfile();

  if (profile === null) {
    return null;
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

      {/* Play button is a stub until Phase 14 wires up the game canvas. */}
      <button className="main-play-button" aria-label="Play">
        Play
      </button>

      <DailyRewardSlot />
    </div>
  );
}
