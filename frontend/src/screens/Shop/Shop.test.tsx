import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider } from '@/state/auth.tsx';
import { createMemoryTokenStore } from '@/lib/storage.ts';
import * as shopEndpoints from '@/api/endpoints/shop.ts';
import * as profileEndpoints from '@/api/endpoints/profile.ts';
import { Shop } from './index.tsx';

vi.mock('@/api/endpoints/shop.ts');
vi.mock('@/api/endpoints/profile.ts');

const mockProfile = {
  user_id: 'user-1',
  display_name: 'Test User',
  avatar_id: 0,
  trophies: 0,
  gold: 0,
  diamonds: 500,
  energy: 5,
  energy_max: 5,
  xp: 0,
  level: 1,
};

const mockTowerAvailable = {
  id: 'tower-1',
  name: 'Archer Tower',
  rarity: 'common' as const,
  base_damage: 50,
  base_range: 3,
  base_rate: 1,
  cost_diamonds: 100,
  description: 'A basic tower.',
  owned: false,
};

const mockTowerOwned = {
  ...mockTowerAvailable,
  id: 'tower-2',
  name: 'Cannon Tower',
  owned: true,
};

function renderShop() {
  const store = createMemoryTokenStore();
  store.setTokens('access-abc', 'refresh-xyz');
  vi.mocked(profileEndpoints.getMe).mockResolvedValue(mockProfile);
  render(
    <AuthProvider store={store}>
      <Shop />
    </AuthProvider>,
  );
}

describe('Shop', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('shows a loading state initially', () => {
    vi.mocked(shopEndpoints.getCatalog).mockReturnValue(new Promise(() => {}));
    renderShop();
    expect(screen.getByText(/loading shop/i)).toBeInTheDocument();
  });

  it('renders catalog towers after loading', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({
      towers: [mockTowerAvailable, mockTowerOwned],
    });
    renderShop();
    await waitFor(() => {
      expect(screen.getByText('Archer Tower')).toBeInTheDocument();
      expect(screen.getByText('Cannon Tower')).toBeInTheDocument();
    });
  });

  it('shows the price for unowned towers', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerAvailable] });
    renderShop();
    await waitFor(() => {
      expect(screen.getByText('100')).toBeInTheDocument();
    });
  });

  it('shows the Owned badge for already-owned towers', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerOwned] });
    renderShop();
    await waitFor(() => {
      expect(screen.getByText('Owned')).toBeInTheDocument();
    });
  });

  it('disables the buy button for owned towers', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerOwned] });
    renderShop();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /cannon tower/i })).toBeDisabled();
    });
  });

  it('opens the confirm modal when an unowned tower is clicked', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerAvailable] });
    renderShop();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText(/buy archer tower/i)).toBeInTheDocument();
  });

  it('closes the modal when Cancel is clicked', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerAvailable] });
    renderShop();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('calls buyTower and closes the modal on confirm', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerAvailable] });
    vi.mocked(shopEndpoints.buyTower).mockResolvedValue({
      tower: {
        template_id: 'tower-1',
        name: 'Archer Tower',
        rarity: 'common',
        cost_diamonds: 100,
        description: 'A basic tower.',
        current: { level: 1, gold_cost: 200, damage: 50, range: 3, rate: 1 },
      },
    });
    renderShop();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /buy for/i }));
    await waitFor(() => {
      expect(shopEndpoints.buyTower).toHaveBeenCalledWith('tower-1');
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('shows an error in the modal when purchase fails', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockResolvedValue({ towers: [mockTowerAvailable] });
    vi.mocked(shopEndpoints.buyTower).mockRejectedValue(new Error('Insufficient diamonds.'));
    renderShop();
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /archer tower/i }));
    await user.click(screen.getByRole('button', { name: /buy for/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Insufficient diamonds.');
    });
  });

  it('shows an error when the catalog fetch fails', async () => {
    vi.mocked(shopEndpoints.getCatalog).mockRejectedValue(new Error('Network error'));
    renderShop();
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Network error');
    });
  });
});
