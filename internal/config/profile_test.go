package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileValidation(t *testing.T) {
	cases := []struct {
		name, field string
		change      func(*Profile)
	}{
		{"valid", "", func(p *Profile) {}},
		{"optional", "", func(p *Profile) { p.Customer.VATNumber = ""; p.Issuer.TaxID = ""; p.Issuer.Bank.Correspondent = nil }},
		{"issuer", "issuer.name", func(p *Profile) { p.Issuer.Name = " " }},
		{"issuer address", "issuer.address", func(p *Profile) { p.Issuer.Address = nil }},
		{"customer", "customer.name", func(p *Profile) { p.Customer.Name = "" }},
		{"customer address", "customer.address[0]", func(p *Profile) { p.Customer.Address = []string{" "} }},
		{"currency", "currency", func(p *Profile) { p.Currency = "eur" }},
		{"beneficiary", "issuer.bank.beneficiary", func(p *Profile) { p.Issuer.Bank.Beneficiary = "" }},
		{"bank name", "issuer.bank.name", func(p *Profile) { p.Issuer.Bank.Name = "" }},
		{"bank address", "issuer.bank.address", func(p *Profile) { p.Issuer.Bank.Address = nil }},
		{"iban checksum", "issuer.bank.iban", func(p *Profile) { p.Issuer.Bank.IBAN = "DE88370400440532013000" }},
		{"iban length", "issuer.bank.iban", func(p *Profile) { p.Issuer.Bank.IBAN = "DE893704004405320130000" }},
		{"iban country", "issuer.bank.iban", func(p *Profile) { p.Issuer.Bank.IBAN = "ZZ89370400440532013000" }},
		{"iban spaces", "", func(p *Profile) { p.Issuer.Bank.IBAN = "DE89 3704 0044 0532 0130 00" }},
		{"bic short", "issuer.bank.swift_bic", func(p *Profile) { p.Issuer.Bank.SWIFTBIC = "COBADE" }},
		{"bic invalid country", "issuer.bank.swift_bic", func(p *Profile) { p.Issuer.Bank.SWIFTBIC = "COBA11FF" }},
		{"correspondent name", "correspondent_bank.name", func(p *Profile) { p.Issuer.Bank.Correspondent.Name = "" }},
		{"correspondent account omitted", "", func(p *Profile) { p.Issuer.Bank.Correspondent.Account = "" }},
		{"correspondent account control", "correspondent_bank.account", func(p *Profile) { p.Issuer.Bank.Correspondent.Account = "abc\x00" }},
		{"correspondent bic", "correspondent_bank.swift_bic", func(p *Profile) { p.Issuer.Bank.Correspondent.SWIFTBIC = "" }},
		{"defaults empty", "default_works", func(p *Profile) { p.DefaultWorks = nil }},
		{"description empty", "default_works[0].description", func(p *Profile) { p.DefaultWorks[0].Description = " " }},
		{"amount zero", "default_works[0].amount", func(p *Profile) { p.DefaultWorks[0].Amount = "0" }},
		{"control character", "issuer.name", func(p *Profile) { p.Issuer.Name = "x\x00" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Load("../../examples/profiles/example-2026.yml")
			if err != nil {
				t.Fatal(err)
			}
			tc.change(p)
			err = p.Validate()
			if tc.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("got %v, want field %s", err, tc.field)
			}
		})
	}
}

func TestStrictProfileYAML(t *testing.T) {
	data, err := os.ReadFile("../../examples/profiles/example-2026.yml")
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	for _, tc := range []struct{ name, data, want string }{
		{"numeric amount", strings.Replace(valid, `"2293.00"`, "2293.00", 1), "default_works[0].amount"},
		{"unknown field", valid + "total: 2293\n", "field total"},
		{"unknown nested field", strings.Replace(valid, "  tax_id:", "  typo:", 1), "field typo"},
		{"duplicate field", valid + "currency: USD\n", "already defined"},
		{"extra document", valid + "---\n{}\n", "one YAML document"},
		{"malformed", "issuer: [", "YAML"},
		{"empty", "", "YAML"},
		{"alias", strings.Replace(valid, "name: Example Issuer Ltd", "name: &issuer Example Issuer Ltd", 1) + "extra: *issuer\n", "aliases"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.yml")
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want file and %s", err, tc.want)
			}
		})
	}
}

func TestCorrespondentAccount(t *testing.T) {
	p, err := Load("../../examples/profiles/example-2026.yml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Issuer.Bank.Correspondent.Account != "000123456789" {
		t.Fatalf("account changed: %q", p.Issuer.Bank.Correspondent.Account)
	}
}
