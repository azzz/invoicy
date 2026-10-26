cmd := "./cmd/invoicy"
bin := "bin/invoicy"

[private]
default: help

# Install the CLI into $(go env GOBIN) or $(go env GOPATH)/bin.
install:
    go install {{ cmd }}

# Run the CLI, for example: just run render invoices/invoice.yml
run *args:
    go run {{ cmd }} {{ args }}

# Build bin/invoicy.
build:
    go build -o {{ bin }} {{ cmd }}

# Remove build artifacts (bin/).
clean:
    rm -rf bin

# Run golangci-lint.
lint *args:
    bash scripts/lint.sh "$@"

# Format code with gofmt.
fmt:
    gofmt -w .

# Run all tests.
test:
    go test ./...

# Format, lint, and test.
check: fmt lint test

# Show the end-to-end invoicing flow and the available recipes.
help:
    #!/usr/bin/env bash
    b=$'\e[1m'; c=$'\e[36m'; d=$'\e[2m'; r=$'\e[0m'
    cat <<TXT
    ${b}Invoicy${r} ${d}- YAML in, single-page A4 PDF out${r}

    ${b}Workflow${r} ${d}(run from your workspace directory)${r}

      ${b}1. Initialize${r}  ${d}creates profiles/, invoices/, output/ and starter templates${r}
         ${c}invoicy init${r}

      ${b}2. Profile${r}     ${d}copy the template and fill in issuer, customer, bank, currency${r}
         ${c}cp profiles/profiles.yml profiles/customer-2026.yml${r}
         ${d}profiles are immutable once referenced; create a new version instead of editing${r}

      ${b}3. Invoice${r}     ${d}copies the profile's default works into a new specification${r}
         ${c}invoicy new profiles/customer-2026.yml --number INV-2026-09/01${r}
         ${d}then edit invoices/invoice-2026-09--01.yml: date, works, optional note, signature (PNG name from signatures/)${r}

      ${b}4. Validate${r}    ${d}checks the specification, profile, fonts, and one-page fit${r}
         ${c}invoicy validate invoices/invoice-2026-09--01.yml${r}

      ${b}5. Render${r}      ${d}writes output/INV-2026-09--01.pdf${r}
         ${c}invoicy render invoices/invoice-2026-09--01.yml${r}

    ${b}Workspace and defaults${r}

      ${c}--workspace DIR${r}            ${d}workspace for any command (profiles/, invoices/, output/, signatures/)${r}
      ${c}INVOICY_WORKSPACE_DIR${r}      ${d}same, from the environment; default is the current directory${r}
      ${c}INVOICY_DEFAULT_PROFILE${r}    ${d}profile for "invoicy new" when PROFILE is omitted${r}

    ${b}Recipes${r}

    TXT
    just --justfile "{{ justfile() }}" --list --unsorted --list-heading "" --list-prefix "  "
