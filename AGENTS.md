# AGENTS.md

## Service Summary

Invoicy is a standalone local Go CLI that initializes workspaces, creates invoice specification templates, and renders single-page PDF invoices from YAML profiles and invoice specifications. The project has no web service, database, or automatic invoice sequence tracking.

Read `docs/implementation-plan.md` before changing data formats, package boundaries, or delivery scope.

## Service Map

```text
cmd/invoicy       CLI entry point, commands, flags, and user-facing errors
internal/config   Profile parsing and validation
internal/invoice  Specification creation, parsing, profile resolution, and totals
internal/render   Single-page A4 PDF rendering
profiles          Immutable issuer/customer profiles and default work items
invoices          Historical invoice specifications
output            Generated PDF files
docs              Design and implementation documentation
```

## Core Rules

- Treat YAML files as the source of truth and PDFs as generated artifacts.
- Do not modify a profile after an invoice references it; create a versioned profile instead.
- Resolve profile paths from the command's current working directory, not from the invoice specification's directory.
- Copy `default_works` into new invoice specifications. Never merge defaults during validation or rendering.
- Calculate totals from `works`; do not persist a total in YAML.
- Do not use binary floating-point types for money.
- Reject content that does not fit on one A4 page.
- Keep template creation, parsing, validation, calculation, and PDF rendering separate.
- Keep all repository documentation and code comments in English.

## Verification

Run formatting and tests before completing a change:

```bash
gofmt -w .
go test ./...
```
