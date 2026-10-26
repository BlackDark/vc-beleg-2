<div align="center">

<h1>Mahlzeit: meal allowance receipts and monthly reports</h1>

**Self-hosted app for German meal allowances (Essenszuschuss) paid to yourself as managing shareholder.**<br>
Snap or mail your lunch receipts, let it pick the days that count, and archive a clean monthly PDF.

[![CI](https://github.com/BlackDark/vc-beleg-2/actions/workflows/ci.yml/badge.svg)](https://github.com/BlackDark/vc-beleg-2/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)

</div>

## Why

The rules are simple until they aren't: one allowance per working day, at most €7.67 in 2026, at most 15 days without proof, the receipt caps the amount, and up to €4.57 of it (the benefit value) is taxable. Mahlzeit applies them to your receipts and documents every month.

- **Correct amounts**: 2026 benefit values, tax models A (company pays 25 % flat tax on the taxable part), B (taxable part through payroll) and C (allowance capped at the €3.10 top-up; tax-free when your own share covers the benefit value, otherwise the rest goes through payroll), rule IDs on every day.
- **Receipts in, no typing**: photo or PDF upload, mail them to a dedicated mailbox, or drop them into a WebDAV folder (Nextcloud, NAS); an LLM (Claude, Gemini or any OpenAI-compatible API such as OpenRouter or a local Ollama) reads date, total and merchant, you confirm. Reading runs one receipt at a time, survives restarts, and shows queued, processing or failed status with retry after failure.
- **Receipt deletion**: delete mistaken or duplicate receipts with confirmation until they appear in an archived monthly report. Archived evidence stays protected, including after reopening a month.
- **Multiple receipts per day**: use the highest checked receipt automatically, choose one explicitly, or combine receipts for the same meal.
- **Monthly PDF**: summary, day table and every receipt, versioned when a month is reopened.
- **Organized settings**: separate sections for company and allowance, account and appearance, integrations, and import log.
- **Phone and desktop**: installable web app in German or English, from 360 px wide.

## Install

One container with one volume, images for amd64 and arm64 on GHCR:

```text
ghcr.io/blackdark/vc-beleg-2:<version>
```

Kubernetes example: [deploy/kubernetes.yaml](deploy/kubernetes.yaml). Configuration, Pocket ID, mail and folder import, backup: [docs/OPERATIONS.md](docs/OPERATIONS.md).

## Documentation

- [Brief](docs/BRIEF.md): scope and deferred features
- [Calculation rules](docs/CALCULATION_RULES.md) with sources and open questions
- [Glossary](GLOSSARY.md), [decisions](docs/adr/)

Not tax advice; confirm the rules with your tax adviser.

## Development

Tools come from [`mise.toml`](mise.toml) (Go, Node, pnpm, Typst):

```bash
mise install
pnpm --dir web install
mise run check        # gofmt, go vet, go test, Biome, tsc, Vitest
mise run gen          # after API changes: OpenAPI document + TypeScript client
mise run dev          # server on :8080; `pnpm --dir web dev` serves the UI with hot reload
```

License: [AGPL-3.0-only](LICENSE).
