package invoice

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/azzz/invoicy/internal/config"
)

// Specification holds the invoice-specific source data.
type Specification struct {
	Profile   string        `yaml:"profile"`
	Number    string        `yaml:"number"`
	Date      string        `yaml:"date"`
	Signature string        `yaml:"signature,omitempty"`
	Works     []config.Work `yaml:"works"`
}

// SignatureIsImage distinguishes PNG paths from literal signature placeholders.
func (s Specification) SignatureIsImage() bool {
	return strings.EqualFold(filepath.Ext(s.Signature), ".png")
}

// Invoice combines a validated specification, profile, and calculated total.
type Invoice struct {
	Specification   Specification
	Profile         *config.Profile
	Total           int64
	Source          string
	ProfileSource   string
	SignatureSource string
}

var numberPattern = regexp.MustCompile(`^[A-Za-z0-9]+([._/-][A-Za-z0-9]+)*$`)

// ValidateNumber accepts filename-safe invoice identifiers up to 80 characters.
func ValidateNumber(number string) error {
	if len(number) > 80 || !numberPattern.MatchString(number) {
		return fmt.Errorf("number: expected 1–80 letters/digits separated by '.', '_', '/', or '-'")
	}
	return nil
}

// Filename derives a workspace filename from a validated invoice number.
func Filename(number, extension string) string {
	s := strings.ToLower(number)
	s = strings.TrimPrefix(s, "inv-")
	return "invoice-" + strings.ReplaceAll(s, "/", "--") + extension
}

// PDFFilename preserves the invoice number, replacing only path separators.
func PDFFilename(number string) string {
	return strings.ReplaceAll(number, "/", "--") + ".pdf"
}

// ResolveProfile confines profile paths, including symlinks, to the workspace.
func ResolveProfile(root, path string) (string, error) {
	return resolvePath(root, path, "profile")
}

func resolvePath(root, path, field string) (string, error) {
	if path == "" || filepath.IsAbs(path) || !filepath.IsLocal(path) {
		return "", fmt.Errorf("%s: expected a relative path inside the working directory", field)
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%s: %w", field, err)
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", fmt.Errorf("%s: %w", field, err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(base, path))
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", field, path, err)
	}
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%s: path resolves outside the working directory", field)
	}
	return resolved, nil
}

// Validate checks specification fields and calculates the total.
func (s Specification) Validate() (int64, error) {
	if strings.TrimSpace(s.Profile) == "" {
		return 0, fmt.Errorf("profile: must not be empty")
	}
	if err := ValidateNumber(s.Number); err != nil {
		return 0, err
	}
	if _, err := time.Parse("2006-01-02", s.Date); err != nil {
		return 0, fmt.Errorf("date: expected a valid date in YYYY-MM-DD format")
	}
	if err := config.Text("signature", s.Signature); err != nil {
		return 0, err
	}
	return config.Total(s.Works, "works")
}

// Load reads a specification and resolves its profile from root.
func Load(root, path string) (*Invoice, error) {
	input := path
	if !filepath.IsAbs(input) {
		input = filepath.Join(root, path)
	}
	var s Specification
	if err := config.ReadYAML(input, &s); err != nil {
		return nil, err
	}
	return Resolve(root, input, s)
}

// Resolve validates a specification and loads its profile.
func Resolve(root, source string, s Specification) (*Invoice, error) {
	total, err := s.Validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	path, err := ResolveProfile(root, s.Profile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	p, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("%s: profile: %w", source, err)
	}
	signaturePath := ""
	if s.SignatureIsImage() {
		signaturePath, err = resolvePath(root, s.Signature, "signature")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}
	}
	return &Invoice{SignatureSource: signaturePath, Specification: s, Profile: p, Total: total, Source: source, ProfileSource: path}, nil
}

// New copies profile defaults into a specification using the supplied local date.
func New(root, profile, number string, now time.Time) (Specification, error) {
	path, err := ResolveProfile(root, profile)
	if err != nil {
		return Specification{}, fmt.Errorf("%s: %w", profile, err)
	}
	p, err := config.Load(path)
	if err != nil {
		return Specification{}, err
	}
	if number == "" {
		number = "INV-" + now.Format("2006-01") + "/01"
	}
	s := Specification{Profile: profile, Number: number, Date: now.Format("2006-01-02"), Works: append([]config.Work(nil), p.DefaultWorks...), Signature: p.DefaultSignature}
	if _, err := s.Validate(); err != nil {
		return Specification{}, fmt.Errorf("%s: template: %w", profile, err)
	}
	return s, nil
}
