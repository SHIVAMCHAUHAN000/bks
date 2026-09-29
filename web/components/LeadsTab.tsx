'use client';

import { FormEvent, useMemo, useState } from 'react';
import { api, errorMessage } from '../lib/api';
import type { DashboardStats, Lead, LeadInput } from '../lib/types';
import { ConfirmDialog } from './ConfirmDialog';
import { LeadTable, leadStatus } from './LeadTable';

type Props = {
  stats: DashboardStats;
  leads: Lead[];
  onChanged: (message: string) => Promise<void>;
  onError: (message: string) => void;
  onSelect: (lead: Lead) => void;
  onLeadsChange: (update: (leads: Lead[]) => Lead[]) => void;
};

const PAGE_SIZE = 50;
const statCards: Array<[keyof DashboardStats, string]> = [['total', 'All leads'], ['emailed', 'Emailed'], ['opened', 'Opened'], ['replied', 'Replied'], ['invalid', 'Invalid / bounced'], ['unsubscribed', 'Unsubscribed']];

export function LeadsTab({ stats, leads, onChanged, onError, onSelect, onLeadsChange }: Props) {
  const [query, setQuery] = useState('');
  const [segment, setSegment] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [pendingDelete, setPendingDelete] = useState<Lead[] | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [adding, setAdding] = useState(false);
  const [formError, setFormError] = useState('');

  const segments = useMemo(() => Array.from(new Set(leads.map(lead => lead.segment).filter(Boolean))).sort(), [leads]);
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return leads.filter(lead => {
      if (segment && lead.segment !== segment) return false;
      if (status && leadStatus(lead) !== status) return false;
      if (!needle) return true;
      return [lead.name, lead.organization, lead.email, lead.phone, lead.designation, lead.address].some(value => value.toLowerCase().includes(needle));
    });
  }, [leads, query, segment, status]);
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pages - 1);
  const visible = filtered.slice(current * PAGE_SIZE, (current + 1) * PAGE_SIZE);

  async function addLead(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    // Keep a reference: React clears event.currentTarget once the handler awaits.
    const form = event.currentTarget;
    const payload = Object.fromEntries(new FormData(form)) as LeadInput;
    setAdding(true); setFormError('');
    try {
      const created = await api.createLead(payload);
      onLeadsChange(list => [created, ...list.filter(item => item.id !== created.id)]);
      form.reset();
      (form.elements.namedItem('name') as HTMLInputElement | null)?.focus();
      await onChanged(`Added ${created.name || created.email || created.phone}`);
    } catch (caught) { setFormError(errorMessage(caught)); }
    finally { setAdding(false); }
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      const ids = pendingDelete.map(lead => lead.id);
      const result = ids.length === 1 ? await api.deleteLead(ids[0]) : await api.deleteLeads(ids);
      onLeadsChange(list => list.filter(lead => !ids.includes(lead.id)));
      setSelected(previous => { const next = new Set(previous); ids.forEach(id => next.delete(id)); return next; });
      setPendingDelete(null);
      await onChanged(`Deleted ${result.deleted} lead${result.deleted === 1 ? '' : 's'}`);
    } catch (caught) { onError(errorMessage(caught)); setPendingDelete(null); }
    finally { setDeleting(false); }
  }

  function toggle(id: number) {
    setSelected(previous => { const next = new Set(previous); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  }
  function togglePage(checked: boolean) {
    setSelected(previous => { const next = new Set(previous); visible.forEach(lead => checked ? next.add(lead.id) : next.delete(lead.id)); return next; });
  }

  return <>
    <div className="stats">{statCards.map(([key, label]) => <div className="card" key={key}><b>{stats[key]}</b><small>{label}</small></div>)}</div>

    <form className="add-lead" onSubmit={addLead}>
      <div className="add-lead-title"><b>Add a lead</b><small>Email or phone number is required</small></div>
      <div className="add-lead-grid">
        <input name="name" placeholder="Name" autoComplete="off"/>
        <input name="organization" placeholder="Organization"/>
        <input name="email" type="email" placeholder="Email"/>
        <input name="phone" type="tel" placeholder="Phone number"/>
        <input name="designation" placeholder="Designation"/>
        <input name="address" placeholder="Address"/>
        <input name="segment" placeholder="Segment (optional)" list="segment-options"/>
        <button disabled={adding}>{adding ? 'Adding…' : 'Add lead'}</button>
      </div>
      <datalist id="segment-options">{segments.map(value => <option key={value} value={value}/>)}</datalist>
      {formError && <p className="form-error">{formError}</p>}
    </form>

    <div className="toolbar">
      <input placeholder="Search name, organization, email, phone…" value={query} onChange={event => { setQuery(event.target.value); setPage(0); }}/>
      <select value={segment} onChange={event => { setSegment(event.target.value); setPage(0); }} aria-label="Segment"><option value="">All segments</option>{segments.map(value => <option key={value}>{value}</option>)}</select>
      <select value={status} onChange={event => { setStatus(event.target.value); setPage(0); }} aria-label="Status"><option value="">All statuses</option>{['Uncontacted', 'Sent', 'Opened', 'Replied', 'Invalid', 'Unsubscribed'].map(value => <option key={value}>{value}</option>)}</select>
      <span className="toolbar-count">{filtered.length} of {leads.length}</span>
    </div>

    {selected.size > 0 && <div className="bulk-bar"><span><b>{selected.size}</b> selected</span><button className="secondary compact" onClick={() => setSelected(new Set())}>Clear</button><button className="danger compact" onClick={() => setPendingDelete(leads.filter(lead => selected.has(lead.id)))}>Delete selected</button></div>}

    <LeadTable leads={visible} selected={selected} onToggle={toggle} onTogglePage={togglePage} onSelect={onSelect} onDelete={lead => setPendingDelete([lead])}/>

    {pages > 1 && <div className="pager"><button className="secondary compact" disabled={current === 0} onClick={() => setPage(current - 1)}>Previous</button><span>Page {current + 1} of {pages}</span><button className="secondary compact" disabled={current >= pages - 1} onClick={() => setPage(current + 1)}>Next</button></div>}

    {pendingDelete && <ConfirmDialog danger busy={deleting}
      title={pendingDelete.length === 1 ? 'Delete this lead?' : `Delete ${pendingDelete.length} leads?`}
      message={pendingDelete.length === 1 ? `${pendingDelete[0].name || pendingDelete[0].email || pendingDelete[0].phone} and their email history will be removed permanently.` : 'These leads and their email history will be removed permanently.'}
      confirmLabel="Delete" onConfirm={confirmDelete} onCancel={() => setPendingDelete(null)}/>}
  </>;
}
