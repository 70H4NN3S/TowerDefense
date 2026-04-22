import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider } from '@/state/auth.tsx';
import { createMemoryTokenStore } from '@/lib/storage.ts';
import { Login } from './index.tsx';
import * as authEndpoints from '@/api/endpoints/auth.ts';
import * as profileEndpoints from '@/api/endpoints/profile.ts';

vi.mock('@/api/endpoints/auth.ts');
vi.mock('@/api/endpoints/profile.ts');

const mockTokens = {
  access_token: 'access-abc',
  refresh_token: 'refresh-xyz',
  expires_in: 3600,
};

const mockProfile = {
  user_id: 'user-1',
  display_name: 'Test User',
  avatar_id: 0,
  trophies: 0,
  gold: 0,
  diamonds: 0,
  energy: 5,
  energy_max: 5,
  xp: 0,
  level: 1,
};

function renderLogin(onNavigateToRegister = vi.fn()) {
  const store = createMemoryTokenStore();
  render(
    <AuthProvider store={store}>
      <Login onNavigateToRegister={onNavigateToRegister} />
    </AuthProvider>,
  );
  return { store, onNavigateToRegister };
}

describe('Login', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('renders email, password inputs and a submit button', () => {
    renderLogin();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /log in/i })).toBeInTheDocument();
  });

  it('calls login endpoint with typed credentials on submit', async () => {
    vi.mocked(authEndpoints.login).mockResolvedValue(mockTokens);
    vi.mocked(profileEndpoints.getMe).mockResolvedValue(mockProfile);
    renderLogin();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'alice@example.com');
    await user.type(screen.getByLabelText(/password/i), 'hunter2');
    await user.click(screen.getByRole('button', { name: /log in/i }));

    await waitFor(() => {
      expect(authEndpoints.login).toHaveBeenCalledWith({
        email: 'alice@example.com',
        password: 'hunter2',
      });
    });
  });

  it('displays the error message when login fails', async () => {
    vi.mocked(authEndpoints.login).mockRejectedValue(new Error('Invalid credentials.'));
    renderLogin();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'alice@example.com');
    await user.type(screen.getByLabelText(/password/i), 'wrong');
    await user.click(screen.getByRole('button', { name: /log in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid credentials.');
    });
  });

  it('disables the submit button while a login is in flight', async () => {
    // Default no-op; will be replaced by the Promise executor immediately.
    let resolveLogin: (value: typeof mockTokens) => void = () => {};
    const pending = new Promise<typeof mockTokens>((res) => {
      resolveLogin = res;
    });
    vi.mocked(authEndpoints.login).mockReturnValue(pending);
    renderLogin();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'alice@example.com');
    await user.type(screen.getByLabelText(/password/i), 'hunter2');
    await user.click(screen.getByRole('button', { name: /log in/i }));

    expect(screen.getByRole('button', { name: /logging in/i })).toBeDisabled();

    // Resolve so the async chain doesn't dangle.
    resolveLogin(mockTokens);
  });

  it('calls onNavigateToRegister when the register link is clicked', async () => {
    const onNavigateToRegister = vi.fn();
    renderLogin(onNavigateToRegister);

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /register/i }));

    expect(onNavigateToRegister).toHaveBeenCalledOnce();
  });
});
