# Mahlzeit

Self-hosted app for one managing shareholder: records meal receipts, computes the monthly meal allowance (Essenszuschuss) and archives each month as a PDF report.

## Verify

`mise run check` green before every commit; `mise run e2e` (Playwright, `web/e2e`) after UI changes.

## Stack

Follows https://github.com/BlackDark/agent-presets/blob/main/stack/core.md.
Deviations: Go backend (Huma, code-first OpenAPI) + React SPA with a Hey API client instead of TanStack Start; Typst CLI for PDFs. Why: [docs/adr/0001](docs/adr/0001-stack-and-separate-app.md). Hey API runs with TypeScript 6 in `web/tools/codegen` until TypeScript 7.1.

## Map

- Read first: [GLOSSARY.md](GLOSSARY.md) (use its English terms in code), [docs/BRIEF.md](docs/BRIEF.md) (scope, deferred items), [docs/CALCULATION_RULES.md](docs/CALCULATION_RULES.md) (rule IDs R1–R8).
- `internal/calc`: the only place for tax rules; pure functions, day lines carry rule IDs.
- `internal/rates/benefit-values.csv`: benefit values per year; a new year is a new row, never code.
- `internal/holidays`: public holidays per Bundesland.
- `internal/app`: persistence, receipts, months and reports (one account, one employee); `internal/api`: Huma operations, the OpenAPI document is generated from them; `internal/export`: Typst report template.
- `internal/extract`: Claude reads receipts; `internal/mailimport`: IMAP polling. Both only suggest values: every receipt is checked by the user.
- `web/`: React SPA, embedded in the binary; `web/src/api/gen` is generated (`mise run gen`), never edit it. UI text in `web/messages/de.json` and `en.json` (keep both complete).
- `docker/Dockerfile`, `.github/workflows/release.yml` (tag `v*` → GHCR image + release), `deploy/kubernetes.yaml`, [docs/OPERATIONS.md](docs/OPERATIONS.md) (all env variables).
- Much of the server and UI is adapted from Spesen (github.com/BlackDark/vc-geschaeftsausgabentest); check there before inventing a pattern.

## Every change

Same PR, only what the change touches:
- tests for the changed behaviour (Go table tests for rules, `internal/app` flow tests with real SQLite and Typst, `internal/api` tests for auth, Playwright in `web/e2e` for user flows)
- `docs/CALCULATION_RULES.md` when amounts change, `docs/BRIEF.md` when scope changes, `GLOSSARY.md` for new terms
- README when a user-visible feature changes, `docs/OPERATIONS.md` when configuration changes
- after API changes: `mise run gen`, commit `web/openapi.json` and `web/src/api/gen`

## Code style

- Data in, data out: pure functions, side effects (I/O, time) at the edges and passed in.
- Expected failures are values (`error`, error unions in TS); panic for bugs only.
- Money is integer cents (`money.Cents`), never floats.
- Small units: split a file past ~400 lines, a function past ~60.
- Names explain what, comments explain why.

## Git

- Conventional Commits. One topic per PR; open it ready for review (drafts skip CI), merge when CI is green.
- Merge: rebase when the commits are logically separate, squash otherwise.
- PR already merged → branch anew from `origin/main`, open a new PR.

## Boundaries

- `sudo`, system packages, global installs: the user runs them; ask with what and why.
- Ask before changes that alter financial results or scope.
- Language: code, comments, docs, commits in English; UI in German and English; the PDF report stays German.
