export type Lead = {
  id: number;
  name: string;
  organization: string;
  email: string;
  phone: string;
  designation: string;
  address: string;
  segment: string;
  mailSent: boolean;
  isInvalid: boolean;
  isOpened: boolean;
  anyFollowup: boolean;
  followupCount: number;
  replied: boolean;
  unsubscribed: boolean;
  lastActivityAt: string;
  createdAt: string;
};

export type LeadInput = {
  name?: string;
  organization?: string;
  email?: string;
  phone?: string;
  designation?: string;
  address?: string;
  segment?: string;
};

export type LeadEvent = {
  id: number;
  kind: string;
  subject: string;
  body: string;
  content: string;
  from: string;
  to: string;
  resendId: string;
  createdAt: string;
};

export type DashboardStats = {
  total: number;
  emailed: number;
  opened: number;
  replied: number;
  invalid: number;
  unsubscribed: number;
  sentToday: number;
};

export type ImportField = 'name' | 'organization' | 'email' | 'phone' | 'designation' | 'address' | 'segment';

export type ImportSource = { csv?: string; url?: string };

export type ImportPreview = {
  headers: string[];
  fields: ImportField[];
  mapping: Record<ImportField, string>;
  ignored: string[];
  rows: number;
  withEmail: number;
  sample: LeadInput[];
};

export type ImportResultRow = { row: number; status: 'added' | 'duplicate' | 'invalid' | 'empty'; lead: LeadInput; id?: number };

export type ImportSummary = {
  rows: number;
  imported: number;
  skipped: number;
  duplicates: number;
  empty: number;
  invalid: number;
  mapping: Record<ImportField, string>;
  results: ImportResultRow[];
};

export type CampaignPayload = {
  mode: 'initial' | 'followup';
  subject: string;
  body: string;
  segment: string;
  before: string;
  noFollowup: boolean;
  maxFollowups: number;
  trackOpens: boolean;
  includeUnsubscribe: boolean;
};

export type CampaignPreview = {
  eligible: number;
  lead: Lead;
  subject: string;
  text: string;
  html: string;
  unknown: string[];
  fields: string[];
  dailyLimit: number;
  sentToday: number;
};

export type CampaignRun = {
  id: number;
  mode: string;
  subject: string;
  segment: string;
  status: 'running' | 'completed' | 'failed' | 'daily_limit' | 'interrupted' | string;
  total: number;
  sent: number;
  failed: number;
  lastError: string;
  createdAt: string;
  finishedAt: string;
};

export type SendSettings = {
  simulated: boolean;
  from: string;
  replyTo: string;
  dailyLimit: number;
  sentToday: number;
  perMinute: number;
  activeCampaign: number;
  warnings: string[];
};

export type Conversation = {
  lead: Lead;
  lastKind: string;
  lastSubject: string;
  lastSnippet: string;
  lastAt: string;
  messages: number;
  replies: number;
};
