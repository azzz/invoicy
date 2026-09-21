package config

import (
	"math"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestMoney(t *testing.T) {
	for _, tc := range []struct {
		input   string
		want    int64
		display string
	}{
		{"0.01", 1, "0.01"}, {"1", 100, "1.00"}, {"1.2", 120, "1.20"},
		{"0001.20", 120, "1.20"}, {"92233720368547758.07", math.MaxInt64, "92233720368547758.07"},
	} {
		n, err := Amount(tc.input).MinorUnits()
		if err != nil || n != tc.want || FormatMoney(n) != tc.display {
			t.Fatalf("%q: %d, %v", tc.input, n, err)
		}
	}
	for _, input := range []string{"", "0", "0.00", "-1", "+1", "1.001", "1.", ".1", "1e2", "1,00", " 1.00", "NaN", "92233720368547758.08"} {
		if _, err := Amount(input).MinorUnits(); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	total, err := Total([]Work{{"a", "0.10"}, {"b", "0.20"}}, "works")
	if err != nil || FormatMoney(total) != "0.30" {
		t.Fatalf("%d, %v", total, err)
	}
	if _, err := Total([]Work{{"a", "92233720368547758.07"}, {"b", "0.01"}}, "works"); err == nil {
		t.Fatal("accepted overflowing sum")
	}
	b, err := yaml.Marshal(Work{"test", "0001.20"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "description: test\namount: \"0001.20\"\n" {
		t.Fatalf("amount text changed: %s", b)
	}
}
