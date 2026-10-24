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

// TestMain keeps the developer's INVOICY_* variables (for example from direnv)
// from leaking into tests, including the installed binary started by one of them.
func TestMain(m *testing.M) {
	os.Unsetenv("INVOICY_WORKSPACE_DIR")
	os.Unsetenv("INVOICY_DEFAULT_PROFILE")
	os.Exit(m.Run())
}

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
	install := exec.CommandContext(t.Context(), "go", "install", ".")
	install.Env = append(os.Environ(), "GOBIN="+binDir)
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	root := t.TempDir()
	binary := filepath.Join(binDir, "invoicy")
	call := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), binary, args...)
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

func TestEnvSettings(t *testing.T) {
	t.Setenv("INVOICY_DEFAULT_PROFILE", "")
	t.Setenv("INVOICY_WORKSPACE_DIR", "")
	root := cliWorkspace(t)
	if err := run([]string{"new"}, root, fixedDate, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("new without any profile: %v", err)
	}
	t.Setenv("INVOICY_DEFAULT_PROFILE", "example")
	var out bytes.Buffer
	if err := run([]string{"new"}, root, fixedDate, &out); err != nil || !strings.Contains(out.String(), "invoices/invoice-2026-09--01.yml") {
		t.Fatalf("profile name not resolved in profiles dir: %q, %v", out.String(), err)
	}
	t.Setenv("INVOICY_DEFAULT_PROFILE", "profiles/missing.yml")
	if err := run([]string{"new", "--number=INV-2026-09/02"}, root, fixedDate, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "missing.yml") {
		t.Fatalf("env profile ignored: %v", err)
	}
	if err := run([]string{"new", "profiles/example.yml", "--number=INV-2026-09/02"}, root, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatalf("argument must beat env: %v", err)
	}
}

func TestWorkspace(t *testing.T) {
	t.Setenv("INVOICY_DEFAULT_PROFILE", "")
	cwd, ws, other := t.TempDir(), filepath.Join(t.TempDir(), "ws"), filepath.Join(t.TempDir(), "other")

	// The environment variable creates and uses a workspace anywhere on disk.
	t.Setenv("INVOICY_WORKSPACE_DIR", ws)
	if err := run([]string{"init"}, cwd, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"profiles/profiles.yml", "invoices/invoice.yml", "output"} {
		if _, err := os.Stat(filepath.Join(ws, name)); err != nil {
			t.Fatalf("init did not use the workspace: %v", err)
		}
	}
	if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 0 {
		t.Fatalf("init wrote to the current directory: %v %v", entries, err)
	}
	data, err := os.ReadFile("../../examples/profiles/example-2026.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "profiles/mario.yml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"new", "mario"}, cwd, fixedDate, &out); err != nil || !strings.Contains(out.String(), "invoices/invoice-2026-09--01.yml") {
		t.Fatalf("new: %q, %v", out.String(), err)
	}
	if err := run([]string{"render", "invoices/invoice-2026-09--01.yml"}, cwd, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, "output/INV-2026-09--01.pdf")); err != nil {
		t.Fatalf("PDF not in workspace output: %v", err)
	}

	// The flag beats the environment, and a relative workspace is relative to the current directory.
	if err := run([]string{"init", "--workspace", "other"}, filepath.Dir(other), fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, "profiles/profiles.yml")); err != nil {
		t.Fatalf("flag ignored: %v", err)
	}

	// Without either, the current directory is the workspace.
	t.Setenv("INVOICY_WORKSPACE_DIR", "")
	plain := t.TempDir()
	if err := run([]string{"init"}, plain, fixedDate, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(plain, "invoices/invoice.yml")); err != nil {
		t.Fatal(err)
	}
}

func TestHelpText(t *testing.T) {
	plain := helpText(false)
	if strings.Contains(plain, "\x1b") {
		t.Fatal("plain help contains ANSI codes")
	}
	for _, want := range []string{"Usage:", "Commands:", "Environment:", "Examples:", "INVOICE_FILE", "PDF_FILE", "--number NUMBER", "--output INVOICE_FILE", "--output PDF_FILE"} {
		if !strings.Contains(plain, want) {
			t.Errorf("help is missing %q", want)
		}
	}
	if !strings.Contains(helpText(true), "\x1b[1m") {
		t.Fatal("colored help has no styling")
	}
}
