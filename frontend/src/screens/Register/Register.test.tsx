import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider } from '@/state/auth.tsx';
import { createMemoryTokenStore } from '@/lib/storage.ts';
import { Register } from './index.tsx';
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
  display_name: 'Bob',
  avatar_id: 0,
  trophies: 0,
  gold: 0,
  diamonds: 0,
  energy: 5,
  energy_max: 5,
  xp: 0,
  level: 1,
};

function renderRegister(onNavigateToLogin = vi.fn()) {
  const store = createMemoryTokenStore();
  render(
    <AuthProvider store={store}>
      <Register onNavigateToLogin={onNavigateToLogin} />
    </AuthProvider>,
  );
  return { store, onNavigateToLogin };
}

describe('Register', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('renders email, username, password inputs and a submit button', () => {
    renderRegister();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create account/i })).toBeInTheDocument();
  });

  it('calls register endpoint with all three fields on submit', async () => {
    vi.mocked(authEndpoints.register).mockResolvedValue(mockTokens);
    vi.mocked(profileEndpoints.getMe).mockResolvedValue(mockProfile);
    renderRegister();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'bob@example.com');
    await user.type(screen.getByLabelText(/username/i), 'bob99');
    await user.type(screen.getByLabelText(/password/i), 'supersecret123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(authEndpoints.register).toHaveBeenCalledWith({
        email: 'bob@example.com',
        username: 'bob99',
        password: 'supersecret123',
      });
    });
  });

  it('displays the error message when registration fails', async () => {
    vi.mocked(authEndpoints.register).mockRejectedValue(new Error('Email already taken.'));
    renderRegister();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'bob@example.com');
    await user.type(screen.getByLabelText(/username/i), 'bob99');
    await user.type(screen.getByLabelText(/password/i), 'supersecret123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Email already taken.');
    });
  });

  it('disables the submit button while registration is in flight', async () => {
    // Default no-op; will be replaced by the Promise executor immediately.
    let resolveRegister: (value: typeof mockTokens) => void = () => {};
    const pending = new Promise<typeof mockTokens>((res) => {
      resolveRegister = res;
    });
    vi.mocked(authEndpoints.register).mockReturnValue(pending);
    renderRegister();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'bob@example.com');
    await user.type(screen.getByLabelText(/username/i), 'bob99');
    await user.type(screen.getByLabelText(/password/i), 'supersecret123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(screen.getByRole('button', { name: /creating account/i })).toBeDisabled();

    resolveRegister(mockTokens);
  });

  it('calls onNavigateToLogin when the login link is clicked', async () => {
    const onNavigateToLogin = vi.fn();
    renderRegister(onNavigateToLogin);

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /log in/i }));

    expect(onNavigateToLogin).toHaveBeenCalledOnce();
  });
});
