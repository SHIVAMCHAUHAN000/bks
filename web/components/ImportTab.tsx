'use client';

import { useState } from 'react';
import { api, errorMessage } from '../lib/api';
import type { ImportField, ImportPreview, ImportResultRow, ImportSource, ImportSummary, LeadInput } from '../lib/types';

type Props = { onImported: (message: string) => Promise<void>; onError: (message: string) => void };

const fieldLabels: Record<ImportField, string> = { name: 'Name', organization: 'Organization', email: 'Email', phone: 'Phone number', designation: 'Designation', address: 'Address', segment: 'Segment' };
const fields: ImportField[] = ['name', 'organization', 'email', 'phone', 'designation', 'address', 'segment'];
const statusLabels: Record<ImportResultRow['status'], string> = { added: 'Added', duplicate: 'Already exists', invalid: 'Invalid email', empty: 'No email or phone' };

export function ImportTab({ onImported, onError }: Props) {
  const [source, setSource] = useState<ImportSource | null>(null);
  const [sourceLabel, setSourceLabel] = useState('');
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [mapping, setMapping] = useState<Partial<Record<ImportField, string>>>({});
  const [segment, setSegment] = useState('');
  const [summary, setSummary] = useState<ImportSummary | null>(null);
  const [busy, setBusy] = useState(false);

  async function loadPreview(nextSource: ImportSource, nextMapping = mapping, nextSegment = segment) {
    setBusy(true); onError('');
    try {
      const result = await api.previewImport({ ...nextSource, mapping: nextMapping, segment: nextSegment });
      setPreview(result); setSource(nextSource); setSummary(null);
    } catch (caught) { onError(errorMessage(caught)); }
    finally { setBusy(false); }
  }

  async function runImport() {
    if (!source) return;
    setBusy(true); onError('');
    try {
      const result = await api.importLeads({ ...source, mapping, segment });
      setSummary(result); setPreview(null);
      await onImported(`Imported ${result.imported} of ${result.rows} rows`);
    } catch (caught) { onError(errorMessage(caught)); }
    finally { setBusy(false); }
  }

  function changeMapping(field: ImportField, header: string) {
    const next = { ...mapping, [field]: header };
    setMapping(next);
    if (source) loadPreview(source, next);
  }

  function reset() { setSource(null); setPreview(null); setSummary(null); setMapping({}); setSegment(''); setSourceLabel(''); }

  if (summary) return <ImportResults summary={summary} onAgain={reset}/>;
  if (preview && source) return <div className="import-page">
    <div className="import-form">
      <div className="mapping-head">
        <div className="source-heading"><span>2</span><div><b>Check the columns</b><small>{sourceLabel} · {preview.rows} rows · {preview.withEmail} with an email</small></div></div>
        <button className="secondary compact" onClick={reset} disabled={busy}>Choose another file</button>
      </div>
      <div className="mapping-grid">{fields.map(field => <label key={field} className={preview.mapping[field] ? 'mapped' : ''}>
        <span>{fieldLabels[field]}{field === 'email' || field === 'phone' ? ' *' : ''}</span>
        <select value={preview.mapping[field] ?? ''} onChange={event => changeMapping(field, event.target.value)} disabled={busy}>
          <option value="">— Not in file —</option>
          {preview.headers.filter(Boolean).map(header => <option key={header} value={header}>{header}</option>)}
        </select>
      </label>)}
        <label className="segment-default"><span>Segment for all rows</span><input value={segment} placeholder="e.g. Hospitals (optional)" onChange={event => setSegment(event.target.value)} onBlur={() => source && loadPreview(source, mapping, segment)}/></label>
      </div>
      {preview.ignored.length > 0 && <p className="ignored-columns"><b>Ignored columns:</b> {preview.ignored.map(header => <span key={header}>{header}</span>)}</p>}
      <div className="sample-wrap">
        <p className="sample-title">First {preview.sample.length} rows as they will be saved</p>
        <LeadRows rows={preview.sample}/>
      </div>
      <div className="import-footer">
        <p>{!preview.mapping.email && !preview.mapping.phone ? <span className="form-error">Choose the Email or Phone number column to continue.</span> : <>Rows without an email or phone number, and leads that already exist, will be skipped.</>}</p>
        <button onClick={runImport} disabled={busy || (!preview.mapping.email && !preview.mapping.phone)}>{busy ? 'Importing…' : `Import ${preview.rows} rows`}</button>
      </div>
    </div>
  </div>;

  return <SourcePicker busy={busy} onPick={(nextSource, label) => { setSourceLabel(label); setMapping({}); loadPreview(nextSource, {}, segment); }} onError={onError}/>;
}

