package render

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/azzz/invoicy/internal/config"
	"github.com/azzz/invoicy/internal/invoice"
	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

const (
	margin = 16.0
	width  = 178.0
	bottom = 281.0
)

type layout struct {
	pdf  *fpdf.Fpdf
	font *sfnt.Font
	y    float64
	err  error
}

// Check verifies text coverage and single-page layout without writing a PDF.
func Check(inv *invoice.Invoice) error {
	_, err := build(inv)
	return err
}

// PDF renders a validated invoice into a single A4 page.
func PDF(inv *invoice.Invoice) ([]byte, error) {
	p, err := build(inv)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := p.Output(&out); err != nil {
		return nil, fmt.Errorf("%s: PDF: %w", inv.Source, err)
	}
	return out.Bytes(), nil
}

func build(inv *invoice.Invoice) (*fpdf.Fpdf, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(margin, margin, margin)
	p.SetAutoPageBreak(false, margin)
	p.SetCatalogSort(true)
	date, _ := time.Parse("2006-01-02", inv.Specification.Date)
	p.SetCreationDate(date)
	p.SetModificationDate(date)
	p.SetTitle("Invoice "+inv.Specification.Number, true)
	p.SetAuthor(inv.Profile.Issuer.Name, true)
	p.SetCreator("Invoicy", true)
	p.AddUTF8FontFromBytes("Go", "", goregular.TTF)
	p.AddUTF8FontFromBytes("Go", "B", gobold.TTF)
	p.SetDrawColor(160, 168, 178)
	p.SetTextColor(30, 38, 48)
	p.SetLineWidth(0.2)
	p.AddPage()
	font, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	l := &layout{pdf: p, font: font, y: margin}
	s, profile := inv.Specification, inv.Profile
	text := func(source, field, value string, bold bool, size, lineHeight float64) {
		l.text(source, field, value, bold, size, lineHeight)
	}
	section := func(title string) { l.y += 3; text(inv.Source, "layout", title, true, 10, 6) }
	text(inv.Source, "number", "INVOICE", true, 22, 11)
	text(inv.Source, "number", s.Number, false, 12, 7)
	l.y += 3
	text(inv.ProfileSource, "issuer.name", profile.Issuer.Name, true, 12, 6)
	for i, line := range profile.Issuer.Address {
		text(inv.ProfileSource, fmt.Sprintf("issuer.address[%d]", i), line, false, 9, 4.5)
	}
	if profile.Issuer.TaxID != "" {
		text(inv.ProfileSource, "issuer.tax_id", "Tax ID: "+profile.Issuer.TaxID, false, 9, 4.5)
	}
	section("PAYMENT DETAILS")
	b := profile.Issuer.Bank
	for _, f := range []struct{ field, label, value string }{
		{"beneficiary", "Beneficiary", b.Beneficiary}, {"iban", "IBAN", b.IBAN},
		{"swift_bic", "SWIFT/BIC", b.SWIFTBIC}, {"name", "Bank name", b.Name},
	} {
		l.detail(inv.ProfileSource, "issuer.bank."+f.field, f.label+":", f.value)
	}
	l.detail(inv.ProfileSource, "issuer.bank.address", "Bank address:", strings.Join(b.Address, ", "))
	if b.Correspondent != nil {
		l.detail(inv.ProfileSource, "issuer.bank.correspondent_bank.name", "Correspondent bank:", b.Correspondent.Name)
		if b.Correspondent.Account != "" {
			l.detail(inv.ProfileSource, "issuer.bank.correspondent_bank.account", "Correspondent account:", b.Correspondent.Account)
		}
		l.detail(inv.ProfileSource, "issuer.bank.correspondent_bank.swift_bic", "Correspondent SWIFT/BIC:", b.Correspondent.SWIFTBIC)
	}
	section("BILL TO")
	l.detail(inv.ProfileSource, "customer.name", "Customer:", profile.Customer.Name)
	l.detail(inv.ProfileSource, "customer.address", "Address:", strings.Join(profile.Customer.Address, "\n"))
	if profile.Customer.VATNumber != "" {
		l.detail(inv.ProfileSource, "customer.vat_number", "VAT number:", profile.Customer.VATNumber)
	}

	l.y += 6
	l.row(inv.Source, "works", "Description", "Amount ("+profile.Currency+")", true)
	for i, w := range s.Works {
		n, _ := w.Amount.MinorUnits()
		l.row(inv.Source, fmt.Sprintf("works[%d].description", i), w.Description, config.FormatMoney(n), false)
	}
	l.row(inv.Source, "works.total", "TOTAL", profile.Currency+" "+config.FormatMoney(inv.Total), true)
	l.y += 8
	text(inv.Source, "date", "Invoice date: "+s.Date, false, 10, 6)
	if s.Signature != "" {
		l.signature(inv)
	}
	if l.err != nil {
		return nil, l.err
	}
	if err := p.Error(); err != nil {
		return nil, fmt.Errorf("%s: PDF: %w", inv.Source, err)
	}
	return p, nil
}

