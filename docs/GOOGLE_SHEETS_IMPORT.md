# Google Sheets Import and Configuration

This guide explains how to import leads from Google Sheets into LeadDesk and how to configure the application for local or deployed use.

## Required sheet format

The first row must contain column headings. Heading names are case-insensitive and surrounding spaces are ignored.

| Column | Required | Description |
| --- | --- | --- |
| `email` | One of `email` or `phone` is required | Lead email address. It is trimmed and converted to lowercase. |
| `phone` | One of `email` or `phone` is required | Lead phone number. |
| `name` | No | Lead name. |
| `category` | No | Used to filter campaign recipients. |
| `subcategory` | No | Additional lead grouping information. |

Recommended header row:

```csv
name,email,phone,category,subcategory
Ava,ava@example.com,555-0100,SaaS,HR
Noah,noah@example.com,555-0101,Agency,Marketing
Mia,,555-0102,Consulting,Finance
```

At least one of `email` or `phone` must exist in the header row. Each data row must contain at least one non-empty email or phone value. Rows with neither value are ignored.

## Publish a Google Sheet correctly

1. Open the spreadsheet in Google Sheets.
2. Select **File > Share > Publish to web**.
3. Select the required sheet or the entire spreadsheet.
4. Select **Comma-separated values (.csv)** as the format.
5. Click **Publish** and confirm.
6. Copy the published URL.
7. In LeadDesk, open **Import**, paste the URL into **Published Google Sheets CSV URL**, and click **Import leads**.

The published sheet must be accessible without a Google login. A private share link or a normal browser edit URL can return an HTML page instead of CSV data.

The importer also converts common Google Sheets edit URLs such as:

```text
https://docs.google.com/spreadsheets/d/SPREADSHEET_ID/edit?gid=0
```

into a CSV export request. Publishing the sheet is still recommended because it provides predictable public access. Published URLs containing `/d/e/` are used as provided.

## Import behavior

- The API accepts CSV pasted into the text area or downloaded from the supplied URL.
- The maximum downloaded import size is 10 MB.
- Email values are trimmed and lowercased before storage.
- A non-empty email or phone value must be unique. Duplicate rows are skipped.
- Rows with neither email nor phone are skipped.
- Invalid email syntax is stored as an invalid lead and is excluded from campaigns.
- `name`, `category`, and `subcategory` may be empty.
- Existing leads are not updated by import. To change a lead, update it through the database or add an update endpoint.
- The import response reports the number of rows read and the number successfully inserted.

Example response:

```json
{"imported": 2, "rows": 3}
```

## API import request

The frontend sends this request to the Go API:

```bash
curl -X POST http://localhost:8080/api/import \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://docs.google.com/spreadsheets/d/SPREADSHEET_ID/export?format=csv"}'
```

A direct CSV request can also be made with:

```bash
curl -X POST http://localhost:8080/api/import \
  -H 'Content-Type: application/json' \
  -d '{"csv":"name,email,phone,category,subcategory\nAva,ava@example.com,555-0100,SaaS,HR"}'
```

The API returns an error when the URL is not HTTPS, the remote server returns a non-success HTTP status, the response is not valid CSV, or the header row has neither `email` nor `phone`.

## Local configuration

Create `api/.env`:

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

Create `web/.env.local`:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Start the services from the repository root:

```bash
cd api
go run ./cmd/server
```

In another terminal:

```bash
cd web
npm run dev
```

Open `http://localhost:3000`.

The Google Sheets import itself does not require a Resend API key. The API server must be running, and the API server must be able to make outbound HTTPS requests to Google. The frontend origin must match `CORS_ALLOWED_ORIGIN`.

## Email configuration

For simulated local campaigns, leave `RESEND_API_KEY` empty. Sends are recorded in the local database but no email is delivered.

For real email:

1. Create a Resend API key.
2. Verify the sending domain in Resend.
3. Set `RESEND_API_KEY` and `MAIL_FROM`.
4. Set `MAIL_REPLY_TO` to an inbound Resend address if replies should be received automatically.
5. Restart the API after changing `api/.env`.

Example:

```env
RESEND_API_KEY=re_xxxxxxxxx
MAIL_FROM=Sales <sales@example.com>
MAIL_REPLY_TO=reply@example.com
```

## Production configuration

For a deployed frontend and API:

```env
# API
PUBLIC_API_URL=https://api.example.com
CORS_ALLOWED_ORIGIN=https://app.example.com
DATABASE_PATH=data/leaddesk.db

# Frontend build environment
NEXT_PUBLIC_API_URL=https://api.example.com
```

Use a public HTTPS API URL. `CORS_ALLOWED_ORIGIN` must be the exact frontend origin, including the scheme and hostname. Do not use `*` for a production admin application.

The SQLite database is stored at `DATABASE_PATH`. Back it up regularly. Move to a server database such as Postgres before running multiple application instances or supporting multiple users.

## Troubleshooting Google Sheets imports

### "provide a CSV with a header row..."

The URL probably returned an HTML page, an empty sheet, or a non-CSV response. Publish the sheet using **File > Share > Publish to web**, choose CSV, and use the published URL.

### "Google Sheets returned HTTP 401 or 403"

The sheet is not publicly readable, or the link is restricted to a Google account. Publish it to the web or change its sharing settings, then retry.

### "CSV must include an email or phone column"

The first row does not contain `email` or `phone`. Rename the headers to exactly `email` and/or `phone`, then retry. Header capitalization does not matter.

### Import says zero leads were added

Check that each row has an email or phone value. Also check whether those values already exist in LeadDesk. Duplicate email and phone values are intentionally skipped.

### The browser shows a network or CORS error

Check that:

- The Go API is running on the URL in `NEXT_PUBLIC_API_URL`.
- `CORS_ALLOWED_ORIGIN` matches the browser origin exactly.
- The API was restarted after changing environment variables.
- The browser can reach `/health` on the API URL.

### The URL works in a browser but not in LeadDesk

A browser may be logged into Google while the API is not. Test the URL in a private browser window or with `curl`. The URL must return CSV without authentication.

## Related endpoints

| Method | Endpoint | Purpose |
| --- | --- | --- |
| `POST` | `/api/import` | Import pasted CSV or a Google Sheets CSV URL. |
| `GET` | `/health` | Check that the API is running. |
| `GET` | `/api/leads?q=` | Verify imported leads. |
