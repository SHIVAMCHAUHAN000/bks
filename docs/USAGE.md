# Automation Tool usage guide

Automation Tool is a small admin application for importing leads, sending personalised first and follow-up emails through Resend, and reading replies in one Inbox.

## Start locally

Open two terminals from the repository root.

```bash
# API
cd api
go mod tidy
go run ./cmd/server
```

```bash
# Admin panel
cd web
npm install
npm run dev
```

Open `http://localhost:3000`. The API defaults to `http://localhost:8080` and the local SQLite database is stored in `api/data/automation_tool.db`.

## Configuration

The API reads `api/.env` at startup. Do not commit this file.

```env
PORT=8080
DATABASE_PATH=data/automation_tool.db
PUBLIC_API_URL=http://localhost:8080
CORS_ALLOWED_ORIGIN=http://localhost:3000

RESEND_API_KEY=
MAIL_FROM=
MAIL_REPLY_TO=
RESEND_WEBHOOK_SECRET=

# Sending pace and daily cap (0 = no cap)
SEND_RATE_PER_SECOND=2
DAILY_SEND_LIMIT=0
```

The frontend reads `web/.env.local`:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

For local work, empty `RESEND_API_KEY` values are safe: campaigns are simulated and still appear in lead history. Real email is sent only when both a valid Resend key and sender address are configured.

## Lead workflow

Every lead has **Name, Organization, Email, Phone number, Designation, Address** and an optional **Segment** (a group such as "Hospitals" or "Schools" used to target campaigns). A lead needs at least an email or a phone number and is unique by email and by phone. Only leads with an email can be sent campaigns.

### Add or delete a lead

Use the form at the top of the **Leads** tab. The new lead appears in the table straight away.

Use **Delete** on a row, or tick several rows and choose **Delete selected**. Deleting removes the lead and its email history permanently; the same email can be imported again later.

### Import leads

Use the **Import** tab to upload a CSV (in Excel: *File → Save As → CSV UTF-8*), paste CSV text, or paste a published Google Sheets CSV URL. See [Google Sheets Import](GOOGLE_SHEETS_IMPORT.md) for publishing a sheet.

Your file can have any column layout. The importer recognises common headings, for example:

| Field | Recognised headings (case and punctuation ignored) |
| --- | --- |
| Name | Name, Full Name, Contact Person, Client Name (or First Name + Last Name) |
| Organization | Organization, Organisation, Company, Company Name, Firm, Business, Institution |
| Email | Email, E-mail, Email ID, Email Address, Mail ID |
| Phone number | Phone, Mobile, Mobile No, Contact, Contact Number, Telephone, WhatsApp |
| Designation | Designation, Title, Job Title, Position, Role |
| Address | Address, Location, City, Office Address |
| Segment | Segment, Category, Industry, Sector |

Every other column is ignored. After choosing a file you see a preview: which column fills each field (change any of them from the drop-downs), the ignored columns, and the first rows exactly as they will be saved. You can also set one segment for the whole file. After importing, a table lists every added row and every skipped row with the reason (already exists, invalid email, or no email/phone).

### Review a lead

Select **View** on a lead to see its details and the full conversation. The panel also lets you record a reply that arrived outside the Inbox, or mark the lead invalid (invalid leads are excluded from campaigns).

## Sending campaigns

The **Automation** tab supports two modes:

| Mode | Recipients |
| --- | --- |
| First email | Valid leads with an email that have not been emailed, have not replied and have not unsubscribed. |
| Follow-up | Valid, previously emailed leads that have not replied and have not unsubscribed. |

Filter by segment and by the date the lead was added. Follow-ups can also be limited by follow-up count.

### Personalisation

Use placeholders in the subject and body. Each lead receives its own copy:

| Placeholder | Value |
| --- | --- |
| `{{first_name}}` | First word of the name (titles such as Mr./Dr. are skipped) |
| `{{name}}` | Full name |
| `{{organization}}` | Organization |
| `{{designation}}` | Designation |
| `{{address}}` | Address |
| `{{email}}`, `{{phone}}`, `{{segment}}` | The lead's other fields |

Add a fallback for empty values with `|`, e.g. `Hi {{first_name|there}},`. The preview on the right shows the exact email for any lead. Unknown placeholders are flagged and block sending.

### How sending works

