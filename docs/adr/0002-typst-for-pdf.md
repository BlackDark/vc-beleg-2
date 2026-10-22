# Typst CLI renders the monthly report

Reports are rendered by the pinned Typst CLI with bundled Inter fonts, a deadline and no network, as in Spesen. It embeds receipt images and PDF pages directly and produces the best-looking output. Cost: about 40 MB more image size and a binary pinned in the Dockerfile and `mise.toml`.

## Considered Options

- Pure-Go PDF libraries (maroto, gofpdf): no external binary, but plainer layout and awkward embedding of PDF receipts.
