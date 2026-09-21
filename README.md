# Invoicy

A local Go CLI for creating invoice specifications in YAML and rendering single-page A4 PDF invoices. No server, database, or invoice counter is required.

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

## Commands

| Command | Behavior |
| --- | --- |
| `invoicy init` | Creates `profiles/profiles.yml`, `invoices/invoice.yml`, a short `README.md`, and `output/`. Existing contents are preserved. |
| `invoicy new PROFILE [--number NUMBER] [--output FILE]` | Copies profile defaults into a new specification. Uses the local date and `INV-YYYY-MM/01` by default. Never replaces an existing file. |
| `invoicy validate SPECIFICATION` | Validates the specification, profile, font coverage, and one-page layout. Prints the calculated total. |
| `invoicy render SPECIFICATION [--output FILE]` | Validates and atomically writes a PDF. An existing PDF at the output path is replaced only after successful rendering. |
| `invoicy --help` | Prints usage. |

Options work before or after the input path. `--output=FILE` and `--number=NUMBER` are also accepted. Use `--` before an input path that starts with a hyphen. Errors go to stderr and return exit code 1.

Output parent directories must exist. Override paths may be absolute or relative to the current directory:

```bash
invoicy new profiles/customer-2026.yml \
  --number INV-2026-09/02 \
  --output invoices/second.yml
invoicy render invoices/second.yml --output output/custom-name.pdf
```

## Input rules

- Profile paths are relative to the **command's current directory**, including when the specification is stored elsewhere. Absolute profile paths and paths escaping the workspace, including through symlinks, are rejected.
- Keep referenced profiles immutable. Create a versioned profile when party or payment details change.
- `new` copies `default_works` verbatim. Existing specifications use only their own `works`; defaults are never merged during validation or rendering.
- Amounts must be quoted positive decimal strings, with at most two fractional digits. Calculations use signed 64-bit integer hundredths; values or totals exceeding that range are rejected. All currencies use two-decimal display, with no conversion or tax calculation.
- Currency codes contain three uppercase ASCII letters. IBANs require a recognized country length and a valid mod-97 checksum; spaces are accepted. SWIFT/BIC values must have the standard 8- or 11-character form. Validation does not verify account ownership or bank existence.
- Invoice dates use `YYYY-MM-DD`. Numbers contain up to 80 ASCII letters/digits, with single `.`, `_`, `/`, or `-` separators between groups. `INV-YYYY-MM/INDEX` is recommended. PDF filenames preserve the number exactly, replacing only `/` with `--` and adding `.pdf`. YAML template filenames lowercase the number, replace a leading `INV-` with `invoice-` (or add that prefix), and replace `/` with `--`.
- YAML must contain exactly one document, with known fields and no aliases. A stored `total` is rejected; totals are calculated from work items.
- The embedded Go fonts support Latin, Greek, and Cyrillic text. Unsupported characters and content exceeding one page produce a field-specific error.

Required profile fields and the full data model are defined in [`docs/implementation-plan.md`](docs/implementation-plan.md).

## Development

```bash
gofmt -w .
go test ./...
```

PDF integration tests also check extracted text and page dimensions when Poppler's `pdfinfo` and `pdftotext` are installed. Generated PDFs are ignored by Git.

Optional `issuer.bank.correspondent_bank.account` appears in the PDF. Use a quoted string (for example, `"000123456789"`) to preserve leading zeros.

Set `default_signature` in the profile to a PNG path (`"profiles/signature.png"`) or a manual-signature placeholder (`"________________"`).
Values ending in `.png` (case-insensitive) are image paths; other nonempty strings are printed after `Signature:`.
`new` copies it into the invoice's `signature` field; rendering uses only that field.
Omit or empty `signature` to hide the entire signature block. PNG paths are relative to
the working directory and must stay inside it. Transparent PNGs are supported and
scaled proportionally to fit 50 × 20 mm. Keep referenced images unchanged; use a new
filename for a new signature. The image is not a cryptographic digital signature.
