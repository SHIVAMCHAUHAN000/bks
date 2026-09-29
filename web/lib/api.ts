import type {
  CampaignPayload, CampaignPreview, CampaignRun, Conversation, DashboardStats, ImportField, ImportPreview, ImportSource,
  ImportSummary, Lead, LeadEvent, LeadInput, SendSettings,
} from './types';

const getApiURL = () => process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${getApiURL()}${path}`, {
      ...options,
      headers: { 'Content-Type': 'application/json', ...options?.headers },
    });
  } catch {
    throw new Error(`Cannot reach the API at ${getApiURL()}. Is the Go server running?`);
  }
  let data: any = null;
  try {
    data = await response.json();
  } catch {
    // ignore non-JSON body
  }
  if (!response.ok) throw new Error(data?.error ?? `Request failed (HTTP ${response.status})`);
  return data as T;
}

const post = (body: unknown): RequestInit => ({ method: 'POST', body: JSON.stringify(body) });

type ImportOptions = ImportSource & { mapping?: Partial<Record<ImportField, string>>; segment?: string };

export const api = {
  listLeads: (query = '', segment = '') => request<Lead[]>(`/api/leads?q=${encodeURIComponent(query)}&segment=${encodeURIComponent(segment)}`),
  getLead: (id: number) => request<Lead>(`/api/leads/${id}`),
  dashboard: () => request<DashboardStats>('/api/dashboard'),
  createLead: (payload: LeadInput) => request<Lead>('/api/leads', post(payload)),
  deleteLead: (id: number) => request<{ deleted: number }>(`/api/leads/${id}`, { method: 'DELETE' }),
  deleteLeads: (ids: number[]) => request<{ deleted: number }>('/api/leads/delete', post({ ids })),
  previewImport: (options: ImportOptions) => request<ImportPreview>('/api/import/preview', post(options)),
  importLeads: (options: ImportOptions) => request<ImportSummary>('/api/import', post(options)),
  previewCampaign: (payload: CampaignPayload, leadId?: number) => request<CampaignPreview>('/api/automation/preview', post({ ...payload, leadId })),
  sendCampaign: (payload: CampaignPayload) => request<CampaignRun>('/api/automation/send', post(payload)),
  campaign: (id: number) => request<CampaignRun>(`/api/campaigns/${id}`),
  campaigns: () => request<CampaignRun[]>('/api/campaigns'),
  settings: () => request<SendSettings>('/api/settings'),
  conversations: (filter: 'all' | 'replied' = 'all', query = '') => request<Conversation[]>(`/api/conversations?filter=${filter}&q=${encodeURIComponent(query)}`),
  events: (leadID: number) => request<LeadEvent[]>(`/api/leads/${leadID}/events`),
  recordReply: (leadID: number, content: string) => request<{ ok: boolean }>(`/api/leads/${leadID}/reply`, post({ content })),
  markInvalid: (leadID: number) => request<{ ok: boolean }>(`/api/leads/${leadID}/invalid`, { method: 'PATCH' }),
};

export function errorMessage(error: unknown) { return error instanceof Error ? error.message : 'Request failed'; }

export function formatDate(value: string) {
  if (!value) return '';
  const date = new Date(value.replace(' ', 'T') + (value.includes('Z') || value.includes('+') ? '' : 'Z'));
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}
