# Automation Tool

A small admin panel for importing, organizing, and emailing leads.

See the full [usage guide](docs/USAGE.md) for setup, workflows, Resend, webhooks, and deployment guidance.

## Run locally

```bash
# terminal 1
cd api
go mod tidy
go run ./cmd/server

# terminal 2
cd web
npm install
npm run dev
```

The API runs on `http://localhost:8080` and uses `api/data/automation_tool.db` (SQLite).
Set `RESEND_API_KEY` and `MAIL_FROM` to send real emails. Without them, sends are recorded as `simulated` in local development.

The API loads `api/.env` automatically. Copy or update [api/.env.example](api/.env.example) with your Resend key and verified sender address; the real `.env` file is ignored by Git.

Set `NEXT_PUBLIC_API_URL` in `web/.env.local` to the Go API URL. In a deployment, set the API's `CORS_ALLOWED_ORIGIN` to the exact frontend origin.

## Resend webhooks

The API receives signed Resend callbacks at `POST /webhooks/resend`. After deploying the API on a public HTTPS URL, create a webhook in Resend with:

- Endpoint: `https://your-api.example.com/webhooks/resend`
- Events: `email.delivered`, `email.bounced`, `email.opened`, and `email.received`

Copy the generated `whsec_…` signing secret to `RESEND_WEBHOOK_SECRET` in `api/.env`. Set `MAIL_REPLY_TO` to your Resend inbound address (or a receiving-enabled custom-domain address) so campaign replies arrive there. The endpoint verifies the Svix signature and timestamp, ignores duplicate deliveries, records delivery/bounce/open activity, marks bounced leads invalid, and retrieves inbound reply text before recording it as a reply.

## Included in v1

- Leads with name, organization, email, phone, designation, address and segment; duplicate protection; delete and bulk delete
- CSV / Google Sheets import with automatic column matching, preview and per-row results
- Inbox with the exact email sent to each lead and their replies
- Personalised campaigns (`{{first_name}}`, `{{organization}}`, …) sent in the background with rate limiting, daily cap, unsubscribe links and progress
- Resend integration behind a small mail service

## API layout

```text
api/
  cmd/server/main.go       # tiny application entrypoint
  internal/config/         # environment configuration
  internal/database/       # SQLite connection and migrations
  internal/lead/           # lead domain types and SQL repository
  internal/mailer/         # Resend mail provider
  internal/httpapi/        # routes and HTTP handlers
```

## Google Sheets import

In Google Sheets choose **File → Share → Publish to web → CSV**, then paste the generated URL in the import screen. Common headings such as Name, Company/Organization, Email, Phone/Mobile, Designation/Title, Address/City and Segment are detected automatically; other columns are ignored.
