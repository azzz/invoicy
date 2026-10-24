# Invoicy

A local Go CLI for creating invoice specifications in YAML and rendering single-page A4 PDF invoices. No server, database, or invoice counter is required.

See the [example PDF invoice](examples/output/INV-2026-09--01.pdf).

## Getting started

After cloning the repository, run this first:

```bash
just help
```

It prints the whole workflow (`init`, profile, `new`, `validate`, `render`) and every development recipe (`just build`, `just check`, ...). [just](https://github.com/casey/just) is the task runner used here; if you do not have it, everything below works without it.

## Install

Requires Go 1.26.4 or later.

```bash
go install github.com/azzz/invoicy/cmd/invoicy@latest
```

From a source checkout:

```bash
go install ./cmd/invoicy
```

Put Go's binary directory (`$(go env GOPATH)/bin` by default) on your `PATH`. The installed binary includes its fonts and works without the source checkout.

## Create an invoice

Run these commands from your chosen workspace directory:

```bash
invoicy init
# Copy profiles/profiles.yml to profiles/customer-2026.yml and edit the details.
invoicy new profiles/customer-2026.yml --number INV-2026-09/01
# Edit the generated specification's date and work items as needed.
invoicy validate invoices/invoice-2026-09--01.yml
invoicy render invoices/invoice-2026-09--01.yml
```

The PDF is written to `output/INV-2026-09--01.pdf`.

Use [`examples/profiles/example-2026.yml`](examples/profiles/example-2026.yml) as the profile template and [`examples/invoices/invoice-2026-09--01.yml`](examples/invoices/invoice-2026-09--01.yml) as a specification example. The example contains fictional party details and a public sample IBAN; replace all payment and party details before use.

To render the checked-in example, run these commands **from `examples/`**:

```bash
invoicy init
invoicy validate invoices/invoice-2026-09--01.yml
invoicy render invoices/invoice-2026-09--01.yml
```

## Workspace and defaults

All data lives in one **workspace** directory with fixed subdirectories `profiles/`, `invoices/`, and `output/`. Relative paths (profile paths, invoice files, `--output`) are resolved from the workspace. Choose it with, in order of precedence:

1. the `--workspace DIR` flag (any command);
2. the `INVOICY_WORKSPACE_DIR` environment variable (absolute, or relative to the current directory);
3. the current directory.

`INVOICY_DEFAULT_PROFILE` sets the profile used by `invoicy new` when `PROFILE` is omitted. `PROFILE` and `INVOICY_DEFAULT_PROFILE` accept either a path or a bare name: `mario` (no `/` or `.`) means `profiles/mario.yml`. An explicit argument always wins.

```bash
export INVOICY_WORKSPACE_DIR=~/invoices   # for example in .envrc or ~/.zshrc
export INVOICY_DEFAULT_PROFILE=mario
invoicy init
invoicy new --number INV-2026-10/01
```

## Commands

| Command | Behavior |
| --- | --- |
| `invoicy init [--workspace DIR]` | Creates `profiles/profiles.yml`, `invoices/invoice.yml`, a short `README.md`, and the `output/` and `signatures/` directories. Existing contents are preserved. |
| `invoicy new [PROFILE] [--number NUMBER] [--output FILE]` | Copies profile defaults into a new specification. `PROFILE` may come from the configuration. Uses the local date and `INV-YYYY-MM/01` by default. Never replaces an existing file. |
| `invoicy validate SPECIFICATION` | Validates the specification, profile, font coverage, and one-page layout. Prints the calculated total. |
| `invoicy render SPECIFICATION [--output FILE]` | Validates and writes a PDF. An existing PDF at the output path is replaced only after successful rendering. |
| `invoicy --help` | Prints usage. |

Options work before or after the input path. `--output=FILE` and `--number=NUMBER` are also accepted. Use `--` before an input path that starts with a hyphen. Errors go to stderr and return exit code 1.

Output parent directories must exist. Override paths may be absolute or relative to the workspace:

```bash
invoicy new profiles/customer-2026.yml \
  --number INV-2026-09/02 \
  --output invoices/second.yml
invoicy render invoices/second.yml --output output/custom-name.pdf
```

## Input rules

- Profile paths are relative to the **workspace** (the current directory unless `--workspace` or `INVOICY_WORKSPACE_DIR` says otherwise), including when the specification is stored elsewhere. Absolute profile paths and paths escaping the workspace, including through symlinks, are rejected.
- Keep referenced profiles immutable. Create a versioned profile when party or payment details change.
- `new` copies `default_works` verbatim. Existing specifications use only their own `works`; defaults are never merged during validation or rendering.
- Amounts must be quoted positive decimal strings, with at most two fractional digits. Calculations use signed 64-bit integer hundredths; values or totals exceeding that range are rejected. All currencies use two-decimal display, with no conversion or tax calculation.
- Currency codes contain three uppercase ASCII letters. IBANs require a recognized country length and a valid mod-97 checksum; spaces are accepted. SWIFT/BIC values must have the standard 8- or 11-character form. Validation does not verify account ownership or bank existence.
- Invoice dates use `YYYY-MM-DD`. Numbers contain up to 80 ASCII letters/digits, with single `.`, `_`, `/`, or `-` separators between groups. `INV-YYYY-MM/INDEX` is recommended. PDF filenames preserve the number exactly, replacing only `/` with `--` and adding `.pdf`. YAML template filenames lowercase the number, replace a leading `INV-` with `invoice-` (or add that prefix), and replace `/` with `--`.
- YAML must contain exactly one document, with known fields and no aliases. A stored `total` is rejected; totals are calculated from work items.
- The embedded Go fonts support Latin, Greek, and Cyrillic text. Unsupported characters and content exceeding one page produce a field-specific error.

Required profile fields and the full data model are defined in [`docs/design.md`](docs/design.md).

## Development

```bash
just check   # gofmt, golangci-lint, and go test ./...
```

Without `just`: `gofmt -w . && go test ./...`.

PDF integration tests also check extracted text and page dimensions when Poppler's `pdfinfo` and `pdftotext` are installed. Generated PDFs are ignored by Git, except for the checked-in example.

Optional `issuer.bank.correspondent_bank.account` appears in the PDF. Use a quoted string (for example, `"000123456789"`) to preserve leading zeros.

Put signature images in `signatures/` and set `default_signature` in the profile (or `signature` in the invoice) to the file name (`"mario.png"`), or to a manual-signature placeholder (`"________________"`).
Values ending in `.png` (case-insensitive) are images: a bare name is read from `signatures/`, while a value with a `/` is a path relative to the workspace. Other nonempty strings are printed after `Signature:`.
`new` copies it into the invoice's `signature` field; rendering uses only that field.
Omit or empty `signature` to hide the entire signature block. Image paths must stay inside the workspace. Transparent PNGs are supported and
scaled proportionally to fit 50 × 20 mm. Keep referenced images unchanged; use a new
filename for a new signature. The image is not a cryptographic digital signature.
