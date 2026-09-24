export type Lead = {
  id: number;
  name: string;
  email: string;
  phone: string;
  category: string;
  subcategory: string;
  mailSent: boolean;
  isInvalid: boolean;
  isOpened: boolean;
  anyFollowup: boolean;
  followupCount: number;
  replied: boolean;
  createdAt: string;
};

export type LeadEvent = {
  id: number;
  kind: string;
  subject: string;
  body: string;
  content: string;
  createdAt: string;
};

export type DashboardStats = {
  total: number;
  emailed: number;
  opened: number;
  replied: number;
  invalid: number;
};
