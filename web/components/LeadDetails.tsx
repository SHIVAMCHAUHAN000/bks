'use client';

import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import type { Lead, LeadEvent } from '../lib/types';

type Props = { lead: Lead; onClose: () => void; onChanged: (message: string) => void };

export function LeadDetails({ lead, onClose, onChanged }: Props) {
  const [events, setEvents] = useState<LeadEvent[]>([]);
  const [reply, setReply] = useState('');
  const [error, setError] = useState('');
  const [working, setWorking] = useState(false);

  async function loadEvents() { try { setEvents(await api.events(lead.id)); } catch (e) { setError(errorMessage(e)); } }
  useEffect(() => { loadEvents(); }, [lead.id]);

  async function saveReply() {
    if (!reply.trim()) return;
    setWorking(true); setError('');
    try { await api.recordReply(lead.id, reply); setReply(''); await loadEvents(); onChanged('Reply recorded'); }
    catch (e) { setError(errorMessage(e)); } finally { setWorking(false); }
  }
  async function invalidate() {
    setWorking(true); setError('');
    try { await api.markInvalid(lead.id); onChanged('Lead marked invalid'); }
    catch (e) { setError(errorMessage(e)); } finally { setWorking(false); }
  }

  return <aside className="detail-panel"><div className="detail-heading"><div><p className="eyebrow">LEAD DETAILS</p><h2>{lead.name || 'Unnamed lead'}</h2><p>{lead.email || lead.phone}</p></div><button className="icon-button" onClick={onClose} aria-label="Close details">×</button></div>
    <div className="detail-meta"><span>{lead.category || 'Uncategorized'}</span>{lead.isInvalid ? <i className="bad">Invalid</i> : <button className="danger compact" disabled={working} onClick={invalidate}>Mark invalid</button>}</div>
    <h3>Record reply</h3><textarea value={reply} onChange={event => setReply(event.target.value)} placeholder="Paste or summarize the reply…"/><button disabled={working || !reply.trim()} onClick={saveReply}>Save reply</button>
    {error && <p className="form-error">{error}</p>}
    <h3>Activity</h3><div className="timeline">{events.length ? events.map(event => <article key={event.id}><b>{event.kind}</b><small>{formatDate(event.createdAt)}</small>{event.subject && <p>{event.subject}</p>}{event.body && <p>{event.body}</p>}{event.content && <p>{event.content}</p>}</article>) : <p className="muted">No activity yet.</p>}</div>
  </aside>;
}

function errorMessage(error: unknown) { return error instanceof Error ? error.message : 'Request failed'; }
function formatDate(value: string) { return new Date(value.replace(' ', 'T') + 'Z').toLocaleString(); }
