interface ResourceHudProps {
  gold: number;
  diamonds: number;
  energy: number;
  energyMax: number;
}

/** Abbreviates large numbers: 1500 → "1.5K", 1_500_000 → "1.5M". */
function formatResource(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toString();
}

/**
 * Compact HUD strip showing gold, diamonds, and energy.
 * Rendered at the top of the Main screen and reused wherever resources
 * need to be visible at a glance.
 */
export function ResourceHud({ gold, diamonds, energy, energyMax }: ResourceHudProps) {
  return (
    <div className="resource-hud" aria-label="Resources">
      <div className="resource-item">
        <span className="resource-label resource-label--gold">Gold</span>
        <span className="resource-value">{formatResource(gold)}</span>
      </div>
      <div className="resource-item">
        <span className="resource-label resource-label--diamond">Diamonds</span>
        <span className="resource-value">{formatResource(diamonds)}</span>
      </div>
      <div className="resource-item">
        <span className="resource-label resource-label--energy">Energy</span>
        <span className="resource-value">
          {energy}/{energyMax}
        </span>
      </div>
    </div>
  );
}
