import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthProvider } from '@/state/auth.tsx';
import { createMemoryTokenStore } from '@/lib/storage.ts';
import * as profileEndpoints from '@/api/endpoints/profile.ts';
import { Main } from './index.tsx';

vi.mock('@/api/endpoints/profile.ts');

const mockProfile = {
  user_id: 'user-1',
  display_name: 'Test User',
  avatar_id: 0,
  trophies: 420,
  gold: 5000,
  diamonds: 80,
  energy: 3,
  energy_max: 5,
  xp: 1200,
  level: 4,
};

function renderMain() {
  // Pre-seed a token so AuthProvider calls getMe() on mount and populates user.
  const store = createMemoryTokenStore();
  store.setTokens('access-abc', 'refresh-xyz');
  vi.mocked(profileEndpoints.getMe).mockResolvedValue(mockProfile);
  render(
    <AuthProvider store={store}>
      <Main />
    </AuthProvider>,
  );
}

describe('Main', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('renders the play button after the profile loads', async () => {
    renderMain();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /play/i })).toBeInTheDocument();
    });
  });

  it('renders the resource HUD after the profile loads', async () => {
    renderMain();
    await waitFor(() => {
      expect(screen.getByText(/gold/i)).toBeInTheDocument();
      expect(screen.getByText(/diamonds/i)).toBeInTheDocument();
      expect(screen.getByText(/energy/i)).toBeInTheDocument();
    });
  });

  it('renders nothing while the profile is null', () => {
    const store = createMemoryTokenStore();
    // No token → AuthProvider never calls getMe() → user stays null.
    vi.mocked(profileEndpoints.getMe).mockResolvedValue(mockProfile);
    const { container } = render(
      <AuthProvider store={store}>
        <Main />
      </AuthProvider>,
    );
    expect(container.firstChild).toBeNull();
  });
});

describe('ResourceHud', () => {
  it('abbreviates gold values above 1000', async () => {
    renderMain();
    // 5000 gold → "5.0K"
    await waitFor(() => {
      expect(screen.getByText('5.0K')).toBeInTheDocument();
    });
  });

  it('shows energy as a fraction', async () => {
    renderMain();
    await waitFor(() => {
      expect(screen.getByText('3/5')).toBeInTheDocument();
    });
  });
});
