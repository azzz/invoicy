package config

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Profile holds the parties, payment details, and template defaults.
type Profile struct {
	Issuer           Issuer   `yaml:"issuer"`
	Customer         Customer `yaml:"customer"`
	Currency         string   `yaml:"currency"`
	DefaultSignature string   `yaml:"default_signature,omitempty"`
	DefaultWorks     []Work   `yaml:"default_works"`
}

// Issuer identifies the invoice sender and payment recipient.
type Issuer struct {
	Name    string   `yaml:"name"`
	Address []string `yaml:"address"`
	TaxID   string   `yaml:"tax_id,omitempty"`
	Bank    Bank     `yaml:"bank"`
}

// Customer identifies the billed party.
type Customer struct {
	Name      string   `yaml:"name"`
	Address   []string `yaml:"address"`
	VATNumber string   `yaml:"vat_number,omitempty"`
}

// Bank contains the recipient's bank account and routing details.
type Bank struct {
	Beneficiary   string             `yaml:"beneficiary"`
	IBAN          string             `yaml:"iban"`
	SWIFTBIC      string             `yaml:"swift_bic"`
	Name          string             `yaml:"name"`
	Address       []string           `yaml:"address"`
	Correspondent *CorrespondentBank `yaml:"correspondent_bank,omitempty"`
}

// CorrespondentBank identifies an optional intermediary bank.
type CorrespondentBank struct {
	Account  string `yaml:"account,omitempty"`
	Name     string `yaml:"name"`
	SWIFTBIC string `yaml:"swift_bic"`
}

// Load reads and validates a profile.
func Load(path string) (*Profile, error) {
	var p Profile
	if err := ReadYAML(path, &p); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &p, nil
}

// Required validates a nonempty printable text field.
func Required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: must not be empty", field)
	}
	return Text(field, value)
}

// Text rejects control characters in printable fields.
func Text(field, value string) error {
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' {
			return fmt.Errorf("%s: contains a control character", field)
		}
	}
	return nil
}

func address(field string, lines []string) error {
	if len(lines) == 0 {
		return fmt.Errorf("%s: must contain at least one line", field)
	}
	for i, line := range lines {
		if err := Required(fmt.Sprintf("%s[%d]", field, i), line); err != nil {
			return err
		}
	}
	return nil
}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var bicPattern = regexp.MustCompile(`^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
var ibanPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]+$`)

// Validate checks required profile fields, bank identifiers, and default work.
func (p *Profile) Validate() error {
	for _, f := range []struct{ name, value string }{
		{"issuer.name", p.Issuer.Name}, {"customer.name", p.Customer.Name},
		{"issuer.bank.beneficiary", p.Issuer.Bank.Beneficiary}, {"issuer.bank.name", p.Issuer.Bank.Name},
	} {
		if err := Required(f.name, f.value); err != nil {
			return err
		}
	}
	for _, f := range []struct {
		name  string
		value []string
	}{
		{"issuer.address", p.Issuer.Address}, {"customer.address", p.Customer.Address}, {"issuer.bank.address", p.Issuer.Bank.Address},
	} {
		if err := address(f.name, f.value); err != nil {
			return err
		}
	}
	if err := Text("issuer.tax_id", p.Issuer.TaxID); err != nil {
		return err
	}
	if err := Text("customer.vat_number", p.Customer.VATNumber); err != nil {
		return err
	}
	if !currencyPattern.MatchString(p.Currency) {
		return fmt.Errorf("currency: expected a three-letter uppercase currency code")
	}
	if err := validateIBAN(p.Issuer.Bank.IBAN); err != nil {
		return fmt.Errorf("issuer.bank.iban: %w", err)
	}
	if !bicPattern.MatchString(p.Issuer.Bank.SWIFTBIC) {
		return fmt.Errorf("issuer.bank.swift_bic: expected an 8 or 11 character SWIFT/BIC")
	}
	if c := p.Issuer.Bank.Correspondent; c != nil {
		if err := Required("issuer.bank.correspondent_bank.name", c.Name); err != nil {
			return err
		}
		if err := Text("issuer.bank.correspondent_bank.account", c.Account); err != nil {
			return err
		}
		if !bicPattern.MatchString(c.SWIFTBIC) {
			return fmt.Errorf("issuer.bank.correspondent_bank.swift_bic: expected an 8 or 11 character SWIFT/BIC")
		}
	}
	if err := Text("default_signature", p.DefaultSignature); err != nil {
		return err
	}
	_, err := Total(p.DefaultWorks, "default_works")
	return err
}

func validateIBAN(value string) error {
	s := strings.ReplaceAll(value, " ", "")
	if len(s) < 15 || len(s) > 34 || !ibanPattern.MatchString(s) {
		return fmt.Errorf("invalid IBAN format")
	}
	// Check country-specific lengths before the ISO 13616 mod-97 checksum.
	lengths := map[string]int{
		"AD": 24, "AE": 23, "AL": 28, "AT": 20, "AZ": 28, "BA": 20, "BE": 16, "BG": 22, "BH": 22, "BI": 27, "BR": 29, "BY": 28,
		"CH": 21, "CR": 22, "CY": 28, "CZ": 24, "DE": 22, "DJ": 27, "DK": 18, "DO": 28, "EE": 20, "EG": 29, "ES": 24, "FI": 18,
		"FK": 18, "FO": 18, "FR": 27, "GB": 22, "GE": 22, "GI": 23, "GL": 18, "GR": 27, "GT": 28, "HN": 28, "HR": 21, "HU": 28,
		"IE": 22, "IL": 23, "IQ": 23, "IS": 26, "IT": 27, "JO": 30, "KW": 30, "KZ": 20, "LB": 28, "LC": 32, "LI": 21, "LT": 20,
		"LU": 20, "LV": 21, "LY": 25, "MC": 27, "MD": 24, "ME": 22, "MK": 19, "MN": 20, "MR": 27, "MT": 31, "MU": 30, "NI": 28,
		"NL": 18, "NO": 15, "OM": 23, "PK": 24, "PL": 28, "PS": 29, "PT": 25, "QA": 29, "RO": 24, "RS": 22, "RU": 33, "SA": 24,
		"SC": 31, "SD": 18, "SE": 24, "SI": 19, "SK": 24, "SM": 27, "SO": 23, "ST": 25, "SV": 28, "TL": 23, "TN": 24, "TR": 26,
		"UA": 29, "VA": 22, "VG": 24, "XK": 20,
	}
	if lengths[s[:2]] != len(s) {
		return fmt.Errorf("invalid IBAN country or length")
	}
	rem := 0
	for _, c := range s[4:] + s[:4] {
		if c >= 'A' && c <= 'Z' {
			rem = (rem*100 + int(c-'A') + 10) % 97
		} else {
			rem = (rem*10 + int(c-'0')) % 97
		}
	}
	if rem != 1 {
		return fmt.Errorf("invalid IBAN checksum")
	}
	return nil
}
