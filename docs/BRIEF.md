# Brief

Terms: [GLOSSARY.md](../GLOSSARY.md). Architecture choices: [adr/](adr/). Tax rules: [CALCULATION_RULES.md](CALCULATION_RULES.md).

## Owner's case

One employee (the managing shareholder), lunch from the supermarket on most working days, 15 days per month, maximum allowance, tax model A (company pays the flat tax). Company in Baden-Württemberg.

## Phase 1

- API like Spesen: Huma generates the OpenAPI spec, Hey API the client (separate codegen package on TypeScript 6).
- Login like Spesen: password (argon2id, account created with a one-time setup token) and OIDC (generic discovery, tested against Pocket ID, access by group or `sub`/email allowlist). Each mode can be switched off; at least one stays on. Password login is the emergency route when the identity provider is down. One account, no user admin, no own passkeys (Pocket ID provides them).
- Receipt import: photo or PDF upload (desktop and phone camera), and IMAP polling (every 5 min) of a dedicated mailbox, attachments from allowed senders only. Processed mails are marked read and moved to a folder; deleting them is an option. Optionally a WebDAV folder (Nextcloud, NAS) is polled the same way; processed files move to a subfolder or are deleted.
- An LLM fills date, total, merchant (Claude by default; Gemini or an OpenAI-compatible API selectable); every imported receipt is `zu prüfen` until the employee checks it. Missing date → warning, must be entered by hand. Duplicate detection by checksum.
- Calculation per month: a receipt belongs to the day of its own date; per day the highest receipt counts, or all receipts of that day combined when the employee chooses so (warning); above the day limit (15) the meal days with the highest totals count; allowance capped by the receipt total; tax models A, B, C; benefit values from 2026.
- Company settings: Bundesland, church tax on the flat tax (simplified rate or proof of non-membership; the owner uses proof → none), optional legal basis text printed on every report.
- The employee pays meals personally; the company pays the month's allowances in one sum at month end and the flat tax to the tax office.
- Warnings, never blocks: weekend, public holiday (Bundesland of the company), several receipts on one day, receipt dated in a closed month (counts only after reopening).
- Month lifecycle `offen` → `abgeschlossen` → `ausgezahlt`; reopening needs a reason and creates report v2, v1 stays byte-identical.
- Monthly report PDF (Typst, layout borrowed from Spesen): summary with totals per tax part and who pays the tax, day table, every counted receipt appended.
- Screens: Start (current month progress, payout so far, receipts to check, capture button), Belege (filter defaults: current month, all statuses), Monat (day table, close/reopen/paid), Berichte (all report versions), Einstellungen (company, employee, tax model, mail and folder import status, import log of the last 90 days, rate table read-only). Bottom navigation on phone, sidebar on desktop.
- Capture: camera, or picking an existing photo or PDF (iOS file picker). Check screen: receipt next to the extracted fields (stacked on phone), low-confidence fields highlighted, confirm jumps to the next receipt.
- Originals stay unchanged as evidence; previews and report images are re-rendered without EXIF.
- German and English UI (browser language, switchable in the settings; the report stays German), PWA, dark by default (light and system selectable, PDF light), usable from 360 px.
- Delivery: one OCI image (amd64/arm64, GHCR `ghcr.io/blackdark/vc-beleg-2`), Kubernetes single replica, SQLite and receipts on one PVC. Receipts included in any archived monthly report cannot be deleted, including after reopening (6-year retention for payroll records). Other receipts can be deleted with confirmation, even after checking; receipt numbers are never reused. `mahlzeit backup DIR` CLI and documented restore.

## Deferred (recorded, not decided)

- **More than 15 days per month**: optional higher day limit with an Arbeitstage calendar (Urlaub, Krank marked; other days count) as proof.
- **Backup to S3**: options are cluster volume backups (Longhorn/Velero/k8up), Litestream sidecar, or a built-in scheduled upload of DB snapshot + new files + reports, optionally encrypted. Owner decides later.
- **Automatic reports**: generate at month start, deliver by mail or S3.
- **Other meals**: breakfast or dinner instead of lunch (values already stored), see ADR 0002.
- **Notifications** (low priority): mail-import failures, "month ready to close".
- **Share target** (Android only; the owner uses iOS).
- **TypeScript 7.1**: drop the TypeScript 6 codegen package once 7.1 ships its programmatic API and Hey API supports it (2026-10-22: `7.1.0-beta`; also on the agent-presets watch list).
- **Further employees or companies**: the data model allows them; the UI does not.
