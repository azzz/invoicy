# Invoicy

Run commands from the workspace directory. Replace example party and bank details before use.

| Command | What it does |
| --- | --- |
| `invoicy init` | Creates example YAML files, this README, and `output/`; preserves existing files. |
| `invoicy new profiles/profiles.yml [--number INV-2026-09/02] [--output invoices/next.yml]` | Copies `default_works` into a new invoice; defaults to today's date and `INV-YYYY-MM/01`. Never overwrites files or increments numbers. |
| `invoicy validate invoices/invoice.yml` | Checks the invoice, profile, and one-page layout; prints the total. |
| `invoicy render invoices/invoice.yml [--output output/invoice.pdf]` | Writes `output/NUMBER.pdf`, replacing only `/` with `--` in the number; replaces an existing PDF. |
| `invoicy --help` | Shows command usage. |

`profiles/profiles.yml` holds issuer, customer, bank details, currency, and default work items.
`invoices/invoice.yml` holds the profile path (relative to the working directory), number, date (`YYYY-MM-DD`), and final work items.
Amounts are quoted positive decimals with at most two decimal places; totals are calculated, never stored.
Create a versioned profile when details change after use. Output directories must exist.

**profiles/profiles.yml**

```yaml
issuer:
  name: Example Issuer Ltd
  address:
    - 10 Example Street
    - Berlin, 10115
    - Germany
  tax_id: EXAMPLE-TAX-ID
  bank:
    beneficiary: Example Issuer Ltd
    iban: DE89370400440532013000
    swift_bic: COBADEFFXXX
    name: Example Bank
    address:
      - 20 Bank Street
      - Berlin, 10117
      - Germany
    correspondent_bank:
      name: Example Correspondent Bank
      account: "000123456789"
      swift_bic: DEUTDEFF

customer:
  name: Example Customer Ltd
  address:
    - 30 Customer Street
    - Dublin, D02 EX00
    - Ireland
  vat_number: EXAMPLE-VAT-NUMBER

currency: EUR

# Optional: PNG path (profiles/signature.png), manual placeholder ("________________"), or empty.
default_signature: ""

default_works:
  - description: Software development and consulting
    amount: "2293.00"
```

**invoices/invoice.yml**

```yaml
profile: profiles/profiles.yml
number: INV-2026-09/01
date: 2026-09-21
signature: "" # PNG path or manual placeholder; empty or omitted hides the signature.
works:
  - description: Software development and consulting
    amount: "2293.00"
```

Optional `issuer.bank.correspondent_bank.account` appears in the PDF. Use a quoted string (for example, `"000123456789"`) to preserve leading zeros.

Set `default_signature` in the profile to a PNG path (`"profiles/signature.png"`) or a manual-signature placeholder (`"________________"`).
Values ending in `.png` (case-insensitive) are image paths; other nonempty strings are printed after `Signature:`.
`new` copies it into the invoice's `signature` field; rendering uses only that field.
Omit or empty `signature` to hide the entire signature block. PNG paths are relative to
the working directory and must stay inside it. Transparent PNGs are supported and
scaled proportionally to fit 50 × 20 mm. Keep referenced images unchanged; use a new
filename for a new signature. The image is not a cryptographic digital signature.
