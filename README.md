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

- Leads with email/phone duplicate protection, categories, state, and follow-up counters
- Manual creation, CSV / Google Sheets CSV URL import, and filters
- Email history, reply recording, and an open-tracking pixel endpoint
- Basic email automation with date filtering and configurable follow-up delay
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

In Google Sheets choose **File → Share → Publish to web → CSV**, then paste the generated URL in the import screen. Expected headings include `email`, `phone`, `category`, `subcategory`, and `name` (case-insensitive).
