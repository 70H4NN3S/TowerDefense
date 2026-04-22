import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider } from '@/state/auth.tsx';
import { createMemoryTokenStore } from '@/lib/storage.ts';
import * as towersEndpoints from '@/api/endpoints/towers.ts';
import * as profileEndpoints from '@/api/endpoints/profile.ts';
import { Towers } from './index.tsx';

vi.mock('@/api/endpoints/towers.ts');
vi.mock('@/api/endpoints/profile.ts');

const mockProfile = {
  user_id: 'user-1',
  display_name: 'Test User',
  avatar_id: 0,
  trophies: 0,
  gold: 2000,
  diamonds: 50,
  energy: 5,
  energy_max: 5,
  xp: 0,
  level: 1,
};

const mockTower = {
  template_id: 'tower-1',
  name: 'Archer Tower',
  rarity: 'common' as const,
  cost_diamonds: 100,
  description: 'A basic tower.',
  current: {
    level: 1,
    gold_cost: 200,
    damage: 50,
    range: 3,
    rate: 1,
  },
};

const maxLevelTower = {
  ...mockTower,
  template_id: 'tower-2',
  name: 'Max Tower',
  current: { ...mockTower.current, level: 10, gold_cost: 0 },
};

function renderTowers(profile = mockProfile) {
  const store = createMemoryTokenStore();
  store.setTokens('access-abc', 'refresh-xyz');
  vi.mocked(profileEndpoints.getMe).mockResolvedValue(profile);
  render(
    <AuthProvider store={store}>
      <Towers />
    </AuthProvider>,
  );
}

describe('Towers', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('shows a loading state initially', () => {
    vi.mocked(towersEndpoints.getOwned).mockReturnValue(new Promise(() => {}));
    renderTowers();
    expect(screen.getByText(/loading towers/i)).toBeInTheDocument();
  });

  it('shows an empty state when the player owns no towers', async () => {
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [] });
    renderTowers();
    await waitFor(() => {
      expect(screen.getByText(/don't own any towers/i)).toBeInTheDocument();
    });
  });

  it('renders a card for each owned tower', async () => {
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [mockTower] });
    renderTowers();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /archer tower/i })).toBeInTheDocument();
    });
  });

  it('shows an error message when the fetch fails', async () => {
    vi.mocked(towersEndpoints.getOwned).mockRejectedValue(new Error('Network error'));
    renderTowers();
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Network error');
    });
  });

  it('opens the upgrade modal on card click', async () => {
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [mockTower] });
    renderTowers();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    // Modal title confirms the right tower is shown.
    expect(screen.getByRole('heading', { name: 'Archer Tower' })).toBeInTheDocument();
  });

  it('closes the upgrade modal when Cancel is clicked', async () => {
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [mockTower] });
    renderTowers();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('disables the upgrade button when the tower is at max level', async () => {
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [maxLevelTower] });
    renderTowers();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /max tower/i }));
    await user.click(screen.getByRole('button', { name: /max tower/i }));
    expect(screen.getByRole('button', { name: /max level/i })).toBeDisabled();
  });

  it('disables the upgrade button when gold is insufficient', async () => {
    const poorProfile = { ...mockProfile, gold: 100 }; // tower costs 200
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [mockTower] });
    renderTowers(poorProfile);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /not enough gold/i })).toBeDisabled();
    });
  });

  it('calls the upgrade endpoint and closes the modal on success', async () => {
    vi.mocked(towersEndpoints.getOwned).mockResolvedValue({ towers: [mockTower] });
    vi.mocked(towersEndpoints.upgrade).mockResolvedValue({
      tower: { ...mockTower, current: { ...mockTower.current, level: 2, gold_cost: 400 } },
    });
    vi.mocked(profileEndpoints.getMe).mockResolvedValue(mockProfile);
    renderTowers();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /^upgrade$/i }));
    await waitFor(() => {
      expect(towersEndpoints.upgrade).toHaveBeenCalledWith('tower-1');
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });
});
