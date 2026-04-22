import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NavProvider } from '@/state/nav.tsx';
import { createMemoryNavStorage, type Tab } from '@/state/nav.ts';
import { TabBar } from './index.tsx';

function renderTabBar(initialTab?: Tab) {
  const storage = createMemoryNavStorage(initialTab);
  render(
    <NavProvider storage={storage}>
      <TabBar />
    </NavProvider>,
  );
  return storage;
}

describe('TabBar', () => {
  it('renders all five tabs', () => {
    renderTabBar();
    expect(screen.getByRole('tab', { name: 'Shop' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Towers' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Main' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Alliance' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Events' })).toBeInTheDocument();
  });

  it('defaults to the Main tab when no initial tab is stored', () => {
    renderTabBar();
    expect(screen.getByRole('tab', { name: 'Main' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Shop' })).toHaveAttribute('aria-selected', 'false');
  });

  it('restores the active tab from storage on mount', () => {
    renderTabBar('events');
    expect(screen.getByRole('tab', { name: 'Events' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Main' })).toHaveAttribute('aria-selected', 'false');
  });

  it('activates the clicked tab', async () => {
    renderTabBar();
    const user = userEvent.setup();
    await user.click(screen.getByRole('tab', { name: 'Shop' }));
    expect(screen.getByRole('tab', { name: 'Shop' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Main' })).toHaveAttribute('aria-selected', 'false');
  });

  it('persists the active tab to storage after navigation', async () => {
    const storage = renderTabBar();
    const user = userEvent.setup();
    await user.click(screen.getByRole('tab', { name: 'Alliance' }));
    expect(storage.getActiveTab()).toBe('alliance');
  });

  it('marks the tab bar with an accessible navigation role', () => {
    renderTabBar();
    expect(screen.getByRole('tablist', { name: 'Main navigation' })).toBeInTheDocument();
  });
});
