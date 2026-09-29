'use client';

import { useCallback, useEffect, useState } from 'react';
import { api, errorMessage, formatDate } from '../lib/api';
import type { Lead, LeadEvent } from '../lib/types';

type Props = { lead: Lead; onChanged: (message: string) => void | Promise<void>; refreshKey?: number };

const statusText: Record<string, string> = {
  delivered: 'Delivered',
  opened: 'Opened',
  bounced: 'Bounced — lead marked invalid',
  failed: 'Failed to send',
  unsubscribed: 'Unsubscribed',
};

// Conversation shows every email sent to a lead and every reply, oldest first.
export function Conversation({ lead, onChanged, refreshKey }: Props) {
  const [events, setEvents] = useState<LeadEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [reply, setReply] = useState('');
  const [error, setError] = useState('');
  const [working, setWorking] = useState(false);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  const load = useCallback(async () => {
    try { setEvents(await api.events(lead.id)); setError(''); }
    catch (caught) { setError(errorMessage(caught)); }
    finally { setLoading(false); }
  }, [lead.id]);
  useEffect(() => { setLoading(true); setExpanded(new Set()); load(); }, [load, refreshKey]);

  async function saveReply() {
    if (!reply.trim()) return;
    setWorking(true); setError('');
    try { await api.recordReply(lead.id, reply); setReply(''); await load(); await onChanged('Reply recorded'); }
    catch (caught) { setError(errorMessage(caught)); } finally { setWorking(false); }
  }

  const messages = events.filter(event => event.kind === 'initial' || event.kind === 'followup' || event.kind === 'reply');
  const latestID = messages.length ? messages[messages.length - 1].id : 0;

  return <div className="conversation">
    {loading ? <p className="muted">Loading conversation…</p> : events.length === 0 ? <div className="thread-empty"><b>No emails yet</b><p className="muted">Emails you send to this lead and their replies will appear here.</p></div> :
      <div className="thread">{events.map(event => {
        if (event.kind === 'initial' || event.kind === 'followup') {
          const open = expanded.has(event.id) || event.id === latestID || messages.length <= 3;
          return <article key={event.id} className="message outgoing">
            <button type="button" className="message-head" onClick={() => setExpanded(previous => { const next = new Set(previous); if (next.has(event.id)) next.delete(event.id); else next.add(event.id); return next; })}>
              <span className="message-tag">{event.kind === 'initial' ? 'Sent' : 'Follow-up'}</span>
              <b>{event.subject || '(no subject)'}</b>
              <small>{formatDate(event.createdAt)}</small>
            </button>
            <div className="message-meta">{event.from ? `From ${event.from} · ` : ''}To {event.to || lead.email}</div>
            {open ? <div className="message-body">{event.body || event.content}</div> : <div className="message-snippet">{(event.body || '').slice(0, 140)}{(event.body || '').length > 140 ? '…' : ''}</div>}
          </article>;
        }
        if (event.kind === 'reply') {
          return <article key={event.id} className="message incoming">
            <div className="message-head static"><span className="message-tag reply-tag">Reply</span><b>{event.subject || 'Reply'}</b><small>{formatDate(event.createdAt)}</small></div>
            <div className="message-meta">From {event.from || lead.email}{event.from ? '' : ' (recorded manually)'}</div>
            <div className="message-body">{event.content}</div>
          </article>;
        }
        return <div key={event.id} className={`thread-status ${event.kind}`}><span/>{statusText[event.kind] ?? event.kind}{event.kind === 'bounced' || event.kind === 'failed' ? `: ${event.content}` : ''}<small>{formatDate(event.createdAt)}</small></div>;
      })}</div>}
    {error && <p className="form-error">{error}</p>}
    <div className="manual-reply">
      <label htmlFor={`reply-${lead.id}`}>Record a reply manually</label>
      <textarea id={`reply-${lead.id}`} value={reply} onChange={event => setReply(event.target.value)} placeholder="Paste a reply that arrived outside the Inbox (phone, WhatsApp, another mailbox)…"/>
      <button disabled={working || !reply.trim()} onClick={saveReply}>Save reply</button>
    </div>
  </div>;
}
