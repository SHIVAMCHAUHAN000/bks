'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { api, errorMessage, formatDate } from '../lib/api';
import type { CampaignPayload, CampaignPreview, CampaignRun, Lead, SendSettings } from '../lib/types';
import { ConfirmDialog } from './ConfirmDialog';

type Props = { leads: Lead[]; onChanged: (message: string) => Promise<void>; onError: (message: string) => void };

const placeholders: Array<[string, string]> = [['first_name', 'First name'], ['name', 'Full name'], ['organization', 'Organization'], ['designation', 'Designation'], ['address', 'Address']];
const starterBody = `Hi {{first_name|there}},

I came across {{organization}} and wanted to reach out to you as {{designation|a leader there}}.

[Write one or two lines about why you are contacting them.]

Would you be open to a short call next week?

Best regards,
[Your name]`;

const statusLabel: Record<string, string> = { running: 'Sending', completed: 'Completed', failed: 'Failed', daily_limit: 'Paused — daily limit', interrupted: 'Interrupted' };

export function AutomationTab({ leads, onChanged, onError }: Props) {
  const [mode, setMode] = useState<'initial' | 'followup'>('initial');
  const [segment, setSegment] = useState('');
  const [before, setBefore] = useState('');
  const [noFollowup, setNoFollowup] = useState(false);
  const [maxFollowups, setMaxFollowups] = useState('');
  const [subject, setSubject] = useState('Quick question for {{organization}}');
  const [body, setBody] = useState(starterBody);
  const [includeUnsubscribe, setIncludeUnsubscribe] = useState(true);
  const [trackOpens, setTrackOpens] = useState(false);
  const [previewLeadId, setPreviewLeadId] = useState(0);
  const [preview, setPreview] = useState<CampaignPreview | null>(null);
  const [settings, setSettings] = useState<SendSettings | null>(null);
  const [runs, setRuns] = useState<CampaignRun[]>([]);
  const [activeRun, setActiveRun] = useState<CampaignRun | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [starting, setStarting] = useState(false);
  const lastField = useRef<{ name: 'subject' | 'body'; element: HTMLInputElement | HTMLTextAreaElement } | null>(null);

  const payload: CampaignPayload = useMemo(() => ({ mode, subject, body, segment, before, noFollowup, maxFollowups: Number(maxFollowups || 0), trackOpens, includeUnsubscribe }),
    [mode, subject, body, segment, before, noFollowup, maxFollowups, trackOpens, includeUnsubscribe]);
  const segments = useMemo(() => Array.from(new Set(leads.map(lead => lead.segment).filter(Boolean))).sort(), [leads]);
  const previewChoices = useMemo(() => leads.filter(lead => lead.email && !lead.isInvalid).slice(0, 200), [leads]);

  useEffect(() => {
    api.settings().then(setSettings).catch(() => undefined);
    api.campaigns().then(list => { setRuns(list); const running = list.find(run => run.status === 'running'); if (running) setActiveRun(running); }).catch(() => undefined);
  }, []);

  // Live preview of the personalised email.
  useEffect(() => {
    const timer = setTimeout(() => { api.previewCampaign(payload, previewLeadId || undefined).then(setPreview).catch(caught => onError(errorMessage(caught))); }, 350);
    return () => clearTimeout(timer);
  }, [payload, previewLeadId, onError, leads]);

  // Poll the running campaign until it finishes.
  useEffect(() => {
    if (!activeRun || activeRun.status !== 'running') return;
    const timer = setInterval(async () => {
      try {
        const next = await api.campaign(activeRun.id);
        setActiveRun(next);
        if (next.status !== 'running') {
          setRuns(await api.campaigns());
          api.settings().then(setSettings).catch(() => undefined);
          await onChanged(`Campaign finished: ${next.sent} sent${next.failed ? `, ${next.failed} failed` : ''}`);
        }
      } catch { /* keep polling */ }
    }, 1500);
    return () => clearInterval(timer);
  }, [activeRun, onChanged]);

  function insert(tag: string) {
    const token = `{{${tag}}}`;
    const target = lastField.current;
    if (!target) { setBody(value => value + token); return; }
    const { element, name } = target;
    const start = element.selectionStart ?? element.value.length;
    const end = element.selectionEnd ?? start;
    const next = element.value.slice(0, start) + token + element.value.slice(end);
    (name === 'subject' ? setSubject : setBody)(next);
    requestAnimationFrame(() => { element.focus(); element.setSelectionRange(start + token.length, start + token.length); });
  }

  async function start() {
    setStarting(true);
    try {
      const run = await api.sendCampaign(payload);
      setActiveRun(run); setConfirming(false);
      setRuns(list => [run, ...list]);
    } catch (caught) { onError(errorMessage(caught)); setConfirming(false); }
    finally { setStarting(false); }
  }

  const eligible = preview?.eligible ?? 0;
  const sending = activeRun?.status === 'running';
  const remainingToday = settings?.dailyLimit ? Math.max(0, settings.dailyLimit - settings.sentToday) : null;
  const minutes = settings?.perMinute ? Math.ceil(eligible / settings.perMinute) : 0;
  const unknown = preview?.unknown ?? [];

  return <div className="automation-page">
    {settings && settings.warnings.length > 0 && <div className="settings-warnings">{settings.warnings.map(warning => <p key={warning}>{warning}</p>)}</div>}
    {activeRun && <CampaignProgress run={activeRun} onDismiss={activeRun.status === 'running' ? undefined : () => setActiveRun(null)}/>}

    <div className="automation-layout">
      <form className="automation-form" onSubmit={event => { event.preventDefault(); setConfirming(true); }}>
        <section className="automation-section campaign-section"><div className="section-label"><span>01</span><b>Campaign type</b></div>
          <div className="campaign-modes">
            <button type="button" className={mode === 'initial' ? 'mode-card active' : 'mode-card'} onClick={() => setMode('initial')}><strong>First email</strong><span>Leads you have not emailed yet</span></button>
            <button type="button" className={mode === 'followup' ? 'mode-card active' : 'mode-card'} onClick={() => setMode('followup')}><strong>Follow-up</strong><span>Emailed leads that have not replied</span></button>
          </div>
        </section>
        <section className="automation-section"><div className="section-label"><span>02</span><b>Audience</b></div>
          <div className="automation-table">
            <div className="automation-row"><b>Segment</b><select value={segment} onChange={event => setSegment(event.target.value)}><option value="">All segments</option>{segments.map(value => <option key={value} value={value}>{value}</option>)}</select><small>Only leads in this segment</small></div>
            <div className="automation-row"><b>Added on or before</b><input type="date" value={before} onChange={event => setBefore(event.target.value)}/><small>Leave empty for all dates</small></div>
            {mode === 'followup' && <div className="automation-row"><b>Follow-up limit</b><input type="number" min="1" placeholder="No limit" value={maxFollowups} onChange={event => setMaxFollowups(event.target.value)}/><small>Skip leads that already had this many</small></div>}
            {mode === 'followup' && <div className="automation-row"><b>Only first follow-up</b><label className="table-check"><input type="checkbox" checked={noFollowup} onChange={event => setNoFollowup(event.target.checked)}/><span>{noFollowup ? 'On' : 'Off'}</span></label><small>Skip leads already followed up</small></div>}
          </div>
        </section>
        <section className="automation-section composer-section"><div className="section-label"><span>03</span><b>Message</b></div>
          <div className="placeholder-bar"><small>Insert:</small>{placeholders.map(([tag, label]) => <button type="button" key={tag} className="chip" onMouseDown={event => event.preventDefault()} onClick={() => insert(tag)}>{label}</button>)}</div>
          <label>Subject<input required value={subject} onFocus={event => { lastField.current = { name: 'subject', element: event.currentTarget }; }} onChange={event => setSubject(event.target.value)} placeholder="Subject"/></label>
          <label>Body<textarea required value={body} onFocus={event => { lastField.current = { name: 'body', element: event.currentTarget }; }} onChange={event => setBody(event.target.value)} placeholder="Write your message…"/></label>
          <p className="hint">Use <code>{'{{first_name|there}}'}</code> to show “there” when a lead has no name. Write in plain text, keep it short and personal, and avoid ALL CAPS, many links or attachments — that keeps you out of spam folders.</p>
          {unknown.length > 0 && <p className="form-error">Unknown placeholder{unknown.length > 1 ? 's' : ''}: {unknown.map(tag => `{{${tag}}}`).join(', ')}. Use one of the Insert buttons.</p>}
          <div className="option-row">
            <label className="table-check"><input type="checkbox" checked={includeUnsubscribe} onChange={event => setIncludeUnsubscribe(event.target.checked)}/><span>Add unsubscribe link <small>(recommended — Gmail and Yahoo expect it for bulk mail)</small></span></label>
            <label className="table-check"><input type="checkbox" checked={trackOpens} onChange={event => setTrackOpens(event.target.checked)}/><span>Track opens <small>(adds a tracking pixel, which can slightly raise spam scores)</small></span></label>
          </div>
        </section>
        <section className="send-review">
          <div><strong>{eligible} eligible {eligible === 1 ? 'lead' : 'leads'}</strong>
            <small>{eligible ? `About ${minutes || 1} min to send` : 'Nobody matches this audience'}{remainingToday !== null ? ` · ${remainingToday} left in today's limit of ${settings?.dailyLimit}` : ''}{settings?.simulated ? ' · simulation mode' : ''}</small></div>
          <button type="submit" disabled={sending || !eligible || !subject.trim() || !body.trim() || unknown.length > 0}>{sending ? 'Sending…' : 'Send campaign'}</button>
        </section>
      </form>

      <aside className="email-preview">
        <div className="preview-head"><b>Preview</b>
          <select value={previewLeadId} onChange={event => setPreviewLeadId(Number(event.target.value))} aria-label="Preview as lead">
            <option value={0}>First eligible lead</option>
            {previewChoices.map(lead => <option key={lead.id} value={lead.id}>{lead.name || lead.email}{lead.organization ? ` — ${lead.organization}` : ''}</option>)}
          </select>
        </div>
        {preview ? <div className="preview-mail">
          <div className="preview-row"><small>To</small><span>{preview.lead.name ? `${preview.lead.name} <${preview.lead.email}>` : preview.lead.email}</span></div>
          <div className="preview-row"><small>Subject</small><b>{preview.subject || <span className="muted">(empty)</span>}</b></div>
          <div className="preview-body" dangerouslySetInnerHTML={{ __html: preview.html.replace(/<img[^>]*>/g, '') }}/>
          {!preview.eligible && <p className="muted">No eligible lead — showing sample data.</p>}
        </div> : <p className="muted">Loading preview…</p>}
      </aside>
    </div>

    {runs.length > 0 && <section className="campaign-history"><h3>Recent campaigns</h3>
      <div className="table-wrap"><table><thead><tr><th>Started</th><th>Type</th><th>Subject</th><th>Segment</th><th>Sent</th><th>Failed</th><th>Status</th></tr></thead>
        <tbody>{runs.map(run => <tr key={run.id}><td>{formatDate(run.createdAt)}</td><td>{run.mode === 'followup' ? 'Follow-up' : 'First email'}</td><td>{run.subject}</td><td>{run.segment || 'All'}</td><td>{run.sent} / {run.total}</td><td>{run.failed}</td><td><i className={run.status === 'completed' ? '' : run.status === 'running' ? 'reply' : 'bad'} title={run.lastError}>{statusLabel[run.status] ?? run.status}</i></td></tr>)}</tbody></table></div>
    </section>}

    {confirming && <ConfirmDialog busy={starting} title={`Send to ${eligible} ${eligible === 1 ? 'lead' : 'leads'}?`}
      message={`Each lead gets a personalised copy of “${preview?.subject ?? subject}”. Emails go out about ${settings?.perMinute ?? 120} per minute in the background — you can keep working while it sends.${settings?.simulated ? ' Simulation mode: no real email will be sent.' : ''}`}
      confirmLabel="Start sending" onConfirm={start} onCancel={() => setConfirming(false)}/>}
  </div>;
}

function CampaignProgress({ run, onDismiss }: { run: CampaignRun; onDismiss?: () => void }) {
  const done = run.sent + run.failed;
  const percent = run.total ? Math.round((done / run.total) * 100) : 0;
  return <div className={`campaign-progress ${run.status}`} role="status" aria-live="polite">
    <div className="progress-top"><b>{statusLabel[run.status] ?? run.status}: {run.subject}</b>{onDismiss && <button className="icon-button" onClick={onDismiss} aria-label="Dismiss">×</button>}</div>
    <div className="progress-track"><div style={{ width: `${percent}%` }}/></div>
    <small>{run.sent} sent · {run.failed} failed · {Math.max(0, run.total - done)} remaining of {run.total}</small>
    {run.lastError && run.status !== 'running' && <small className="form-error">{run.lastError}</small>}
  </div>;
}
