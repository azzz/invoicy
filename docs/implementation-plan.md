# Invoicy Implementation Plan

## Goal

Invoicy is a standalone local Go application that creates compact, single-page PDF invoices from YAML files. It can be installed and used from any directory without access to its source repository. It has no web interface, database, or automatic invoice sequence storage.

The application is a non-interactive command-line tool that initializes workspaces, creates invoice specification templates, validates YAML specifications, and renders PDF invoices.

The YAML files are the source of truth. Generated PDFs are disposable artifacts that can be reproduced from the same profile and invoice specification.

## Source Repository Layout

```text
.
├── cmd/
│   └── invoicy/
│       └── main.go
├── internal/
│   ├── config/
│   ├── invoice/
│   └── render/
├── docs/
│   └── implementation-plan.md
├── AGENTS.md
├── go.mod
└── README.md
```

Runtime data does not depend on the source repository. A user initializes a workspace in any directory with:

```bash
invoicy init
```

The command creates this structure relative to the current working directory:

```text
.
├── profiles/profiles.yml
├── invoices/invoice.yml
├── README.md
└── output/
```

`init` succeeds when any of these directories already exist and leaves their contents unchanged. It returns an error when one of the paths exists but is not a directory. It creates an example profile at `profiles/profiles.yml`, an invoice specification at `invoices/invoice.yml`, and a short `README.md` with commands and YAML examples. These templates are embedded in the binary. Existing files are never replaced.

`profiles/`, `invoices/`, and `output/` are the default workspace locations, not hard requirements. CLI options allow callers to override input and output paths.

## Data Model

### Profile

A profile contains the issuer, customer, currency, payment details, and work items commonly used for that customer.

```yaml
issuer:
  name: Example Issuer Ltd
  address:
    - 10 Example Street
    - Example City, EX 1000
    - Example Country
  tax_id: ISSUER-TAX-ID
  bank:
    beneficiary: Example Issuer Ltd
    iban: ISSUER-IBAN
    swift_bic: ISSUER-SWIFT-BIC
    name: Example Bank
    address:
      - 20 Bank Street
      - Example City, EX 2000
      - Example Country
    correspondent_bank:
      name: Example Correspondent Bank
      account: "000123456789"
      swift_bic: CORRESPONDENT-SWIFT-BIC

customer:
  name: Example Customer Ltd
  address:
    - 30 Customer Street
    - Customer City, CU 3000
    - Customer Country
  vat_number: CUSTOMER-VAT-NUMBER

currency: EUR

# Optional: PNG path (profiles/signature.png), manual placeholder ("________________"), or empty.
default_signature: ""

default_works:
  - description: Software development and consulting
    amount: "2293.00"
```

`issuer` is the person or company issuing the invoice and receiving payment. `customer` is the company being billed. The values above are placeholders, not real personal or company data.

The issuer and customer names and addresses are required. The issuer bank beneficiary, IBAN, SWIFT/BIC, name, and address are also required. The bank name and address are separate fields so the PDF can label them correctly. `correspondent_bank` is optional. When present, both its `name` and `swift_bic` are required. Its optional `account` is a string printed as the correspondent account in the PDF; quote it to preserve leading zeros. `customer.vat_number` is optional and is printed with the billing address when provided. `default_works` must contain at least one work item with a non-empty description and a positive decimal amount.

Profiles are immutable after they have been referenced by an invoice specification. When details change, create a new profile such as `companyname-2027.yml` instead of editing `companyname-2026.yml`. This keeps old invoices reproducible.

### Invoice Specification

An invoice specification contains all fields that vary between invoices.

```yaml
profile: profiles/companyname-2026.yml

number: INV-2026-09/01
date: 2026-09-21
signature: "" # PNG path or manual placeholder; empty or omitted hides the signature.

works:
  - description: Software development and consulting
    amount: "2293.00"
  - description: Travel expenses reimbursement
    amount: "125.40"
  - description: Additional development work
    amount: "300.00"
```

`profile` is a relative path resolved from the command's current working directory, not from the invoice specification's directory. Nested paths such as `profiles/customers/companyname-2026.yml` are supported. Absolute paths and relative paths that resolve outside the working directory are rejected so the workspace remains self-contained.

The `works` list is final and is the only source of invoice line items during validation and rendering. The template command copies `default_works` into `works`, after which the two lists are independent. Editing a generated specification never changes its profile, and later profile changes never alter an existing specification.

The optional profile `default_signature` is copied verbatim into the invoice's `signature` by `new`, just like work defaults. Validation and rendering never fall back to the profile default. An empty or omitted invoice `signature` hides the signature block. A value ending in `.png` (case-insensitive) is a PNG path resolved from the working directory with the same workspace confinement as profile paths, including symlinks. Missing or invalid images are errors before writing output. Transparent PNGs are supported and scaled proportionally to fit 50 × 20 mm. Any other nonempty string is printed verbatim after `Signature:` as a manual-signature placeholder, for example `"________________"`. Both text and image signatures count toward the single-page limit. Keep referenced signature images immutable and use versioned filenames for changes. Only the path is copied, not the image bytes. This is an image, not a cryptographic digital signature.

The total is derived from `works` and is never stored in the specification. Amounts are quoted decimal strings so the application can parse them without binary floating-point rounding.

The recommended invoice number format is `INV-{YYYY}-{MM}/{INDEX}`. The number and date are set explicitly in each specification. The application does not persist or increment the index automatically.

The slash in the invoice number cannot be used directly in a filename. The default PDF path preserves the number exactly, converting only `/` to `--` and appending `.pdf`:

```text
INV-2026-09/01 -> output/INV-2026-09--01.pdf
```

## Command-Line Application

### Scope

