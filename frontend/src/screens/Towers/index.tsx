import { useState } from 'react';
import { useTowers } from '@/hooks/useTowers.ts';
import { useProfile } from '@/hooks/useProfile.ts';
import { TowerCard } from '@/components/TowerCard/index.tsx';
import { UpgradeModal } from '@/components/UpgradeModal/index.tsx';
import type { OwnedTower } from '@/api/types.ts';

/**
 * Towers screen — grid of owned towers.
 * Tapping a card opens an UpgradeModal.
 */
export function Towers() {
  const { towers, isLoading, error, upgradeTower } = useTowers();
  const { profile, refresh: refreshProfile } = useProfile();
  const [selected, setSelected] = useState<OwnedTower | null>(null);

  async function handleUpgrade(templateId: string) {
    await upgradeTower(templateId);
    await refreshProfile();
  }

  if (isLoading) {
    return (
      <div className="screen-empty" aria-busy="true">
        Loading towers…
      </div>
    );
  }

  if (error !== null) {
    return (
      <p className="screen-error" role="alert">
        {error}
      </p>
    );
  }

  if (towers.length === 0) {
    return (
      <div className="screen-empty">
        <p>You don&apos;t own any towers yet.</p>
        <p>Visit the Shop to unlock your first tower.</p>
      </div>
    );
  }

  const gold = profile?.gold ?? 0;

  return (
    <>
      <div className="screen-header">
        <h1 className="screen-title">Towers</h1>
      </div>
      <div className="towers-grid">
        {towers.map((tower) => (
          <TowerCard key={tower.template_id} tower={tower} onClick={() => setSelected(tower)} />
        ))}
      </div>
      {selected !== null && (
        <UpgradeModal
          tower={selected}
          gold={gold}
          onUpgrade={() => handleUpgrade(selected.template_id)}
          onClose={() => setSelected(null)}
        />
      )}
    </>
  );
}
