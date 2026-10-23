# Operations

Mahlzeit runs as one container (server binary plus the Typst CLI) with one persistent volume. TLS terminates at your ingress or reverse proxy. The image runs as `nonroot` with a read-only root filesystem and a writable `/tmp`. Kubernetes example: [`deploy/kubernetes.yaml`](../deploy/kubernetes.yaml).

## Configuration

All settings are environment variables. Secrets can also be read from a file: append `_FILE` to the variable name. Invalid combinations stop the server at startup with a list of all problems.

| Variable | Default | Meaning |
| --- | --- | --- |
| `MAHLZEIT_BASE_URL` | `http://localhost:8080` | Public URL. `https://` enables Secure cookies; the OIDC redirect uses it. |
| `MAHLZEIT_DATA_DIR` | `/data` | SQLite database (`mahlzeit.db`), receipts (`files/`), reports (`exports/`). |
| `MAHLZEIT_LISTEN` | `:8080` | Listen address. |
| `MAHLZEIT_SETUP_TOKEN` | – | Token for creating the account with a password. Without it, a random token is written to `$MAHLZEIT_DATA_DIR/setup-token` while no account exists. |
| `MAHLZEIT_PASSWORD_LOGIN` | `true` | Password login. Keep it as the emergency route even with OIDC. |
| `MAHLZEIT_OIDC_ISSUER` | – | Enables OIDC, e.g. `https://id.example.com` (Pocket ID). |
| `MAHLZEIT_OIDC_CLIENT_ID` / `_CLIENT_SECRET` | – | Client credentials. |
| `MAHLZEIT_OIDC_LABEL` | `Pocket ID` | Button text on the login page. |
| `MAHLZEIT_OIDC_SCOPES` | `openid,profile,email,groups` | Requested scopes. |
| `MAHLZEIT_OIDC_GROUPS_CLAIM` | `groups` | Claim holding the groups. |
| `MAHLZEIT_OIDC_ALLOWED_EMAILS` / `_GROUPS` / `_SUBJECTS` | – | Who may log in. The first allowed identity creates the account (or is linked to the password account); afterwards only that identity logs in. |
| `MAHLZEIT_EXTRACT_PROVIDER` | `claude` | Who reads receipts: `claude`, `gemini` or `openai` (any OpenAI-compatible API). See [Receipt reading](#receipt-reading). |
| `ANTHROPIC_API_KEY` | – | Claude: enables reading receipts. Without it, date, total and merchant are typed by hand. |
| `GEMINI_API_KEY` | – | Gemini: Google AI Studio key (required with `gemini`). |
| `OPENAI_API_KEY` | – | OpenAI-compatible: API key; leave empty for local servers. |
| `MAHLZEIT_EXTRACT_MODEL` | `claude-opus-5-5` / `gemini-flash-latest` / – | Model; required with `openai`. It must read images, and PDFs for PDF receipts. |
| `MAHLZEIT_EXTRACT_BASE_URL` | provider's API | Another endpoint, e.g. `https://openrouter.ai/api/v1` or `http://ollama:11434/v1` (default for `openai`: `https://api.openai.com/v1`). |
| `MAHLZEIT_EXTRACT_FALLBACK` | `true` | Claude only: when Claude declines a receipt, the API retries it with a fallback model it picks. Turn off for models without server-side fallback. |
| `MAHLZEIT_IMAP_HOST` | – | Enables the mail import, e.g. `imap.example.com` (TLS). |
| `MAHLZEIT_IMAP_PORT` | `993` | IMAP TLS port. |
| `MAHLZEIT_IMAP_USERNAME` / `_PASSWORD` | – | Mailbox credentials (required with the host). |
| `MAHLZEIT_IMAP_ALLOWED_SENDERS` | – | Comma-separated sender addresses; mail from anyone else is not imported, but processed (moved or deleted) like the rest (required). |
| `MAHLZEIT_IMAP_FOLDER` | `INBOX` | Folder polled for unread mail. |
| `MAHLZEIT_IMAP_PROCESSED_FOLDER` | `Processed` | Processed mails are moved here (created if missing). |
| `MAHLZEIT_IMAP_DELETE` | `false` | Delete processed mails instead of moving them. |
| `MAHLZEIT_IMAP_INTERVAL` | `5m` | Polling interval (at least `1m`). |
| `MAHLZEIT_TYPST` | `typst` | Typst binary (the image sets `/usr/local/bin/typst`). |
| `MAHLZEIT_MAX_UPLOAD_MB` | `25` | Upload limit per file (1–25). |
| `MAHLZEIT_MAX_MEGAPIXELS` | `40` | Image size limit (1–40). |
| `MAHLZEIT_MAX_PDF_PAGES` | `50` | PDF page limit (1–50). |
| `MAHLZEIT_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Logs never contain passwords, tokens or receipt contents. |

## First start

1. Deploy with `MAHLZEIT_SETUP_TOKEN` set, or read `/data/setup-token`.
2. Open the URL, enter the token and create your account; you land on the settings page. Enter company, Bundesland, tax model and optionally the legal basis printed on every report.
3. With OIDC configured, log in once with Pocket ID: an allowed identity is linked to the account.

OIDC only (`MAHLZEIT_PASSWORD_LOGIN=false`): there is no setup page. The first login of an allowed identity creates the account. A password can still be set later in the settings as an emergency route (the username is the account's email address).

## Pocket ID

1. Create an OIDC client "Mahlzeit" with callback URL `https://<your-host>/auth/oidc/callback`. Mahlzeit always sends PKCE.
2. Set `MAHLZEIT_OIDC_ISSUER`, `MAHLZEIT_OIDC_CLIENT_ID`, `MAHLZEIT_OIDC_CLIENT_SECRET` and one of the allow lists (simplest: `MAHLZEIT_OIDC_ALLOWED_EMAILS` with your address).

The identity is bound to issuer and subject, never to the email address. Logout ends the Mahlzeit session only.

## Mail import

Use a dedicated mailbox. Send receipts as attachments or inline photos (JPEG, PNG, PDF) from an allowed address. Every few minutes Mahlzeit fetches unread mail, imports the attachments as receipts to check (with the subject as note) and moves the mail to the processed folder. Settings → Mail-Eingang shows the last run and the last error.

## Receipt reading

Every new receipt is sent to the configured model, which returns date, total and merchant; the image leaves your server for this (except with a local model). Nothing counts until you check it. Rate limits and temporary API errors are retried automatically; with Claude, a declined receipt goes to a fallback model. Readings that still fail can be retried from the check screen, or you type the values.

| Provider | Settings |
| --- | --- |
| Claude (default) | `ANTHROPIC_API_KEY` |
| Gemini | `MAHLZEIT_EXTRACT_PROVIDER=gemini`, `GEMINI_API_KEY` from Google AI Studio |
| OpenRouter | `MAHLZEIT_EXTRACT_PROVIDER=openai`, `MAHLZEIT_EXTRACT_BASE_URL=https://openrouter.ai/api/v1`, `OPENAI_API_KEY`, `MAHLZEIT_EXTRACT_MODEL` (e.g. a vision model's OpenRouter ID) |
| Ollama, LM Studio | `MAHLZEIT_EXTRACT_PROVIDER=openai`, `MAHLZEIT_EXTRACT_BASE_URL=http://<host>:11434/v1`, `MAHLZEIT_EXTRACT_MODEL` (a vision model) |

Free tiers (Gemini, some OpenRouter models) may let the provider use receipts to improve its models or have people review them; check the provider's terms, especially for use in the EU.

## Backup and restore

Receipts and reports are evidence: payroll records must be kept for six years. Mahlzeit never deletes a checked receipt or a report.

A copy of the live SQLite file alone is not a backup. Use the built-in command; it writes a consistent database snapshot (`VACUUM INTO`) and then copies receipts and reports:

```sh
kubectl exec deploy/mahlzeit -- mahlzeit backup /data/backups/$(date +%F)
```

The target must not exist yet. Snapshots on the data volume protect against mistakes, not against losing the volume: back up the volume itself (Longhorn, Velero, k8up, …) as well. A built-in upload to S3 is on the list of deferred work ([BRIEF](BRIEF.md)).

Restore: stop the deployment, replace the contents of `/data` with a backup directory, start it again and check every checksum:

```sh
kubectl exec deploy/mahlzeit -- mahlzeit verify
```

## Upgrades

Database migrations run automatically at startup. Back up before upgrading to a new minor version.
