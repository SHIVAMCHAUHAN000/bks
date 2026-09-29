'use client';

import { useCallback, useEffect, useState } from 'react';
import { api, errorMessage, formatDate } from '../lib/api';
import type { Conversation as ConversationRow } from '../lib/types';
import { Conversation } from './Conversation';
import { LeadFacts } from './LeadDetails';

type Props = { onChanged: (message: string) => Promise<void>; onError: (message: string) => void };

export function InboxTab({ onChanged, onError }: Props) {
  const [filter, setFilter] = useState<'all' | 'replied'>('all');
  const [query, setQuery] = useState('');
  const [rows, setRows] = useState<ConversationRow[]>([]);
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshKey, setRefreshKey] = useState(0);

  const load = useCallback(async () => {
    try { setRows(await api.conversations(filter, query)); }
    catch (caught) { onError(errorMessage(caught)); }
    finally { setLoading(false); }
  }, [filter, query, onError]);

  useEffect(() => { const timer = setTimeout(load, 200); return () => clearTimeout(timer); }, [load]);
  // Replies arrive through the Resend webhook, so check for new ones regularly.
  useEffect(() => { const timer = setInterval(() => { load(); setRefreshKey(key => key + 1); }, 30000); return () => clearInterval(timer); }, [load]);

  const selected = rows.find(row => row.lead.id === selectedID) ?? null;
  async function changed(message: string) { await onChanged(message); await load(); }

  return <div className={selected ? 'inbox thread-open' : 'inbox'}>
    <section className="inbox-list">
      <div className="inbox-tools">
        <div className="segmented" role="tablist">
          <button role="tab" aria-selected={filter === 'all'} className={filter === 'all' ? 'active' : ''} onClick={() => setFilter('all')}>All sent</button>
          <button role="tab" aria-selected={filter === 'replied'} className={filter === 'replied' ? 'active' : ''} onClick={() => setFilter('replied')}>Replied</button>
        </div>
        <button className="secondary compact" onClick={() => { load(); setRefreshKey(key => key + 1); }}>Refresh</button>
      </div>
      <input className="inbox-search" placeholder="Search name, organization or email" value={query} onChange={event => setQuery(event.target.value)}/>
      {loading ? <p className="muted pad">Loading…</p> : rows.length === 0 ? <div className="empty">{filter === 'replied' ? 'No replies yet.' : 'No emails have been sent yet. Start a campaign from Automation.'}</div> :
        <ul>{rows.map(row => <li key={row.lead.id}>
          <button className={row.lead.id === selectedID ? 'inbox-row active' : 'inbox-row'} onClick={() => setSelectedID(row.lead.id)}>
            <div className="inbox-row-top"><b>{row.lead.name || row.lead.email}</b><small>{formatDate(row.lastAt)}</small></div>
            <div className="inbox-row-org">{[row.lead.organization, row.lead.designation].filter(Boolean).join(' · ') || row.lead.email}</div>
            <div className="inbox-row-snippet">{row.lastKind === 'reply' ? <span className="reply-dot">Reply:</span> : <span className="sent-dot">You:</span>} {row.lastSnippet || row.lastSubject}</div>
            <div className="inbox-row-meta">{row.messages} message{row.messages === 1 ? '' : 's'}{row.replies ? <i className="reply">{row.replies} repl{row.replies === 1 ? 'y' : 'ies'}</i> : null}{row.lead.isInvalid ? <i className="bad">Bounced</i> : null}{row.lead.isOpened && !row.replies ? <i>Opened</i> : null}</div>
          </button>
        </li>)}</ul>}
    </section>
    <section className="inbox-thread">
      {selected ? <>
        <div className="thread-heading">
          <button className="secondary compact back-button" onClick={() => setSelectedID(null)}>← Back</button>
          <div><h2>{selected.lead.name || selected.lead.email}</h2><p>{[selected.lead.designation, selected.lead.organization].filter(Boolean).join(' · ')}</p></div>
        </div>
        <LeadFacts lead={selected.lead} onChanged={changed}/>
        <Conversation lead={selected.lead} onChanged={changed} refreshKey={refreshKey}/>
      </> : <div className="thread-placeholder"><b>Select a conversation</b><p className="muted">Click an email on the left to see what was sent and any reply.</p></div>}
    </section>
  </div>;
}
