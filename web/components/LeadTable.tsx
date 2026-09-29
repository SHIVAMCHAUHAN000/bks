import type { Lead } from '../lib/types';

type Props = {
  leads: Lead[];
  selected: Set<number>;
  onToggle: (id: number) => void;
  onTogglePage: (checked: boolean) => void;
  onSelect: (lead: Lead) => void;
  onDelete: (lead: Lead) => void;
};

export function LeadTable({ leads, selected, onToggle, onTogglePage, onSelect, onDelete }: Props) {
  if (!leads.length) return <div className="empty">No leads found.</div>;
  const allSelected = leads.every(lead => selected.has(lead.id));
  return <div className="table-wrap"><table className="lead-table"><thead><tr>
    <th className="check-cell"><input type="checkbox" aria-label="Select all on this page" checked={allSelected} onChange={event => onTogglePage(event.target.checked)}/></th>
    <th>Lead</th><th>Organization</th><th>Contact</th><th>Segment</th><th>Follow-ups</th><th>Status</th><th/>
  </tr></thead><tbody>{leads.map(lead => <tr key={lead.id} className={selected.has(lead.id) ? 'selected' : ''}>
    <td className="check-cell"><input type="checkbox" aria-label={`Select ${lead.name || lead.email}`} checked={selected.has(lead.id)} onChange={() => onToggle(lead.id)}/></td>
    <td><b>{lead.name || 'Unnamed'}</b><br/><small>{lead.designation || '—'}</small></td>
    <td>{lead.organization || '—'}<br/><small>{lead.address}</small></td>
    <td className="contact-cell">{lead.email || <span className="muted">No email</span>}<br/><small>{lead.phone}</small></td>
    <td>{lead.segment || '—'}</td>
    <td>{lead.anyFollowup ? lead.followupCount : '—'}</td>
    <td><LeadStatus lead={lead}/></td>
    <td className="row-actions"><button className="secondary compact" onClick={() => onSelect(lead)}>View</button><button className="danger compact" onClick={() => onDelete(lead)} aria-label={`Delete ${lead.name || lead.email}`}>Delete</button></td>
  </tr>)}</tbody></table></div>;
}

export function LeadStatus({ lead }: { lead: Lead }) {
  const status = leadStatus(lead);
  const className = { Unsubscribed: 'bad', Invalid: 'bad', Replied: 'reply', Opened: '', Sent: '', Uncontacted: 'draft' }[status];
  return <i className={className}>{status}</i>;
}

export function leadStatus(lead: Lead) {
  if (lead.unsubscribed) return 'Unsubscribed';
  if (lead.isInvalid) return 'Invalid';
  if (lead.replied) return 'Replied';
  if (lead.isOpened) return 'Opened';
  if (lead.mailSent) return 'Sent';
  return 'Uncontacted';
}
