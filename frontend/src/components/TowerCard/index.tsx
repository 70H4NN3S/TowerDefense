import type { OwnedTower, Rarity } from '@/api/types.ts';

interface TowerCardProps {
  tower: OwnedTower;
  onClick: () => void;
}

function rarityClass(rarity: Rarity): string {
  return `rarity-badge rarity-badge--${rarity}`;
}

/**
 * Compact card summarising one owned tower.
 * Clicking it opens the UpgradeModal.
 */
export function TowerCard({ tower, onClick }: TowerCardProps) {
  const { name, rarity, current } = tower;
  const isMaxLevel = current.gold_cost === 0;

  return (
    <button className="tower-card" onClick={onClick} aria-label={`${name}, level ${current.level}`}>
      <div className="tower-card__header">
        <span className="tower-card__name">{name}</span>
        <span className={rarityClass(rarity)}>{rarity}</span>
      </div>
      <span className="tower-card__level">
        {isMaxLevel ? 'Max Level' : `Level ${current.level}`}
      </span>
      <div className="tower-card__stats">
        <div className="tower-stat">
          <span>Damage</span>
          <span className="tower-stat__value">{current.damage}</span>
        </div>
        <div className="tower-stat">
          <span>Range</span>
          <span className="tower-stat__value">{current.range}</span>
        </div>
        <div className="tower-stat">
          <span>Rate</span>
          <span className="tower-stat__value">{current.rate}</span>
        </div>
      </div>
    </button>
  );
}
