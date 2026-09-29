'use client';

import { useEffect } from 'react';

type Props = {
  title: string;
  message: string;
  confirmLabel: string;
  danger?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({ title, message, confirmLabel, danger, busy, onConfirm, onCancel }: Props) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) onCancel(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [busy, onCancel]);

  return <div className="modal-backdrop" onClick={() => !busy && onCancel()}>
    <div className="modal" role="dialog" aria-modal="true" aria-labelledby="confirm-title" onClick={event => event.stopPropagation()}>
      <h3 id="confirm-title">{title}</h3>
      <p>{message}</p>
      <div className="modal-actions">
        <button className="secondary" onClick={onCancel} disabled={busy}>Cancel</button>
        <button className={danger ? 'danger-solid' : ''} onClick={onConfirm} disabled={busy} autoFocus>{busy ? 'Working…' : confirmLabel}</button>
      </div>
    </div>
  </div>;
}
