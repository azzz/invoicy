package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/azzz/invoicy/internal/invoice"
)

var fixedDate = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

func cliWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := run([]string{"init"}, root, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../examples/profiles/example-2026.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "profiles/example.yml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCommands(t *testing.T) {
	root := cliWorkspace(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"new", "profiles/example.yml"}, "invoices/invoice-2026-09--01.yml"},
		{[]string{"new", "profiles/example.yml", "--number", "INV-2026-09/02"}, "invoices/invoice-2026-09--02.yml"},
		{[]string{"new", "--number=INV-2026-09/03", "profiles/example.yml", "--output", "invoices/custom.yml"}, "invoices/custom.yml"},
		{[]string{"validate", "invoices/custom.yml"}, "valid (EUR 2293.00)"},
		{[]string{"render", "invoices/invoice-2026-09--01.yml"}, "output/INV-2026-09--01.pdf"},
		{[]string{"render", "invoices/custom.yml", "--output=output/custom.pdf"}, "output/custom.pdf"},
	} {
		var out bytes.Buffer
		if err := run(tc.args, root, fixedDate, &out); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v: got %q, want %q", tc.args, out.String(), tc.want)
		}
		if tc.args[0] != "validate" {
			if _, err := os.Stat(filepath.Join(root, strings.TrimSpace(out.String()))); err != nil {
				t.Fatal(err)
			}
		}
	}
	inv, err := invoice.Load(root, "invoices/custom.yml")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Specification.Number != "INV-2026-09/03" || inv.Specification.Date != "2026-09-21" {
		t.Fatalf("unexpected template: %+v", inv.Specification)
	}
	if err := run([]string{"new", "profiles/example.yml"}, root, fixedDate, &bytes.Buffer{}); err == nil {
		t.Fatal("replaced an existing template")
	}
	if err := run([]string{"render", "invoices/custom.yml", "--output=output/custom.pdf"}, root, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatalf("rerender failed: %v", err)
	}
}

func TestValidationBeforeWriting(t *testing.T) {
	root := cliWorkspace(t)
	profile := filepath.Join(root, "profiles/example.yml")
	original, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{"", strings.Repeat("Long description ", 1000)} {
		data := strings.Replace(string(original), "Software development and consulting", replacement, 1)
		if err := os.WriteFile(profile, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"new", "profiles/example.yml", "--output=invoices/invalid.yml"}, root, fixedDate, &bytes.Buffer{}); err == nil {
			t.Fatal("invalid profile generated a specification")
		}
		if _, err := os.Stat(filepath.Join(root, "invoices/invalid.yml")); !os.IsNotExist(err) {
			t.Fatalf("output created: %v", err)
		}
	}
	if err := os.WriteFile(profile, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"new", "profiles/example.yml"}, root, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(root, "invoices/invoice-2026-09--01.yml")
	valid, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "output/keep.pdf")
	if err := os.WriteFile(output, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(string(valid), `"2293.00"`, `"-1.00"`, 1),
		strings.Replace(string(valid), "Software development and consulting", strings.Repeat("Long description ", 1000), 1),
	} {
		if err := os.WriteFile(spec, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"validate", spec}, {"render", spec, "--output", output}, {"render", spec, "--output", "output/absent.pdf"},
		} {
			if err := run(args, root, fixedDate, &bytes.Buffer{}); err == nil {
				t.Fatalf("accepted invalid invoice: %v", args)
			}
		}
		got, err := os.ReadFile(output)
		if err != nil || string(got) != "keep" {
			t.Fatal("failed render changed existing output")
		}
		if _, err := os.Stat(filepath.Join(root, "output/absent.pdf")); !os.IsNotExist(err) {
			t.Fatalf("failed render created output: %v", err)
		}
	}
	if err := os.WriteFile(spec, valid, 0600); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{spec, profile} {
		if err := run([]string{"render", spec, "--output", output}, root, fixedDate, &bytes.Buffer{}); err == nil {
			t.Fatal("allowed replacing a YAML source")
		}
	}
}

func TestArguments(t *testing.T) {
	for _, args := range [][]string{
		nil, {"unknown"}, {"new"}, {"validate"}, {"render"}, {"init", "extra"},
		{"new", "profile", "--number"}, {"new", "profile", "--number="},
		{"new", "profile", "--unknown"}, {"new", "profile", "extra"},
		{"new", "profile", "--output=a", "--output=b"},
		{"validate", "spec", "--output=x"}, {"render", "spec", "--number=x"},
	} {
		if err := run(args, t.TempDir(), fixedDate, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{"--help"}, {"new", "--help"}, {"init", "-h"}} {
		var out bytes.Buffer
		if err := run(args, t.TempDir(), fixedDate, &out); err != nil || !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("%v: %v %s", args, err, out.String())
		}
	}
}

func TestInstalledBinary(t *testing.T) {
	binDir := t.TempDir()
	install := exec.Command("go", "install", ".")
	install.Env = append(os.Environ(), "GOBIN="+binDir)
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	root := t.TempDir()
	binary := filepath.Join(binDir, "invoicy")
	call := func(args ...string) ([]byte, error) {
		cmd := exec.Command(binary, args...)
		cmd.Dir = root
		return cmd.CombinedOutput()
	}
	if out, err := call("init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	for _, args := range [][]string{
		{"validate", "invoices/invoice.yml"},
		{"render", "invoices/invoice.yml"},
		{"new", "profiles/profiles.yml", "--number=INV-2026-09/01"},
		{"validate", "invoices/invoice-2026-09--01.yml"},
		{"render", "invoices/invoice-2026-09--01.yml"},
	} {
		if out, err := call(args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	out, err := call("validate", "invoices/missing.yml")
	if err == nil || !strings.Contains(string(out), "invoices/missing.yml") {
		t.Fatalf("expected nonzero exit and file error: %v %s", err, out)
	}
}

func TestSignatureValidationBeforeWriting(t *testing.T) {
	root := cliWorkspace(t)
	profile := filepath.Join(root, "profiles/example.yml")
	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`default_signature: ""`), []byte(`default_signature: "profiles/signature.png"`), 1)
	if err := os.WriteFile(profile, data, 0600); err != nil {
		t.Fatal(err)
	}
	signature := filepath.Join(root, "profiles/signature.png")
	if err := os.WriteFile(signature, []byte("invalid PNG"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"new", "profiles/example.yml", "--output=invoices/signed.yml"}, root, fixedDate, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("invalid signature accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "invoices/signed.yml")); !os.IsNotExist(err) {
		t.Fatalf("created invalid invoice: %v", err)
	}
	spec, err := invoice.New(root, "profiles/example.yml", "", fixedDate)
	if err != nil {
		t.Fatal(err)
	}
	if err := invoice.WriteSpecification(filepath.Join(root, "invoices/signed.yml"), spec); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "output/keep.pdf")
	if err := os.WriteFile(output, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{output, signature} {
		before, err := os.ReadFile(destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"render", "invoices/signed.yml", "--output", destination}, root, fixedDate, &bytes.Buffer{}); err == nil {
			t.Fatal("render accepted invalid signature or overwrote source")
		}
		after, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("changed %s: %v", destination, err)
		}
	}
}
