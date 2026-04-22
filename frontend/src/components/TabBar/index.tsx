import { useNav } from '@/hooks/useNav.ts';
import type { Tab } from '@/state/nav.ts';

interface TabItem {
  tab: Tab;
  label: string;
}

const TABS: TabItem[] = [
  { tab: 'shop', label: 'Shop' },
  { tab: 'towers', label: 'Towers' },
  { tab: 'main', label: 'Main' },
  { tab: 'alliance', label: 'Alliance' },
  { tab: 'events', label: 'Events' },
];

/**
 * Persistent bottom navigation bar exposing the five game sections.
 * Uses ARIA tablist/tab roles so the active section is announced to
 * screen readers.
 */
export function TabBar() {
  const { activeTab, navigate } = useNav();

  return (
    <nav role="tablist" aria-label="Main navigation" className="tab-bar">
      {TABS.map(({ tab, label }) => (
        <button
          key={tab}
          role="tab"
          aria-selected={activeTab === tab}
          className="tab-bar__button"
          onClick={() => navigate(tab)}
        >
          {label}
        </button>
      ))}
    </nav>
  );
}
