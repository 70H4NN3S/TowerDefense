import { useState, useEffect } from 'react';
import { useEvents } from '@/hooks/useEvents.ts';
import type { GameEvent } from '@/api/types.ts';

/* ─── Countdown helpers ─────────────────────────────────────────────────── */

function formatDuration(msLeft: number): string {
  if (msLeft <= 0) return 'Ended';
  const totalSeconds = Math.floor(msLeft / 1000);
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m ${seconds}s`;
}

interface CountdownResult {
  display: string;
  isUpcoming: boolean;
  isEnded: boolean;
}

/**
 * Tracks the current time in state (updated every second) and derives
 * countdown display, upcoming, and ended flags from it.
 * Using state for `now` keeps `Date.now()` out of the render body,
 * satisfying the react-hooks/purity rule.
 */
function useCountdown(startsAt: string, endsAt: string): CountdownResult {
  // Lazy initializer so Date.now() runs only once per mount, not on re-renders.
  const [now, setNow] = useState<number>(() => Date.now());

  useEffect(() => {
    const id = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => clearInterval(id);
  }, []);

  const startMs = new Date(startsAt).getTime();
  const endMs = new Date(endsAt).getTime();
  const isUpcoming = startMs > now;
  const isEnded = endMs <= now;

  if (isEnded) {
    return { display: 'Ended', isUpcoming: false, isEnded: true };
  }

  const targetMs = isUpcoming ? startMs : endMs;
  const msLeft = targetMs - now;
  const prefix = isUpcoming ? 'Starts in ' : '';
  return { display: `${prefix}${formatDuration(msLeft)}`, isUpcoming, isEnded: false };
}

/* ─── Event card ────────────────────────────────────────────────────────── */

interface EventCardProps {
  event: GameEvent;
  onClaim: (tier: number) => Promise<void>;
}

function EventCard({ event, onClaim }: EventCardProps) {
  const { display, isUpcoming } = useCountdown(event.starts_at, event.ends_at);
  const [isClaiming, setIsClaiming] = useState(false);
  const [claimError, setClaimError] = useState<string | null>(null);

  async function handleClaim() {
    setClaimError(null);
    setIsClaiming(true);
    try {
      // Claim tier 0 (first reward tier) as default.
      // Full tier UI requires backend progress data; tracked in followups.md.
      await onClaim(0);
    } catch (err) {
      setClaimError(err instanceof Error ? err.message : 'Failed to claim reward.');
    } finally {
      setIsClaiming(false);
    }
  }

  return (
    <div className="event-card">
      <div className="event-card__header">
        <h2 className="event-card__name">{event.name}</h2>
        <span className="event-countdown" aria-label="Time remaining">
          {display}
        </span>
      </div>
      <p className="event-card__description">{event.description}</p>
      {/* Progress bar is a placeholder until per-user progress is exposed by the API. */}
      <div className="event-progress">
        <div className="event-progress__bar-track" aria-hidden="true">
          <div className="event-progress__bar-fill" style={{ width: '0%' }} />
        </div>
        <span className="event-progress__label">Complete matches to earn progress</span>
      </div>
      {claimError !== null && (
        <p role="alert" className="screen-error">
          {claimError}
        </p>
      )}
      <button
        className="btn-primary"
        onClick={handleClaim}
        disabled={isClaiming || isUpcoming}
        aria-label={`Claim reward for ${event.name}`}
      >
        {isUpcoming ? 'Not started' : isClaiming ? 'Claiming…' : 'Claim Reward'}
      </button>
    </div>
  );
}

/* ─── Events screen ─────────────────────────────────────────────────────── */

/**
 * Events screen — active and upcoming events with progress bars and
 * countdown timers.
 */
export function Events() {
  const { events, isLoading, error, claim } = useEvents();

  if (isLoading) {
    return (
      <div className="screen-empty" aria-busy="true">
        Loading events…
      </div>
    );
  }

  if (error !== null) {
    return (
      <p className="screen-error" role="alert">
        {error}
      </p>
    );
  }

  if (events.length === 0) {
    return (
      <div className="screen-empty">
        <p>No active events right now.</p>
        <p>Check back soon!</p>
      </div>
    );
  }

  return (
    <>
      <div className="screen-header">
        <h1 className="screen-title">Events</h1>
      </div>
      <div className="events-list">
        {events.map((event) => (
          <EventCard key={event.id} event={event} onClaim={(tier) => claim(event.id, tier)} />
        ))}
      </div>
    </>
  );
}
