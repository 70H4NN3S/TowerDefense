import { useState } from 'react';
import { useAlliance } from '@/hooks/useAlliance.ts';
import { leaveAlliance } from '@/api/endpoints/profile.ts';
import type { Alliance, AllianceMember } from '@/api/types.ts';

/* ─── Sub-views ─────────────────────────────────────────────────────────── */

type AllianceSubTab = 'roster' | 'chat' | 'events';

interface AllianceViewProps {
  alliance: Alliance;
  members: AllianceMember[];
  onLeave: () => void;
}

function RosterTab({ members }: { members: AllianceMember[] }) {
  return (
    <div className="alliance-roster">
      {members.map((m) => (
        <div key={m.user_id} className="member-row">
          <span className="member-row__name">{m.user_id}</span>
          <span className="member-role">{m.role}</span>
        </div>
      ))}
    </div>
  );
}

function AllianceView({ alliance, members, onLeave }: AllianceViewProps) {
  const [subTab, setSubTab] = useState<AllianceSubTab>('roster');

  return (
    <>
      <div className="screen-header">
        <div>
          <h1 className="screen-title">{alliance.name}</h1>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-secondary)' }}>
            [{alliance.tag}]
          </p>
        </div>
        <button
          className="btn-secondary"
          onClick={onLeave}
          style={{ padding: 'var(--space-2) var(--space-3)', fontSize: 'var(--text-sm)' }}
        >
          Leave
        </button>
      </div>

      <div className="alliance-tabs" role="tablist">
        {(['roster', 'chat', 'events'] as AllianceSubTab[]).map((tab) => (
          <button
            key={tab}
            role="tab"
            aria-selected={subTab === tab}
            className={`alliance-tab${subTab === tab ? ' alliance-tab--active' : ''}`}
            onClick={() => setSubTab(tab)}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
          </button>
        ))}
      </div>

      {subTab === 'roster' && <RosterTab members={members} />}
      {subTab === 'chat' && (
        <div className="screen-empty">
          <p>Alliance chat comes in Phase 15.</p>
        </div>
      )}
      {subTab === 'events' && (
        <div className="screen-empty">
          <p>Alliance events come in a future phase.</p>
        </div>
      )}
    </>
  );
}

/* ─── No-alliance view ──────────────────────────────────────────────────── */

interface CreateFormState {
  name: string;
  tag: string;
  description: string;
}

interface NoAllianceViewProps {
  onCreate: (name: string, tag: string, description: string) => Promise<void>;
}

function NoAllianceView({ onCreate }: NoAllianceViewProps) {
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<CreateFormState>({ name: '', tag: '', description: '' });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);
    try {
      await onCreate(form.name.trim(), form.tag.trim(), form.description.trim());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create alliance.');
    } finally {
      setIsSubmitting(false);
    }
  }

  if (showCreate) {
    return (
      <>
        <div className="screen-header">
          <h1 className="screen-title">Create Alliance</h1>
          <button
            className="btn-secondary"
            onClick={() => setShowCreate(false)}
            style={{ padding: 'var(--space-2) var(--space-3)', fontSize: 'var(--text-sm)' }}
          >
            Back
          </button>
        </div>
        <form
          onSubmit={handleCreate}
          style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}
        >
          <div className="form-field">
            <label className="form-label" htmlFor="alliance-name">
              Name
            </label>
            <input
              id="alliance-name"
              className="form-input"
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              maxLength={24}
              required
            />
          </div>
          <div className="form-field">
            <label className="form-label" htmlFor="alliance-tag">
              Tag (2–4 chars)
            </label>
            <input
              id="alliance-tag"
              className="form-input"
              value={form.tag}
              onChange={(e) => setForm((f) => ({ ...f, tag: e.target.value }))}
              maxLength={4}
              required
            />
          </div>
          <div className="form-field">
            <label className="form-label" htmlFor="alliance-description">
              Description
            </label>
            <textarea
              id="alliance-description"
              className="form-input form-textarea"
              value={form.description}
              onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              maxLength={200}
            />
          </div>
          {error !== null && (
            <p role="alert" className="screen-error">
              {error}
            </p>
          )}
          <button type="submit" className="btn-primary" disabled={isSubmitting}>
            {isSubmitting ? 'Creating…' : 'Create Alliance'}
          </button>
        </form>
      </>
    );
  }

  return (
    <>
      <div className="screen-header">
        <h1 className="screen-title">Alliance</h1>
      </div>
      <div className="screen-empty">
        <p>You are not in an alliance.</p>
      </div>
      <div className="alliance-join-options">
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          Create Alliance
        </button>
        {/* TODO(claude, 2026-04-22): browse/join existing alliances; see docs/followups.md#alliance-browse */}
        <button className="btn-secondary" disabled>
          Browse Alliances (coming soon)
        </button>
      </div>
    </>
  );
}

/* ─── Alliance screen ───────────────────────────────────────────────────── */

/**
 * Alliance screen — shows either a no-alliance view (Create / Browse) or
 * the current alliance with a roster, chat, and events sub-tab.
 */
export function Alliance() {
  const { alliance, members, isLoading, error, create, refresh } = useAlliance();

  async function handleLeave() {
    await leaveAlliance();
    refresh();
  }

  if (isLoading) {
    return (
      <div className="screen-empty" aria-busy="true">
        Loading alliance…
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

  if (alliance === null) {
    return <NoAllianceView onCreate={create} />;
  }

  return <AllianceView alliance={alliance} members={members} onLeave={handleLeave} />;
}
