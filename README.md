<div align="center">

<h1>Mahlzeit: meal allowance receipts and monthly reports</h1>

**Self-hosted app for German meal allowances (Essenszuschuss) paid to yourself as managing shareholder.**<br>
Snap or mail your lunch receipts, let it pick the days that count, and archive a clean monthly PDF.

[![CI](https://github.com/BlackDark/vc-beleg-2/actions/workflows/ci.yml/badge.svg)](https://github.com/BlackDark/vc-beleg-2/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)

</div>

## Why

The rules are simple until they aren't: one allowance per working day, at most €7.67 in 2026, at most 15 days without proof, the receipt caps the amount, and up to €4.57 of it (the benefit value) is taxable. Mahlzeit applies them to your receipts and documents every month.

- **Correct amounts**: 2026 benefit values, tax models A (company pays 25 % flat tax on the taxable part), B (taxable part through payroll) and C (allowance capped at the €3.10 top-up; tax-free when your own share covers the benefit value, otherwise the rest goes through payroll), lunch by default or breakfast or dinner per day, more than 15 days with a working-day calendar, rule IDs on every day.
- **Receipts in, no typing**: photo or PDF upload, or mail them to a dedicated mailbox; Claude reads date, total and merchant, you confirm.
- **Monthly PDF**: summary, day table and every receipt, versioned when a month is reopened.
- **Phone and desktop**: installable web app in German or English, from 360 px wide.

## Install

One container with one volume, images for amd64 and arm64 on GHCR:

```text
ghcr.io/blackdark/vc-beleg-2:<version>
```

Kubernetes example: [deploy/kubernetes.yaml](deploy/kubernetes.yaml). Configuration, Pocket ID, mail import, backup: [docs/OPERATIONS.md](docs/OPERATIONS.md).

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
