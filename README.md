<div align="center">

<h1>Mahlzeit: meal allowance receipts and monthly reports</h1>

**Self-hosted app for German meal allowances (Essenszuschuss) paid to yourself as managing shareholder.**<br>
Snap or mail your lunch receipts, let it pick the days that count, and archive a clean monthly PDF.

[![CI](https://github.com/BlackDark/vc-beleg-2/actions/workflows/ci.yml/badge.svg)](https://github.com/BlackDark/vc-beleg-2/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)

</div>

> Work in progress. Phase 1 scope: [docs/BRIEF.md](docs/BRIEF.md).

## Why

The rules are simple until they aren't: one allowance per working day, at most €7.67 in 2026, at most 15 days without proof, €4.57 of it taxable, the receipt caps the amount. Mahlzeit applies them to your receipts and documents every month.

- **Correct amounts**: 2026 benefit values, tax models A (company pays 25 % flat tax), B (payroll) and C (tax-free top-up only), rule IDs on every day.
- **Receipts in, no typing**: photo or PDF upload, or mail them to a dedicated mailbox; Claude reads date, total and merchant, you confirm.
- **Monthly PDF**: summary, day table and every receipt, versioned when a month is reopened.

## Documentation

- [Brief](docs/BRIEF.md): scope and deferred features
- [Calculation rules](docs/CALCULATION_RULES.md) with sources and open questions
- [Glossary](GLOSSARY.md), [decisions](docs/adr/)

Not tax advice; confirm the rules with your tax adviser.

## Development

Tools come from [`mise.toml`](mise.toml):

```bash
mise install
mise run check
```

License: [AGPL-3.0-only](LICENSE).