func (l *layout) lines(source, field, value string, bold bool, size, w float64) []string {
	if l.err != nil {
		return nil
	}
	for _, r := range value {
		if r == '\n' {
			continue
		}
		idx, err := l.font.GlyphIndex(nil, r)
		if err != nil || idx == 0 || r > 0xffff {
			l.err = fmt.Errorf("%s: %s: character %U is not supported by the embedded font", source, field, r)
			return nil
		}
	}
	style := ""
	if bold {
		style = "B"
	}
	l.pdf.SetFont("Go", style, size)
	return l.pdf.SplitText(value, w)
}

func (l *layout) fits(source, field string, height float64) bool {
	if l.err != nil {
		return false
	}
	if l.y+height > bottom {
		l.err = fmt.Errorf("%s: %s: content does not fit on one A4 page", source, field)
		return false
	}
	return true
}

func (l *layout) text(source, field, value string, bold bool, size, lineHeight float64) {
	lines := l.lines(source, field, value, bold, size, width)
	if !l.fits(source, field, float64(len(lines))*lineHeight) {
		return
	}
	for _, line := range lines {
		l.pdf.SetXY(margin, l.y)
		l.pdf.CellFormat(width, lineHeight, line, "", 0, "L", false, 0, "")
		l.y += lineHeight
	}
}

func (l *layout) row(source, field, description, amount string, bold bool) {
	const leftWidth, rightWidth, lineHeight = 122.0, 56.0, 5.0
	lines := l.lines(source, field, description, bold, 10, leftWidth-4)
	right := l.lines(source, field, amount, bold, 10, rightWidth-4)
	height := float64(max(len(lines), len(right)))*lineHeight + 4
	if !l.fits(source, field, height) {
		return
	}
	if bold {
		l.pdf.SetFillColor(242, 244, 247)
		l.pdf.Rect(margin, l.y, width, height, "F")
	}
	l.pdf.Rect(margin, l.y, leftWidth, height, "D")
	l.pdf.Rect(margin+leftWidth, l.y, rightWidth, height, "D")
	for i, line := range lines {
		l.pdf.SetXY(margin+2, l.y+2+float64(i)*lineHeight)
		l.pdf.CellFormat(leftWidth-4, lineHeight, line, "", 0, "L", false, 0, "")
	}
	for i, line := range right {
		l.pdf.SetXY(margin+leftWidth+2, l.y+2+float64(i)*lineHeight)
		l.pdf.CellFormat(rightWidth-4, lineHeight, line, "", 0, "R", false, 0, "")
	}
	l.y += height
}

// detail aligns labels and values, allowing either column to wrap.
func (l *layout) detail(source, field, label, value string) {
	const labelWidth, padding, lineHeight = 49.0, 2.0, 4.5
	labels := l.lines(source, field, label, false, 8.5, labelWidth-2*padding)
	values := l.lines(source, field, value, false, 9, width-labelWidth-2*padding)
	height := float64(max(len(labels), len(values)))*lineHeight + 2*padding
	if !l.fits(source, field, height) {
		return
	}
	p := l.pdf
	p.SetFillColor(242, 244, 247)
	p.Rect(margin, l.y, labelWidth, height, "F")
	p.SetDrawColor(220, 225, 231)
	p.Line(margin, l.y+height, margin+width, l.y+height)
	p.SetFont("Go", "", 8.5)
	p.SetTextColor(85, 96, 110)
	for i, line := range labels {
		p.SetXY(margin+padding, l.y+padding+float64(i)*lineHeight)
		p.CellFormat(labelWidth-2*padding, lineHeight, line, "", 0, "L", false, 0, "")
	}
	p.SetFont("Go", "", 9)
	p.SetTextColor(30, 38, 48)
	for i, line := range values {
		p.SetXY(margin+labelWidth+padding, l.y+padding+float64(i)*lineHeight)
		p.CellFormat(width-labelWidth-2*padding, lineHeight, line, "", 0, "L", false, 0, "")
	}
	p.SetDrawColor(160, 168, 178)
	l.y += height
}

// signature prints a literal placeholder or embeds a PNG inside a 50 by 20 mm box.
func (l *layout) signature(inv *invoice.Invoice) {
	if l.err != nil {
		return
	}
	if !inv.Specification.SignatureIsImage() {
		l.y += 7
		l.text(inv.Source, "signature", "Signature: "+inv.Specification.Signature, false, 9, 5)
		return
	}
	data, err := os.ReadFile(inv.SignatureSource)
	if err != nil {
		l.err = fmt.Errorf("%s: signature %q: %w", inv.Source, inv.Specification.Signature, err)
		return
	}
	options := fpdf.ImageOptions{ImageType: "PNG"}
	info := l.pdf.RegisterImageOptionsReader("signature", options, bytes.NewReader(data))
	if err := l.pdf.Error(); err != nil {
		l.err = fmt.Errorf("%s: signature %q: expected a valid PNG: %w", inv.Source, inv.Specification.Signature, err)
		return
	}
	w, h := 50.0, 50.0*info.Height()/info.Width()
	if h > 20 {
		w, h = w*20/h, 20
	}
	l.y += 7
	if !l.fits(inv.Source, "signature", 5+h) {
		return
	}
	l.text(inv.Source, "signature", "Signature:", false, 9, 5)
	l.pdf.ImageOptions("signature", margin, l.y, w, h, false, options, 0, "")
	l.y += h
}