function SourcePicker({ busy, onPick, onError }: { busy: boolean; onPick: (source: ImportSource, label: string) => void; onError: (message: string) => void }) {
  const [dragActive, setDragActive] = useState(false);
  const [url, setUrl] = useState('');
  const [csv, setCsv] = useState('');

  async function pickFile(file: File | undefined) {
    if (!file) return;
    if (!/\.(csv|txt)$/i.test(file.name) && file.type !== 'text/csv') { onError('Please choose a .csv file. In Excel use File → Save As → CSV UTF-8.'); return; }
    onPick({ csv: await file.text() }, file.name);
  }

  return <div className="import-page">
    <div className="import-form">
      <div className="import-sources">
        <section className="import-source"><div className="source-heading"><span>01</span><div><b>Upload CSV</b><small>From your device</small></div></div>
          <label className={dragActive ? 'file-drop active' : 'file-drop'} onDragOver={event => { event.preventDefault(); setDragActive(true); }} onDragLeave={() => setDragActive(false)} onDrop={event => { event.preventDefault(); setDragActive(false); pickFile(event.dataTransfer.files[0]); }}>
            <input type="file" accept=".csv,text/csv" disabled={busy} onChange={event => { pickFile(event.target.files?.[0]); event.target.value = ''; }}/>
            <strong>{busy ? 'Reading file…' : 'Choose a CSV file'}</strong><span>or drop it here</span>
          </label>
        </section>
        <section className="import-source"><div className="source-heading"><span>02</span><div><b>Google Sheet</b><small>Published CSV URL</small></div></div>
          <input type="url" placeholder="Paste published URL" value={url} onChange={event => setUrl(event.target.value)}/>
          <button className="secondary source-button" disabled={busy || !url.trim()} onClick={() => onPick({ url }, 'Google Sheet')}>Preview sheet</button>
        </section>
      </div>
      <section className="paste-source"><div className="source-heading"><span>03</span><div><b>Paste CSV</b><small>Headers in the first row</small></div></div>
        <textarea value={csv} onChange={event => setCsv(event.target.value)} placeholder={'Name,Company,Email,Phone,Designation,Address\nRavi Kumar,Acme Pvt Ltd,ravi@acme.in,9876543210,Director,Pune'}/>
        <button className="secondary source-button" disabled={busy || !csv.trim()} onClick={() => onPick({ csv }, 'Pasted CSV')}>Preview pasted data</button>
      </section>
      <div className="import-footer"><p>Any column layout works: we pick out <b>Name, Organization, Email, Phone, Designation, Address</b> (and an optional Segment) and ignore the rest. You can check the match before importing.</p></div>
    </div>
  </div>;
}

function ImportResults({ summary, onAgain }: { summary: ImportSummary; onAgain: () => void }) {
  const [view, setView] = useState<'added' | 'skipped'>(summary.imported ? 'added' : 'skipped');
  const rows = summary.results.filter(row => view === 'added' ? row.status === 'added' : row.status !== 'added');
  const breakdown: Array<[number, string]> = [[summary.duplicates, 'already existed'], [summary.invalid, 'invalid email'], [summary.empty, 'no email or phone']];
  return <div className="import-page">
    <div className="import-result-head">
      <div><span className="summary-kicker">IMPORT COMPLETE</span><h2>{summary.imported} of {summary.rows} rows added</h2>
        <p className="muted">{breakdown.filter(([count]) => count > 0).map(([count, label]) => `${count} ${label}`).join(' · ') || 'Every row was added.'}</p></div>
      <button onClick={onAgain}>Import another file</button>
    </div>
    <div className="segmented">
      <button className={view === 'added' ? 'active' : ''} onClick={() => setView('added')}>Added ({summary.imported})</button>
      <button className={view === 'skipped' ? 'active' : ''} onClick={() => setView('skipped')}>Skipped ({summary.skipped})</button>
    </div>
    {rows.length ? <LeadRows rows={rows.slice(0, 500).map(row => row.lead)} status={rows.slice(0, 500)}/> : <div className="empty">Nothing here.</div>}
    {rows.length > 500 && <p className="muted">Showing the first 500 rows. All added leads are in the Leads tab.</p>}
  </div>;
}

function LeadRows({ rows, status }: { rows: LeadInput[]; status?: ImportResultRow[] }) {
  return <div className="table-wrap"><table className="import-table"><thead><tr>
    {status && <th>Row</th>}{fields.map(field => <th key={field}>{fieldLabels[field]}</th>)}{status && <th>Result</th>}
  </tr></thead><tbody>{rows.map((row, index) => <tr key={index}>
    {status && <td className="muted">{status[index].row}</td>}
    {fields.map(field => <td key={field}>{row[field] || <span className="muted">—</span>}</td>)}
    {status && <td><i className={status[index].status === 'added' ? '' : status[index].status === 'duplicate' ? 'draft' : 'bad'}>{statusLabels[status[index].status]}</i></td>}
  </tr>)}</tbody></table></div>;
}
