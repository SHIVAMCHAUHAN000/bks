import type { Lead } from '../lib/types';

type Props = { leads: Lead[]; onSelect: (lead: Lead) => void };

export function LeadTable({ leads, onSelect }: Props) {
  if (!leads.length) return <div className="empty">No leads found.</div>;
  return <table><thead><tr><th>Lead</th><th>Category</th><th>Mail sent</th><th>Follow-up</th><th>Replied</th><th>Status</th><th /></tr></thead><tbody>{leads.map(lead => <tr key={lead.id}>
    <td><b>{lead.name || 'Unnamed'}</b><br/><small>{lead.email || lead.phone}</small></td>
    <td>{lead.category || '—'}<br/><small>{lead.subcategory}</small></td>
    <td>{lead.mailSent ? 'Yes' : 'No'}</td>
    <td>{lead.anyFollowup ? `Yes (${lead.followupCount})` : 'No'}</td>
    <td>{lead.replied ? 'Yes' : 'No'}</td>
    <td><LeadStatus lead={lead}/></td>
    <td><button className="secondary compact" onClick={() => onSelect(lead)}>Details</button></td>
  </tr>)}</tbody></table>;
}

function LeadStatus({ lead }: { lead: Lead }) {
  if (lead.isInvalid) return <i className="bad">Invalid</i>;
  if (lead.replied) return <i className="reply">Replied</i>;
  if (lead.mailSent) return <i>Sent{lead.isOpened ? ' · Opened' : ''}</i>;
  return <i className="draft">Uncontacted</i>;
}
