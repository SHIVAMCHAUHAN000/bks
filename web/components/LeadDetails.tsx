'use client';

import { useState } from 'react';
import { api, errorMessage } from '../lib/api';
import type { Lead } from '../lib/types';
import { Conversation } from './Conversation';
import { LeadStatus } from './LeadTable';

type Props = { lead: Lead; onClose: () => void; onChanged: (message: string) => void | Promise<void> };

export function LeadDetails({ lead, onClose, onChanged }: Props) {
  return <aside className="detail-panel">
    <div className="detail-heading"><div><p className="eyebrow">LEAD DETAILS</p><h2>{lead.name || 'Unnamed lead'}</h2><p>{[lead.designation, lead.organization].filter(Boolean).join(' · ') || lead.email || lead.phone}</p></div><button className="icon-button" onClick={onClose} aria-label="Close details">×</button></div>
    <LeadFacts lead={lead} onChanged={onChanged}/>
    <h3>Conversation</h3>
    <Conversation lead={lead} onChanged={onChanged}/>
  </aside>;
}

export function LeadFacts({ lead, onChanged }: { lead: Lead; onChanged: (message: string) => void | Promise<void> }) {
  const [working, setWorking] = useState(false);
  const [error, setError] = useState('');
  async function invalidate() {
    setWorking(true); setError('');
    try { await api.markInvalid(lead.id); await onChanged('Lead marked invalid'); }
    catch (caught) { setError(errorMessage(caught)); } finally { setWorking(false); }
  }
  const facts: Array<[string, string]> = [['Email', lead.email], ['Phone', lead.phone], ['Organization', lead.organization], ['Designation', lead.designation], ['Address', lead.address], ['Segment', lead.segment]];
  return <>
    <dl className="lead-facts">{facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value || '—'}</dd></div>)}</dl>
    <div className="detail-meta"><LeadStatus lead={lead}/>{!lead.isInvalid && <button className="danger compact" disabled={working} onClick={invalidate}>Mark invalid</button>}</div>
    {error && <p className="form-error">{error}</p>}
  </>;
}
