import { useState, type ReactNode } from 'react';

interface ConfirmModalProps {
  title: string;
  children: ReactNode;
  confirmLabel: string;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
}

/**
 * Generic bottom-sheet confirmation modal.
 * Shows a title, arbitrary body content, and Confirm / Cancel actions.
 * Handles the in-flight state internally.
 */
export function ConfirmModal({
  title,
  children,
  confirmLabel,
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  const [isConfirming, setIsConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleConfirm() {
    setError(null);
    setIsConfirming(true);
    try {
      await onConfirm();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong.');
      setIsConfirming(false);
    }
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-label={title}>
      <div className="modal-sheet">
        <h2 className="modal-title">{title}</h2>
        <div className="modal-subtitle">{children}</div>
        {error !== null && (
          <p role="alert" className="screen-error">
            {error}
          </p>
        )}
        <div className="modal-actions">
          <button className="btn-secondary" onClick={onCancel} disabled={isConfirming}>
            Cancel
          </button>
          <button className="btn-primary" onClick={handleConfirm} disabled={isConfirming}>
            {isConfirming ? 'Processing…' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
