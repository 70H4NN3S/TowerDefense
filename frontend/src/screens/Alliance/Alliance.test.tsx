import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as profileEndpoints from '@/api/endpoints/profile.ts';
import * as alliancesEndpoints from '@/api/endpoints/alliances.ts';
import { Alliance } from './index.tsx';

vi.mock('@/api/endpoints/profile.ts');
vi.mock('@/api/endpoints/alliances.ts');

const mockMembership = {
  user_id: 'user-1',
  alliance_id: 'alliance-1',
  role: 'member' as const,
  joined_at: '2026-01-01T00:00:00Z',
};

const mockAlliance = {
  id: 'alliance-1',
  name: 'Tower Lords',
  tag: 'TL',
  description: 'We defend together.',
  leader_id: 'user-2',
  channel_id: 'channel-1',
  created_at: '2026-01-01T00:00:00Z',
};

const mockMembers = [
  {
    user_id: 'user-2',
    alliance_id: 'alliance-1',
    role: 'leader' as const,
    joined_at: '2026-01-01T00:00:00Z',
  },
  {
    user_id: 'user-1',
    alliance_id: 'alliance-1',
    role: 'member' as const,
    joined_at: '2026-01-02T00:00:00Z',
  },
];

describe('Alliance — no alliance', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    // Simulate "not in an alliance" — profile endpoint returns not_found.
    vi.mocked(profileEndpoints.getMyAlliance).mockRejectedValue(new Error('not_found'));
    vi.mocked(profileEndpoints.leaveAlliance).mockResolvedValue(undefined);
  });

  it('shows Create and Browse buttons when not in an alliance', async () => {
    render(<Alliance />);
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alliance/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /browse alliances/i })).toBeInTheDocument();
    });
  });

  it('shows the create form when Create Alliance is clicked', async () => {
    render(<Alliance />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /create alliance/i }));
    await user.click(screen.getByRole('button', { name: /create alliance/i }));
    expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/tag/i)).toBeInTheDocument();
  });

  it('calls createAlliance with the form data on submit', async () => {
    vi.mocked(alliancesEndpoints.createAlliance).mockResolvedValue({ alliance: mockAlliance });
    vi.mocked(profileEndpoints.getMyAlliance)
      .mockRejectedValueOnce(new Error('not_found'))
      .mockResolvedValue({ membership: mockMembership });
    vi.mocked(alliancesEndpoints.getAlliance).mockResolvedValue({ alliance: mockAlliance });
    vi.mocked(alliancesEndpoints.getMembers).mockResolvedValue({ members: mockMembers });

    render(<Alliance />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /create alliance/i }));
    await user.click(screen.getByRole('button', { name: /create alliance/i }));
    await user.type(screen.getByLabelText(/name/i), 'Tower Lords');
    await user.type(screen.getByLabelText(/tag/i), 'TL');
    await user.click(screen.getByRole('button', { name: /create alliance$/i }));

    await waitFor(() => {
      expect(alliancesEndpoints.createAlliance).toHaveBeenCalledWith({
        name: 'Tower Lords',
        tag: 'TL',
        description: '',
      });
    });
  });

  it('shows an error when alliance creation fails', async () => {
    vi.mocked(alliancesEndpoints.createAlliance).mockRejectedValue(new Error('Name taken.'));
    render(<Alliance />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /create alliance/i }));
    await user.click(screen.getByRole('button', { name: /create alliance/i }));
    await user.type(screen.getByLabelText(/name/i), 'Taken Name');
    await user.type(screen.getByLabelText(/tag/i), 'TN');
    await user.click(screen.getByRole('button', { name: /create alliance$/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Name taken.');
    });
  });
});

describe('Alliance — in an alliance', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(profileEndpoints.getMyAlliance).mockResolvedValue({ membership: mockMembership });
    vi.mocked(alliancesEndpoints.getAlliance).mockResolvedValue({ alliance: mockAlliance });
    vi.mocked(alliancesEndpoints.getMembers).mockResolvedValue({ members: mockMembers });
    vi.mocked(profileEndpoints.leaveAlliance).mockResolvedValue(undefined);
  });

  it('shows the alliance name and tag', async () => {
    render(<Alliance />);
    await waitFor(() => {
      expect(screen.getByText('Tower Lords')).toBeInTheDocument();
      expect(screen.getByText('[TL]')).toBeInTheDocument();
    });
  });

  it('shows the roster sub-tab by default', async () => {
    render(<Alliance />);
    await waitFor(() => screen.getByText('Tower Lords'));
    expect(screen.getByRole('tab', { name: /roster/i })).toHaveAttribute('aria-selected', 'true');
    // Both member IDs should appear in the roster.
    expect(screen.getByText('user-2')).toBeInTheDocument();
    expect(screen.getByText('user-1')).toBeInTheDocument();
  });

  it('switches to the chat stub tab', async () => {
    render(<Alliance />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByText('Tower Lords'));
    await user.click(screen.getByRole('tab', { name: /chat/i }));
    expect(screen.getByText(/alliance chat comes in phase 15/i)).toBeInTheDocument();
  });

  it('calls leaveAlliance when Leave is clicked', async () => {
    render(<Alliance />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /leave/i }));
    await user.click(screen.getByRole('button', { name: /leave/i }));
    await waitFor(() => {
      expect(profileEndpoints.leaveAlliance).toHaveBeenCalledOnce();
    });
  });
});
