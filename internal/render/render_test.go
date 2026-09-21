package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/azzz/invoicy/internal/config"
	"github.com/azzz/invoicy/internal/invoice"
)

func example(t *testing.T) *invoice.Invoice {
	t.Helper()
	inv, err := invoice.Load("../../examples", "invoices/invoice-2026-09--01.yml")
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

func TestPDF(t *testing.T) {
	inv := example(t)
	data, err := PDF(inv)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("invalid PDF envelope")
	}
	if !bytes.Contains(data, []byte("/Count 1")) || !bytes.Contains(data, []byte("/MediaBox [0 0 595.28 841.89]")) {
		t.Fatal("expected one A4 page")
	}
	for _, tool := range []string{"pdfinfo", "pdftotext"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("Poppler tools not installed; basic PDF checks passed")
		}
	}
	path := filepath.Join(t.TempDir(), "invoice.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := exec.Command("pdfinfo", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pdfinfo: %s: %v", info, err)
	}
	if !regexp.MustCompile(`Pages:\s+1\b`).Match(info) || !bytes.Contains(info, []byte("(A4)")) {
		t.Fatalf("unexpected page: %s", info)
	}
	text, err := exec.Command("pdftotext", "-layout", path, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext: %s: %v", text, err)
	}
	for _, want := range []string{
		"INV-2026-09/01", "Example Issuer Ltd", "10 Example Street", "Berlin, 10115", "Germany", "EXAMPLE-TAX-ID",
		"Beneficiary:", "DE89370400440532013000", "COBADEFFXXX", "Example Bank", "Bank address:", "20 Bank Street", "Berlin, 10117",
		"Example Correspondent Bank", "Correspondent account:", "000123456789", "DEUTDEFF", "BILL TO", "Example Customer Ltd", "30 Customer Street", "Dublin, D02 EX00", "Ireland", "EXAMPLE-VAT-NUMBER",
		"Software development and consulting", "Travel expenses reimbursement", "Additional development work",
		"2293.00", "125.40", "300.00", "EUR 2718.40", "2026-09-21",
	} {
		if !bytes.Contains(text, []byte(want)) {
			t.Errorf("missing %q in PDF text:\n%s", want, text)
		}
	}
	if bytes.Contains(text, []byte("Signature:")) {
		t.Fatal("unsigned invoice has a signature block")
	}
	second, err := PDF(inv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, second, 0600); err != nil {
		t.Fatal(err)
	}
	secondText, err := exec.Command("pdftotext", "-layout", path, "-").CombinedOutput()
	if err != nil || !bytes.Equal(text, secondText) {
		t.Fatalf("rerender changed invoice content: %v", err)
	}
}

func TestLayoutValidation(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		change      func(*invoice.Invoice)
	}{
		{"too many rows", "works", func(inv *invoice.Invoice) {
			inv.Specification.Works = make([]config.Work, 100)
			for i := range inv.Specification.Works {
				inv.Specification.Works[i] = config.Work{Description: "Development", Amount: "1.00"}
			}
		}},
		{"long description", "works[0].description", func(inv *invoice.Invoice) {
			inv.Specification.Works[0].Description = strings.Repeat("Long description ", 1000)
		}},
		{"long address", "issuer.address", func(inv *invoice.Invoice) {
			inv.Profile.Issuer.Address = []string{strings.Repeat("Long address ", 1000)}
		}},
		{"long bank address", "issuer.bank.address", func(inv *invoice.Invoice) {
			inv.Profile.Issuer.Bank.Address = []string{strings.Repeat("Long bank address ", 1000)}
		}},
		{"long customer address", "customer.address", func(inv *invoice.Invoice) {
			inv.Profile.Customer.Address = []string{strings.Repeat("Long customer address ", 1000)}
		}},
		{"unsupported glyph", "customer.name", func(inv *invoice.Invoice) { inv.Profile.Customer.Name = "Customer 😀" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := example(t)
			tc.change(inv)
			err := Check(inv)
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("got %v, want %s", err, tc.field)
			}
			data, renderErr := PDF(inv)
			if renderErr == nil || renderErr.Error() != err.Error() || data != nil {
				t.Fatalf("render/check mismatch: %v / %v", err, renderErr)
			}
		})
	}
}

func TestUnicodeAndWrapping(t *testing.T) {
	inv := example(t)
	inv.Profile.Customer.Name = "Замовник — Müller"
	inv.Specification.Works[0].Description = strings.Repeat("Розробка програмного забезпечення. ", 4)
	inv.Profile.Customer.VATNumber = ""
	inv.Profile.Issuer.Bank.Correspondent = nil
	if _, err := PDF(inv); err != nil {
		t.Fatal(err)
	}
}

func TestSignatureImage(t *testing.T) {
	inv := example(t)
	inv.Specification.Signature = "signature.png"
	inv.SignatureSource = filepath.Join(t.TempDir(), "signature.png")
	img := image.NewNRGBA(image.Rect(0, 0, 100, 40))
	for x := 10; x < 90; x++ {
		img.Set(x, 20, color.NRGBA{A: 255})
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inv.SignatureSource, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Check(inv); err != nil {
		t.Fatal(err)
	}
	data, err := PDF(inv)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("/Subtype /Image")) || !bytes.Contains(data, []byte("/Count 1")) {
		t.Fatal("missing image or invalid page count")
	}
	p, err := build(inv)
	if err != nil {
		t.Fatal(err)
	}
	l := &layout{pdf: p, y: bottom - 1}
	l.signature(inv)
	if l.err == nil || !strings.Contains(l.err.Error(), "signature") {
		t.Fatalf("signature overflow accepted: %v", l.err)
	}
	if err := os.WriteFile(inv.SignatureSource, []byte("not a PNG"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Check(inv); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("invalid image accepted: %v", err)
	}
	if err := os.Remove(inv.SignatureSource); err != nil {
		t.Fatal(err)
	}
	if _, err := PDF(inv); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("missing image accepted: %v", err)
	}
}

func TestSignaturePlaceholder(t *testing.T) {
	inv := example(t)
	inv.Specification.Signature = "________________ / Name"
	if err := Check(inv); err != nil {
		t.Fatal(err)
	}
	data, err := PDF(inv)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("/Subtype /Image")) {
		t.Fatal("placeholder rendered as image")
	}
	for _, value := range []string{strings.Repeat("placeholder ", 1000), "Unsupported 😀"} {
		inv.Specification.Signature = value
		if err := Check(inv); err == nil || !strings.Contains(err.Error(), "signature") {
			t.Fatalf("invalid placeholder accepted: %v", err)
		}
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("Poppler not installed")
	}
	path := filepath.Join(t.TempDir(), "manual.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	text, err := exec.Command("pdftotext", "-layout", path, "-").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(text, []byte("Signature: ________________ / Name")) {
		t.Fatalf("missing placeholder: %s", text)
	}
}
