import { useState } from 'react';
import type { OwnedTower } from '@/api/types.ts';

interface UpgradeModalProps {
  tower: OwnedTower;
  gold: number;
  onUpgrade: () => Promise<void>;
  onClose: () => void;
}

/**
 * Bottom-sheet modal for upgrading an owned tower.
 * Shows current stats and the gold cost of the next upgrade.
 * The Upgrade button is disabled when the player cannot afford it or the
 * tower is at max level.
 *
 * Note: per-level stat deltas require a backend API extension to include
 * next-level stats in the owned-tower response. Tracked in docs/followups.md.
 */
export function UpgradeModal({ tower, gold, onUpgrade, onClose }: UpgradeModalProps) {
  const [isUpgrading, setIsUpgrading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { name, current } = tower;
  const isMaxLevel = current.gold_cost === 0;
  const canAfford = gold >= current.gold_cost;

  async function handleUpgrade() {
    setError(null);
    setIsUpgrading(true);
    try {
      await onUpgrade();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upgrade failed.');
    } finally {
      setIsUpgrading(false);
    }
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-label={`Upgrade ${name}`}>
      <div className="modal-sheet">
        <h2 className="modal-title">{name}</h2>
        <p className="modal-subtitle">Level {current.level}</p>

        <div>
          <p className="modal-section-label">Current stats</p>
          <div className="modal-stats">
            <div className="modal-stat-row">
              <span>Damage</span>
              <span className="modal-stat-row__value">{current.damage}</span>
            </div>
            <div className="modal-stat-row">
              <span>Range</span>
              <span className="modal-stat-row__value">{current.range}</span>
            </div>
            <div className="modal-stat-row">
              <span>Rate</span>
              <span className="modal-stat-row__value">{current.rate}</span>
            </div>
          </div>
        </div>

        {!isMaxLevel && (
          <div className="modal-cost" aria-label={`Upgrade cost: ${current.gold_cost} gold`}>
            <span>Cost:</span>
            <span className="modal-cost__amount">{current.gold_cost} Gold</span>
          </div>
        )}

        {error !== null && (
          <p role="alert" className="screen-error">
            {error}
          </p>
        )}

        <div className="modal-actions">
          <button className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button
            className="btn-primary"
            onClick={handleUpgrade}
            disabled={isMaxLevel || !canAfford || isUpgrading}
            aria-disabled={isMaxLevel || !canAfford || isUpgrading}
          >
            {isMaxLevel
              ? 'Max Level'
              : !canAfford
                ? 'Not enough gold'
                : isUpgrading
                  ? 'Upgrading…'
                  : 'Upgrade'}
          </button>
        </div>
      </div>
    </div>
  );
}
