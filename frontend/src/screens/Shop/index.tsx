import { useState } from 'react';
import { useShop } from '@/hooks/useShop.ts';
import { useProfile } from '@/hooks/useProfile.ts';
import { ConfirmModal } from '@/components/ConfirmModal/index.tsx';
import type { CatalogTower, Rarity } from '@/api/types.ts';

function rarityClass(rarity: Rarity): string {
  return `rarity-badge rarity-badge--${rarity}`;
}

interface ShopCardProps {
  tower: CatalogTower;
  onBuyClick: () => void;
}

function ShopCard({ tower, onBuyClick }: ShopCardProps) {
  const { name, rarity, description, cost_diamonds, owned } = tower;

  return (
    <button
      className={`shop-card${owned ? ' shop-card--owned' : ''}`}
      onClick={owned ? undefined : onBuyClick}
      disabled={owned}
      aria-label={`${name}, ${cost_diamonds} diamonds${owned ? ', owned' : ''}`}
    >
      <div className="shop-card__header">
        <span className="shop-card__name">{name}</span>
        {owned ? (
          <span className="owned-badge">Owned</span>
        ) : (
          <span className={rarityClass(rarity)}>{rarity}</span>
        )}
      </div>
      <p className="shop-card__description">{description}</p>
      {!owned && (
        <div className="shop-card__price">
          <span>{cost_diamonds}</span>
          <span className="resource-label resource-label--diamond">Diamonds</span>
        </div>
      )}
    </button>
  );
}

/**
 * Shop screen — tower catalog with diamond prices.
 * Owned towers are greyed out. Tapping an unowned tower shows a
 * ConfirmModal before the purchase is submitted.
 */
export function Shop() {
  const { catalog, isLoading, error, buy } = useShop();
  const { refresh: refreshProfile } = useProfile();
  const [pending, setPending] = useState<CatalogTower | null>(null);

  async function handleConfirmBuy() {
    if (pending === null) return;
    await buy(pending.id);
    await refreshProfile();
    setPending(null);
  }

  if (isLoading) {
    return (
      <div className="screen-empty" aria-busy="true">
        Loading shop…
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

  return (
    <>
      <div className="screen-header">
        <h1 className="screen-title">Shop</h1>
      </div>
      <div className="shop-grid">
        {catalog.map((tower) => (
          <ShopCard key={tower.id} tower={tower} onBuyClick={() => setPending(tower)} />
        ))}
      </div>
      {pending !== null && (
        <ConfirmModal
          title={`Buy ${pending.name}?`}
          confirmLabel={`Buy for ${pending.cost_diamonds} Diamonds`}
          onConfirm={handleConfirmBuy}
          onCancel={() => setPending(null)}
        >
          <p>
            This will spend{' '}
            <strong className="resource-label resource-label--diamond">
              {pending.cost_diamonds} Diamonds
            </strong>{' '}
            from your account.
          </p>
        </ConfirmModal>
      )}
    </>
  );
}
