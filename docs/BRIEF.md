# Brief

Terms: [CONTEXT.md](../CONTEXT.md). Architecture choices: [adr/](adr/). Tax rules: [CALCULATION_RULES.md](CALCULATION_RULES.md).

## Owner's case

One employee (the managing shareholder), lunch from the supermarket on most working days, 15 days per month, maximum allowance, tax model A (company pays the flat tax). Company in Baden-Württemberg.

## Phase 1

- Login only through OIDC (generic discovery, tested against Pocket ID), access by `sub`/email allowlist; break-glass admin token from env.
- Receipt import: photo or PDF upload (desktop and phone camera), and IMAP polling (every 5 min) of a dedicated mailbox, attachments from allowed senders only. Processed mails are marked read and moved to a folder; deleting them is an option.
- Claude vision fills date, total, merchant; every imported receipt is `zu prüfen` until the employee checks it. Missing date → warning, must be entered by hand. Duplicate detection by checksum.
- Calculation per month: one checked receipt per day by its own date; above the day limit (15) the highest totals count; allowance capped by receipt total; tax models A, B, C; yearly benefit values.
- Warnings, never blocks: weekend, public holiday (Bundesland of the company), second receipt on the same day.
- Month lifecycle `offen` → `abgeschlossen` → `ausgezahlt`; reopening needs a reason and creates report v2, v1 stays byte-identical.
- Monthly report PDF (Typst, layout borrowed from Spesen): summary with totals per tax part and who pays the tax, day table, every counted receipt appended.
- German UI (Paraglide ready for English), PWA, light/dark, usable from 360 px.
- Delivery: one OCI image (amd64/arm64, GHCR `ghcr.io/blackdark/vc-beleg-2`), Kubernetes single replica, SQLite and receipts on one PVC. Receipts are never deleted (6-year retention for payroll records). `mahlzeit backup DIR` CLI and documented restore.

## Deferred (recorded, not decided)

- **More than 15 days per month**: optional higher day limit with an Arbeitstage calendar (Urlaub, Krank marked; other days count) as proof.
- **Backup to S3**: options are cluster volume backups (Longhorn/Velero/k8up), Litestream sidecar, or a built-in scheduled upload of DB snapshot + new files + reports, optionally encrypted. Owner decides later.
- **Automatic reports**: generate at month start, deliver by mail or S3.
- **Other meals**: breakfast or dinner instead of lunch (values already stored), see ADR 0003.
- **Further employees or companies**: the data model allows them; the UI does not.
