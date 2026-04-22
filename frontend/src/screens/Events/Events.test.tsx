import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as eventsEndpoints from '@/api/endpoints/events.ts';
import { Events } from './index.tsx';

vi.mock('@/api/endpoints/events.ts');

const futureDate = new Date(Date.now() + 7 * 24 * 3600 * 1000).toISOString();
const activeEndDate = new Date(Date.now() + 2 * 24 * 3600 * 1000).toISOString();
const pastDate = new Date(Date.now() - 1000).toISOString();

const mockActiveEvent = {
  id: 'event-1',
  kind: 'kill_n_monsters',
  name: 'Monster Slayer',
  description: 'Kill 100 monsters.',
  starts_at: new Date(Date.now() - 3600 * 1000).toISOString(),
  ends_at: activeEndDate,
  config: { target: 100 },
};

const mockUpcomingEvent = {
  id: 'event-2',
  kind: 'kill_n_monsters',
  name: 'Future Challenge',
  description: 'Coming soon.',
  starts_at: futureDate,
  ends_at: new Date(Date.now() + 14 * 24 * 3600 * 1000).toISOString(),
  config: { target: 50 },
};

describe('Events', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('shows a loading state initially', () => {
    vi.mocked(eventsEndpoints.getActive).mockReturnValue(new Promise(() => {}));
    render(<Events />);
    expect(screen.getByText(/loading events/i)).toBeInTheDocument();
  });

  it('shows an empty state when there are no events', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [] });
    render(<Events />);
    await waitFor(() => {
      expect(screen.getByText(/no active events/i)).toBeInTheDocument();
    });
  });

  it('renders event cards with name and description', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [mockActiveEvent] });
    render(<Events />);
    await waitFor(() => {
      expect(screen.getByText('Monster Slayer')).toBeInTheDocument();
      expect(screen.getByText('Kill 100 monsters.')).toBeInTheDocument();
    });
  });

  it('shows the Claim Reward button for active events', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [mockActiveEvent] });
    render(<Events />);
    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /claim reward for monster slayer/i }),
      ).toBeInTheDocument();
    });
  });

  it('disables the claim button for upcoming events', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [mockUpcomingEvent] });
    render(<Events />);
    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /claim reward for future challenge/i }),
      ).toBeDisabled();
    });
  });

  it('calls claimTier when the Claim button is clicked', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [mockActiveEvent] });
    vi.mocked(eventsEndpoints.claimTier).mockResolvedValue({ rewards: { gold: 500 } });
    render(<Events />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /claim reward for monster slayer/i }));
    await user.click(screen.getByRole('button', { name: /claim reward for monster slayer/i }));
    await waitFor(() => {
      expect(eventsEndpoints.claimTier).toHaveBeenCalledWith('event-1', 0);
    });
  });

  it('shows an error when claiming fails', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [mockActiveEvent] });
    vi.mocked(eventsEndpoints.claimTier).mockRejectedValue(new Error('Already claimed.'));
    render(<Events />);
    const user = userEvent.setup();
    await waitFor(() => screen.getByRole('button', { name: /claim reward for monster slayer/i }));
    await user.click(screen.getByRole('button', { name: /claim reward for monster slayer/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Already claimed.');
    });
  });

  it('shows an error when fetching events fails', async () => {
    vi.mocked(eventsEndpoints.getActive).mockRejectedValue(new Error('Server error'));
    render(<Events />);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Server error');
    });
  });

  it('shows a countdown for active events', async () => {
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [mockActiveEvent] });
    render(<Events />);
    await waitFor(() => {
      // Should show remaining time (e.g. "1d 23h" for ~2 days remaining)
      expect(screen.getByLabelText('Time remaining')).toBeInTheDocument();
    });
  });

  it('shows the ended state for past events', async () => {
    const endedEvent = { ...mockActiveEvent, ends_at: pastDate };
    vi.mocked(eventsEndpoints.getActive).mockResolvedValue({ events: [endedEvent] });
    render(<Events />);
    await waitFor(() => {
      expect(screen.getByText('Ended')).toBeInTheDocument();
    });
  });
});
