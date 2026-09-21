package invoice

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/azzz/invoicy/internal/config"
)

func workspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"profiles/example-2026.yml", "invoices/invoice-2026-09--01.yml"} {
		data, err := os.ReadFile(filepath.Join("../../examples", path))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInit(t *testing.T) {
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, "invoices/invoice.yml"); err != nil {
		t.Fatalf("initialized example is invalid: %v", err)
	}
	for _, name := range []string{"profiles/profiles.yml", "invoices/invoice.yml", "README.md"} {
		path := filepath.Join(root, name)
		if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
			t.Fatalf("missing template %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte("user content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"profiles/profiles.yml", "invoices/invoice.yml", "README.md"} {
		if data, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(data) != "user content" {
			t.Fatalf("init replaced %s: %q %v", name, data, err)
		}
	}

	for _, name := range []string{"profiles", "invoices", "output"} {
		path := filepath.Join(root, name, "keep")
		if err := os.WriteFile(path, []byte("untouched"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"profiles", "invoices", "output"} {
		data, err := os.ReadFile(filepath.Join(root, name, "keep"))
		if err != nil || string(data) != "untouched" {
			t.Fatalf("%s changed: %q %v", name, data, err)
		}
		t.Run(name+" is a file", func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, name), []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := Init(root); err == nil {
				t.Fatal("accepted a file instead of directory")
			}
		})
	}
}

func TestLoadAndResolution(t *testing.T) {
	root := workspace(t)
	path := filepath.Join(root, "invoices/invoice-2026-09--01.yml")
	inv, err := Load(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Total != 271840 || len(inv.Specification.Works) != 3 {
		t.Fatalf("unexpected invoice: %+v", inv)
	}
	other := t.TempDir()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(other, "moved.yml")
	if err := os.WriteFile(moved, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, moved); err != nil {
		t.Fatalf("moving specification affected resolution: %v", err)
	}
	if _, err := Load(other, moved); err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("wrong working directory accepted: %v", err)
	}
	nested := filepath.Join(root, "profiles/customers")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "profiles/example-2026.yml"), filepath.Join(nested, "nested.yml")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveProfile(root, "profiles/customers/nested.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nested, filepath.Join(root, "inside")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveProfile(root, "inside/nested.yml"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", filepath.Join(nested, "nested.yml"), "../escape.yml", "profiles/../../escape.yml", "outside/moved.yml", "profiles/missing.yml"} {
		if _, err := ResolveProfile(root, bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestTemplate(t *testing.T) {
	root := workspace(t)
	profile := "profiles/example-2026.yml"
	now := time.Date(2027, 1, 1, 0, 30, 0, 0, time.FixedZone("local", 14*60*60))
	s, err := New(root, profile, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Number != "INV-2027-01/01" || s.Date != "2027-01-01" || s.Profile != profile {
		t.Fatalf("bad defaults: %+v", s)
	}
	p, err := config.Load(filepath.Join(root, profile))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Works, p.DefaultWorks) {
		t.Fatal("defaults not copied exactly")
	}
	s.Works[0].Amount = "0.10"
	path := filepath.Join(root, "invoices/new.yml")
	if err := WriteSpecification(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Total != 10 || loaded.Profile.DefaultWorks[0].Amount != "2293.00" {
		t.Fatal("works and defaults were merged or mutated")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Works[0].Amount = "12.34"
	if err := WriteSpecification(path, s); err == nil {
		t.Fatal("overwrote existing specification")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("existing specification changed")
	}
	if err := WriteSpecification(filepath.Join(root, "missing/new.yml"), s); err == nil {
		t.Fatal("created missing parent")
	}
	s.Works = nil
	invalid := filepath.Join(root, "invoices/invalid.yml")
	if err := WriteSpecification(invalid, s); err == nil {
		t.Fatal("wrote invalid specification")
	}
	if _, err := os.Stat(invalid); !os.IsNotExist(err) {
		t.Fatalf("invalid output exists: %v", err)
	}
	entries, err := filepath.Glob(filepath.Join(root, "invoices/.invoicy-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files left: %v %v", entries, err)
	}
	custom, err := New(root, profile, "INV-2027-01/02", now)
	if err != nil || custom.Number != "INV-2027-01/02" {
		t.Fatalf("override: %+v %v", custom, err)
	}
}

func TestSpecificationValidation(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		change      func(*Specification)
	}{
		{"number missing", "number", func(s *Specification) { s.Number = "" }},
		{"number path", "number", func(s *Specification) { s.Number = "../escape" }},
		{"date missing", "date", func(s *Specification) { s.Date = "" }},
		{"date invalid", "date", func(s *Specification) { s.Date = "2026-02-29" }},
		{"date malformed", "date", func(s *Specification) { s.Date = "21/09/2026" }},
		{"profile missing", "profile", func(s *Specification) { s.Profile = "" }},
		{"works empty", "works", func(s *Specification) { s.Works = nil }},
		{"description empty", "works[0].description", func(s *Specification) { s.Works[0].Description = "" }},
		{"amount invalid", "works[0].amount", func(s *Specification) { s.Works[0].Amount = "-1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Specification{"profiles/example.yml", "INV-2026-09/01", "2026-09-21", "", []config.Work{{Description: "Development", Amount: "1.20"}}}
			tc.change(&s)
			if _, err := s.Validate(); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("got %v, want %s", err, tc.field)
			}
		})
	}
}

func TestSpecificationParsing(t *testing.T) {
	root := workspace(t)
	data, err := os.ReadFile(filepath.Join(root, "invoices/invoice-2026-09--01.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, data, field string }{
		{"numeric amount", strings.Replace(string(data), `"2293.00"`, "2293.00", 1), "works[0].amount"},
		{"total", string(data) + "total: 2718.40\n", "field total"},
		{"bad YAML", "works: [", "YAML"},
		{"missing profile file", strings.Replace(string(data), "example-2026.yml", "missing.yml", 1), "profile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, "invoices/invalid.yml")
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root, path)
			if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("got %v, want path and %s", err, tc.field)
			}
		})
	}
}

func TestNumberAndFilename(t *testing.T) {
	for _, number := range []string{"INV-2026-09/01", "ACME_42", "2026.09", "1"} {
		if err := ValidateNumber(number); err != nil {
			t.Fatal(err)
		}
	}
	for _, number := range []string{"", "../x", "A//B", "A\\B", "/abc", "abc/", "A B", "A\nB", strings.Repeat("a", 81)} {
		if err := ValidateNumber(number); err == nil {
			t.Errorf("accepted %q", number)
		}
	}
	if got := Filename("INV-2026-09/01", ".pdf"); got != "invoice-2026-09--01.pdf" {
		t.Fatal(got)
	}
}

func TestSignatureDefaultsAndResolution(t *testing.T) {
	root := workspace(t)
	profile := "profiles/example-2026.yml"
	path := filepath.Join(root, profile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`default_signature: ""`), []byte(`default_signature: "profiles/signature.png"`), 1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	signaturePath := filepath.Join(root, "profiles/signature.png")
	if err := os.WriteFile(signaturePath, []byte("image validated by renderer"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(root, profile, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if s.Signature != "profiles/signature.png" {
		t.Fatalf("signature not copied: %+v", s)
	}
	specPath := filepath.Join(t.TempDir(), "moved.yml")
	if err := WriteSpecification(specPath, s); err != nil {
		t.Fatal(err)
	}
	inv, err := Load(root, specPath)
	if err != nil || inv.SignatureSource != signaturePath {
		t.Fatalf("signature resolution: %+v %v", inv, err)
	}
	// A missing profile default must not affect an invoice with its own signature.
	data = bytes.Replace(data, []byte("profiles/signature.png"), []byte("profiles/missing.png"), 1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, specPath); err != nil {
		t.Fatal(err)
	}
	s.Signature = ""
	inv, err = Resolve(root, "unsigned.yml", s)
	if err != nil || inv.SignatureSource != "" {
		t.Fatalf("default merged into invoice: %+v %v", inv, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "profiles/escape.png")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"profiles/missing.png", "../outside.png", outside, "profiles/escape.png"} {
		s.Signature = bad
		if _, err := Resolve(root, "invoice.yml", s); err == nil || !strings.Contains(err.Error(), "signature") {
			t.Fatalf("accepted %q: %v", bad, err)
		}
	}
}

func TestSignaturePlaceholder(t *testing.T) {
	root := workspace(t)
	path := filepath.Join(root, "profiles/example-2026.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const placeholder = "________________ / Name"
	data = bytes.Replace(data, []byte(`default_signature: ""`), []byte(`default_signature: "`+placeholder+`"`), 1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(root, "profiles/example-2026.yml", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if s.Signature != placeholder {
		t.Fatalf("placeholder not copied: %q", s.Signature)
	}
	path = filepath.Join(root, "invoices/manual.yml")
	if err := WriteSpecification(path, s); err != nil {
		t.Fatal(err)
	}
	inv, err := Load(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Specification.Signature != placeholder || inv.SignatureSource != "" {
		t.Fatalf("unexpected signature: %+v", inv)
	}
	for _, value := range []string{"profiles/signature.png", "profiles/signature.PNG"} {
		s.Signature = value
		if !s.SignatureIsImage() {
			t.Fatalf("PNG treated as text: %q", value)
		}
	}
}

func TestPDFFilename(t *testing.T) {
	for number, want := range map[string]string{
		"INV-2026-09/01": "INV-2026-09--01.pdf",
		"Client-AbC/02":  "Client-AbC--02.pdf",
		"2026.09_03":     "2026.09_03.pdf",
	} {
		if got := PDFFilename(number); got != want {
			t.Errorf("%q: got %q, want %q", number, got, want)
		}
	}
}
