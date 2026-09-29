'use client';

import { useCallback, useEffect, useState } from 'react';
import { AutomationTab } from '../components/AutomationTab';
import { ImportTab } from '../components/ImportTab';
import { InboxTab } from '../components/InboxTab';
import { LeadDetails } from '../components/LeadDetails';
import { LeadsTab } from '../components/LeadsTab';
import { api, errorMessage } from '../lib/api';
import type { DashboardStats, Lead } from '../lib/types';

type Tab = 'leads' | 'import' | 'automation' | 'inbox';
const emptyStats: DashboardStats = { total: 0, emailed: 0, opened: 0, replied: 0, invalid: 0, unsubscribed: 0, sentToday: 0 };
const titles: Record<Tab, string> = { leads: 'Your lead pipeline', import: 'Import leads', automation: 'Email automation', inbox: 'Inbox' };

export default function Home() {
  const [leads, setLeads] = useState<Lead[]>([]);
  const [stats, setStats] = useState<DashboardStats>(emptyStats);
  const [tab, setTab] = useState<Tab>('leads');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [selectedLead, setSelectedLead] = useState<Lead | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [nextLeads, nextStats] = await Promise.all([api.listLeads(), api.dashboard()]);
      setLeads(nextLeads); setStats(nextStats);
      setSelectedLead(current => current ? nextLeads.find(item => item.id === current.id) ?? null : null);
    } catch (caught) { setError(errorMessage(caught)); }
  }, []);
  useEffect(() => { refresh(); }, [refresh]);
  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(''), 4000);
    return () => clearTimeout(timer);
  }, [notice]);

  const changed = useCallback(async (message: string) => { setNotice(message); await refresh(); }, [refresh]);
  function openTab(next: Tab) { setTab(next); setSelectedLead(null); setError(''); }

  return <main>
    <nav>
      <div className="brand">automation<span>tool</span></div>
      <NavButton active={tab === 'leads'} onClick={() => openTab('leads')} icon="◎">Leads</NavButton>
      <NavButton active={tab === 'import'} onClick={() => openTab('import')} icon="⇪">Import</NavButton>
      <NavButton active={tab === 'automation'} onClick={() => openTab('automation')} icon="✉">Automation</NavButton>
      <NavButton active={tab === 'inbox'} onClick={() => openTab('inbox')} icon="☰" badge={stats.replied}>Inbox</NavButton>
    </nav>
    <section className={selectedLead ? 'content details-visible' : 'content'}>
      <header><div><p className="eyebrow">OPERATIONS</p><h1>{titles[tab]}</h1></div>{notice && <p className="notice" role="status">{notice}</p>}</header>
      {error && <div className="error-banner" role="alert">{error}<button className="icon-button" onClick={() => setError('')} aria-label="Dismiss error">×</button></div>}
      {tab === 'leads' && <LeadsTab stats={stats} leads={leads} onChanged={changed} onError={setError} onSelect={setSelectedLead} onLeadsChange={setLeads}/>}
      {tab === 'import' && <ImportTab onImported={changed} onError={setError}/>}
      {tab === 'automation' && <AutomationTab leads={leads} onChanged={changed} onError={setError}/>}
      {tab === 'inbox' && <InboxTab onChanged={changed} onError={setError}/>}
    </section>
    {selectedLead && tab === 'leads' && <LeadDetails lead={selectedLead} onClose={() => setSelectedLead(null)} onChanged={changed}/>}
  </main>;
}

function NavButton({ active, onClick, children, icon, badge }: { active: boolean; onClick: () => void; children: string; icon: string; badge?: number }) {
  return <button className={active ? 'active' : ''} onClick={onClick} title={children}>
    <span className="nav-icon" aria-hidden="true">{icon}</span><span className="nav-label">{children}</span>
    {badge ? <span className="nav-badge">{badge}</span> : null}
  </button>;
}
