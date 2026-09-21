package config

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Amount preserves the decimal text supplied in YAML.
type Amount string

var decimalPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)

// MinorUnits parses a positive amount into integer hundredths.
func (a Amount) MinorUnits() (int64, error) {
	s := string(a)
	if !decimalPattern.MatchString(s) {
		return 0, fmt.Errorf("expected a positive decimal with at most two fractional digits")
	}
	whole, fraction, _ := strings.Cut(s, ".")
	fraction += strings.Repeat("0", 2-len(fraction))
	n, err := strconv.ParseInt(strings.TrimLeft(whole+fraction, "0"), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("amount must be positive and fit in signed 64-bit minor units")
	}
	return n, nil
}

// MarshalYAML always writes amounts as quoted strings.
func (a Amount) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: string(a), Style: yaml.DoubleQuotedStyle}, nil
}

// FormatMoney formats minor units with exactly two fractional digits.
func FormatMoney(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }

// Work is a description and its fixed price.
type Work struct {
	Description string `yaml:"description"`
	Amount      Amount `yaml:"amount"`
}

// Total validates work items and sums their amounts without rounding.
func Total(works []Work, field string) (int64, error) {
	if len(works) == 0 {
		return 0, fmt.Errorf("%s: must contain at least one work item", field)
	}
	var total int64
	for i, w := range works {
		if err := Required(fmt.Sprintf("%s[%d].description", field, i), w.Description); err != nil {
			return 0, err
		}
		n, err := w.Amount.MinorUnits()
		if err != nil {
			return 0, fmt.Errorf("%s[%d].amount: %w", field, i, err)
		}
		if total > math.MaxInt64-n {
			return 0, fmt.Errorf("%s[%d].amount: total exceeds signed 64-bit minor units", field, i)
		}
		total += n
	}
	return total, nil
}