A campaign runs in the background, so you can keep using the app. Emails are sent one at a time at `SEND_RATE_PER_SECOND` (default 2 per second, Resend's default API limit), and a rate-limited send is retried automatically. A progress bar shows sent, failed and remaining. Every send stores the exact personalised email in the lead's history; failures are recorded with the reason. If `DAILY_SEND_LIMIT` is set, the campaign stops once that many emails were sent today — run it again the next day to continue with the remaining leads.

Each email has a plain-text and a simple HTML version. By default it includes an unsubscribe link and `List-Unsubscribe` headers (expected by Gmail and Yahoo for bulk mail); unsubscribed leads are never emailed again. Open tracking is optional and off by default.

### Staying out of spam

- Send from your own domain verified in Resend (SPF, DKIM, and a DMARC record). `onboarding@resend.dev` can only send to your own address.
- Set `MAIL_FROM` with a real name, e.g. `Shivam from Omnidel <shivam@yourdomain.com>`.
- Warm up a new domain: start with a small `DAILY_SEND_LIMIT` (for example 30–50 a day) and raise it gradually over a few weeks.
- Keep emails short, personal, plain text, with at most one link and no attachments.
- Remove bounced and unsubscribed leads (this happens automatically when webhooks are set up).

## Inbox

The **Inbox** tab lists every lead you have emailed, newest activity first, with a **Replied** filter. Click a conversation to see each email exactly as it was sent, followed by the replies and delivery events (delivered, opened, bounced). Replies arrive automatically through the Resend webhook (see below); the Inbox checks for new ones every 30 seconds.

## Resend setup

1. Create a Resend API key and verify the sending domain.
2. Set `RESEND_API_KEY` and `MAIL_FROM` in `api/.env`.
3. Enable an inbound Resend address or a receiving-enabled custom domain.
4. Set that address as `MAIL_REPLY_TO`. Campaign emails will direct replies there.
5. Deploy the API on a public HTTPS URL and set `PUBLIC_API_URL` to that URL. This is needed for email open tracking.

## Resend webhooks

Create a webhook in the Resend dashboard with this public endpoint:

```text
https://your-api-domain.com/webhooks/resend
```

Select these events:

- `email.delivered`
- `email.bounced`
- `email.opened`
- `email.received`

Copy the generated `whsec_...` signing secret to `RESEND_WEBHOOK_SECRET` and restart the API.

The endpoint validates the raw payload and Svix headers before processing it. Resend retries are safe: each `svix-id` is stored once. Delivered and opened events are written to timeline history. A bounced email is written to history and automatically marks the lead invalid. For received emails, Automation Tool fetches the inbound message text from Resend and saves it as a reply.

## Deployment checklist

- Use a public HTTPS API URL, not `localhost`.
- Set `PUBLIC_API_URL` to the deployed API URL.
- Set `NEXT_PUBLIC_API_URL` to the deployed API URL before building the Next app.
- Set `CORS_ALLOWED_ORIGIN` to the exact deployed frontend origin.
- Configure `MAIL_REPLY_TO` and the Resend webhook before expecting automatic replies.
- Back up `api/data/automation_tool.db`; it is local SQLite storage. Move to Supabase/Postgres before multi-user or production-scale use.

## API reference

| Method | Endpoint | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Health check. |
| `GET` | `/api/leads?q=&segment=` | Search/filter leads. |
| `POST` | `/api/leads` | Create a lead (returns the lead). |
| `DELETE` | `/api/leads/{id}` | Delete a lead and its history. |
| `POST` | `/api/leads/delete` | Delete several leads: `{"ids":[1,2]}`. |
| `GET` | `/api/leads/{id}/events` | Retrieve activity history. |
| `POST` | `/api/leads/{id}/reply` | Record a manual reply. |
| `PATCH` | `/api/leads/{id}/invalid` | Mark a lead invalid. |
| `POST` | `/api/import/preview` | Detect columns and preview rows. |
| `POST` | `/api/import` | Import CSV or a published Sheets URL (optional `mapping`, `segment`). |
| `POST` | `/api/automation/preview` | Render a personalised preview and count eligible leads. |
| `POST` | `/api/automation/send` | Start a campaign in the background (returns the run). |
| `GET` | `/api/campaigns`, `/api/campaigns/{id}` | Campaign progress and history. |
| `GET` | `/api/conversations?filter=replied` | Inbox list. |
| `GET` | `/api/settings` | Sending configuration and warnings. |
| `GET` | `/api/dashboard` | Get dashboard counts. |
| `POST` | `/webhooks/resend` | Receive verified Resend events. |
| `GET` | `/track/open/{id}` | Email open-tracking pixel. |
| `GET`/`POST` | `/unsubscribe/{token}` | Unsubscribe link and one-click unsubscribe. |

Example manual lead creation:

```bash
curl -X POST http://localhost:8080/api/leads \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ava Rao","organization":"Acme","email":"ava@example.com","phone":"9876543210","designation":"CTO","address":"Pune","segment":"SaaS"}'
```