The application provides a deterministic, non-interactive CLI. Users can generate invoice specification templates from profiles or create specifications manually, then pass a specification to the validator or renderer.

Initial commands:

```bash
invoicy init
invoicy new profiles/companyname-2026.yml
invoicy validate invoices/invoice-2026-09--01.yml
invoicy render invoices/invoice-2026-09--01.yml
```

`init` prepares the current directory as an Invoicy workspace without requiring the source repository or a configuration file.

`new` loads a profile and creates an invoice specification template under `invoices/`.

`validate` loads and validates both the invoice specification and its profile without creating a PDF.

`render` performs the same validation and writes a PDF. It resolves the specification's `profile` path from the current working directory and writes the PDF under `output/` in that directory by default. Moving the invoice specification does not change which profile it references, but running the command from another directory does.

Useful overrides:

```bash
invoicy new profiles/companyname-2026.yml \
  --number INV-2026-09/02 \
  --output invoices/invoice-2026-09--02.yml

invoicy render invoices/invoice-2026-09--01.yml \
  --output output/custom-name.pdf
```

### Specification Template Generation

`new` accepts a profile path relative to the current working directory. It validates the profile before creating a specification and writes the same relative path into the specification's `profile` field.

By default, `new` uses the local current date for `date`, generates `INV-{YYYY}-{MM}/01` for `number`, and derives `invoices/invoice-{YYYY}-{MM}--01.yml` as the output path. The optional `--number` flag replaces the generated number and changes the derived filename accordingly. The optional `--output` flag replaces that derived path. The command does not inspect existing invoices or increment the index.

The command copies every profile `default_works` item into the specification's `works` list without changing descriptions or amounts and without adding or merging items. The generated specification must pass the same validation as an input accepted by `validate`.

`new` requires the output parent directory to exist. It returns an error instead of replacing an existing file. The completed YAML is written atomically so a failed operation does not leave a partial specification.

### Validation

The generator will reject input when:

- the referenced profile does not exist;
- the profile path is absolute or resolves outside the current working directory;
- the invoice number or date is missing or malformed;
- issuer, customer, currency, or required issuer bank fields are missing;
- an IBAN or SWIFT/BIC is malformed;
- a correspondent bank is present without both its name and SWIFT/BIC;
- `works` is empty;
- a work description is empty;
- an amount is malformed, zero, or negative;
- the content cannot fit on one A4 page.

Errors must identify the input file and field. Validation must finish before creating or replacing an output file.

### Money Handling

Amounts will use decimal arithmetic or integer minor units. Application code must not use `float32` or `float64` for invoice calculations.

Currency formatting is driven by the profile currency. The initial version only needs standard two-decimal formatting and does not perform currency conversion.

### PDF Layout

The renderer produces one A4 page with:

1. Issuer details and the invoice number.
2. Issuer bank details, including the beneficiary, IBAN, SWIFT/BIC, bank name, bank address, and optional correspondent bank.
3. Customer details under a `BILL TO` heading, including the VAT number when provided.
4. A table containing work descriptions and amounts.
5. A calculated total.
6. The invoice date and an optional signature image or manual-signature placeholder, with no signature block when omitted.

The layout should follow the supplied example while using consistent margins, typography, and thin table borders. Content that exceeds one page is an error rather than an implicit second page.

### Internal Packages

- `internal/config` loads and validates profiles.
- `internal/invoice` creates and loads invoice specifications, resolves profiles, and calculates totals.
- `internal/render` renders a validated invoice as PDF.
- `cmd/invoicy` defines commands, flags, exit codes, and user-facing errors.

Template creation, parsing, validation, calculation, and rendering stay separate so each concern can be tested independently.

### Tests

The application includes tests for:

- workspace initialization in an empty directory;
- repeated workspace initialization without replacing existing files;
- rejection when a workspace path exists as a non-directory;
- valid and invalid profile parsing;
- rejection of empty or invalid `default_works`;
- optional customer VAT numbers;
- optional correspondent bank details and rejection of partial correspondent bank details;
- IBAN and SWIFT/BIC validation;
- valid and invalid invoice specification parsing;
- exact decimal addition and display formatting;
- profile path resolution from the current working directory;
- nested profile paths and rejection of absolute or workspace-escaping paths;
- profile resolution that does not depend on the invoice specification's directory;
- specification template defaults using a fixed local date;
- exact copying of `default_works` into `works`;
- specification generation with `--number` and `--output` overrides;
- refusal to replace an existing specification;
- validation before writing a generated specification;
- default output filename generation;
- invoice number validation;
- rejection of empty work lists and invalid amounts;
- successful PDF generation;
- rejection when content exceeds one page.

A checked-in example profile and invoice specification will act as fixtures. Generated PDF files should not be committed unless a visual regression workflow is introduced later.

### Completion Criteria

The application is complete when:

- `go test ./...` passes;
- the installed `invoicy` binary can initialize and use a workspace in any directory without access to the source repository;
- `invoicy init` creates `profiles/`, `invoices/`, and `output/` without replacing existing contents;
- `invoicy new` creates a valid specification with the current date, the default `/01` invoice number, and an exact copy of the profile's default work items;
- template generation never infers the next invoice index or replaces an existing specification;
- the example specification renders without interaction;
- the PDF contains all profile and invoice fields with the correct total;
- the generated document is a single A4 page;
- invalid input returns a non-zero exit code and a specific error;
- rerendering unchanged inputs produces the same invoice content.

## Deferred Features

The initial version intentionally excludes:

- a web interface;
- a database;
- automatic invoice sequence tracking;
- automatic profile mutation or migration;
- tax calculation;
- currency conversion;
- multi-page invoices;
- sending invoices by email;
- importing data from accounting systems.

These features should only be added in response to a concrete use case.
