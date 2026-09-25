# LeadDesk usage guide

LeadDesk is a small admin application for importing leads, categorizing them, sending initial and follow-up campaigns, and recording email activity.

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

Open `http://localhost:3000`. The API defaults to `http://localhost:8080` and the local SQLite database is stored in `api/data/leaddesk.db`.

## Configuration

The API reads `api/.env` at startup. Do not commit this file.

```env
PORT=8080
DATABASE_PATH=data/leaddesk.db
PUBLIC_API_URL=http://localhost:8080
CORS_ALLOWED_ORIGIN=http://localhost:3000

RESEND_API_KEY=
MAIL_FROM=
MAIL_REPLY_TO=
RESEND_WEBHOOK_SECRET=
```

The frontend reads `web/.env.local`:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

For local work, empty `RESEND_API_KEY` values are safe: campaigns are simulated and still appear in lead history. Real email is sent only when both a valid Resend key and sender address are configured.

## Lead workflow

### Add a lead

Use the **Leads** tab and complete at least one of email or phone. A lead is unique by a non-empty email or phone. Category and subcategory are optional and can be used to target campaigns.

An invalid email format is saved but flagged as invalid. Invalid leads are excluded from campaigns.

### Import leads

See the detailed [Google Sheets Import and Configuration](GOOGLE_SHEETS_IMPORT.md) guide for publishing, required fields, environment variables, API examples, and troubleshooting.

Use the **Import** tab to either:

1. Paste CSV content, or
2. Paste a Google Sheets published CSV URL.

In Google Sheets use **File → Share → Publish to web → CSV**, then paste the URL. Headings are case-insensitive. Supported columns are:

```text
name,email,phone,category,subcategory
```

Rows with neither an email nor phone are ignored. Existing email/phone duplicates are skipped.

### Review a lead

Select **Details** on a lead to view its activity timeline. It includes initial and follow-up sends, opens, Resend delivery updates, bounces, and replies.

From this panel an administrator can:

- Record a reply manually.
- Mark a lead invalid. This permanently excludes the lead from future campaigns unless the database is changed manually.

## Sending campaigns

The **Automation** tab supports two modes:

| Mode | Recipients |
| --- | --- |
| Initial email | Valid leads that have not been emailed and have not replied. |
| Follow-up email | Valid, previously emailed leads that have not replied. |

An optional category filter limits sends to leads in that category. An optional date filter includes only leads created on or before the selected date.

Before a real campaign, use a dedicated test lead and verify the sender address in Resend.

### Follow-up tracking

Each lead tracks `any_followup` and `followup_count`. Every follow-up also creates a dated `followup` record in the activity timeline.

There is currently no dedicated `last_followup_at` field. The latest follow-up timestamp is available from the newest `followup` event in that lead's activity history. Add `last_followup_at` before building delay-based follow-up scheduling.

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

The endpoint validates the raw payload and Svix headers before processing it. Resend retries are safe: each `svix-id` is stored once. Delivered and opened events are written to timeline history. A bounced email is written to history and automatically marks the lead invalid. For received emails, LeadDesk fetches the inbound message text from Resend and saves it as a reply.

## Deployment checklist

- Use a public HTTPS API URL, not `localhost`.
- Set `PUBLIC_API_URL` to the deployed API URL.
- Set `NEXT_PUBLIC_API_URL` to the deployed API URL before building the Next app.
- Set `CORS_ALLOWED_ORIGIN` to the exact deployed frontend origin.
- Configure `MAIL_REPLY_TO` and the Resend webhook before expecting automatic replies.
- Back up `api/data/leaddesk.db`; it is local SQLite storage. Move to Supabase/Postgres before multi-user or production-scale use.

## API reference

| Method | Endpoint | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Health check. |
| `GET` | `/api/leads?q=&category=` | Search/filter leads. |
| `POST` | `/api/leads` | Create a lead. |
| `GET` | `/api/leads/{id}/events` | Retrieve activity history. |
| `POST` | `/api/leads/{id}/reply` | Record a manual reply. |
| `PATCH` | `/api/leads/{id}/invalid` | Mark a lead invalid. |
| `POST` | `/api/import` | Import CSV or a published Sheets URL. |
| `POST` | `/api/automation/send` | Send an initial or follow-up campaign. |
| `GET` | `/api/dashboard` | Get dashboard counts. |
| `POST` | `/webhooks/resend` | Receive verified Resend events. |
| `GET` | `/track/open/{id}` | Email open-tracking pixel. |

Example manual lead creation:

```bash
curl -X POST http://localhost:8080/api/leads \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ava","email":"ava@example.com","category":"SaaS","subcategory":"HR"}'
```
