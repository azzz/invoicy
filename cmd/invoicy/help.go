package main

import (
	"io"
	"os"
	"strings"
)

// useColor enables ANSI styling only for terminals, honoring NO_COLOR.
func useColor(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func helpText(color bool) string {
	style := func(code string) func(string) string {
		return func(s string) string {
			if !color {
				return s
			}
			return "\x1b[" + code + "m" + s + "\x1b[0m"
		}
	}
	bold, cyan, dim := style("1"), style("36"), style("2")

	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\n") }
	heading := func(s string) { line(""); line(bold(s)) }
	// command prints a synopsis followed by an indented description.
	command := func(synopsis string, description ...string) {
		line("")
		line("  " + bold(synopsis))
		for _, d := range description {
			line("      " + d)
		}
	}
	// arg aligns an argument, flag, or variable name with its explanation.
	arg := func(indent int, name, description string) {
		pad := 26 - len(name)
		if pad < 2 {
			pad = 2
		}
		line(strings.Repeat(" ", indent) + cyan(name) + strings.Repeat(" ", pad) + description)
	}
	example := func(cmd string) { line("      " + dim("Example: ") + cyan(cmd)) }

	line(bold("invoicy") + dim(" - YAML in, single-page A4 PDF out"))
	heading("Usage:")
	line("  invoicy <command> [arguments] [flags]")

	heading("Workspace:")
	line("  All data lives in one workspace directory with fixed subdirectories:")
	line("  " + cyan("profiles/") + ", " + cyan("invoices/") + ", " + cyan("output/") + " and " + cyan("signatures/") + ". Relative paths are resolved from it.")
	line("  " + dim("Chosen by --workspace DIR, else INVOICY_WORKSPACE_DIR, else the current directory."))

	heading("Commands:")

	command("init [--workspace DIR]",
		"Create profiles/, invoices/, output/ and signatures/ with example files.",
		"Existing files and directories are left untouched.")
	example("invoicy init --workspace ~/invoices")

	command("new [PROFILE] [--number NUMBER] [--output INVOICE_FILE] [--workspace DIR]",
		"Create an invoice file from a profile, copying the profile's default works.",
		"Never replaces an existing file; use --number to create another invoice.")
	arg(6, "PROFILE", "Profile path (profiles/mario.yml) or bare name (mario means")
	arg(6, "", "profiles/mario.yml). Default: $INVOICY_DEFAULT_PROFILE.")
	arg(6, "--number NUMBER", "Invoice number. Default: INV-YYYY-MM/01 for today's date.")
	arg(6, "--output INVOICE_FILE", "Where to write it. Default: invoices/invoice-<number>.yml.")
	example("invoicy new mario --number INV-2026-10/02")

	command("validate INVOICE_FILE [--workspace DIR]",
		"Check the invoice file, its profile, fonts and one-page fit; print the total.",
		"The profile path inside the invoice must stay inside the workspace.")
	arg(6, "INVOICE_FILE", "Path to an invoice YAML file.")
	arg(6, "", "A PNG signature (mario.png) is read from signatures/.")
	example("invoicy validate invoices/invoice-2026-10--02.yml")

	command("render INVOICE_FILE [--output PDF_FILE] [--workspace DIR]",
		"Validate the invoice file and write a PDF. An existing PDF is replaced",
		"only after rendering succeeds.")
	arg(6, "INVOICE_FILE", "Path to an invoice YAML file. A PNG signature (mario.png)")
	arg(6, "", "is read from signatures/ and scaled to fit 50 x 20 mm.")
	arg(6, "--output PDF_FILE", "Where to write the PDF; the parent directory must exist.")
	arg(6, "", "Default: output/<number>.pdf.")
	example("invoicy render invoices/invoice-2026-10--02.yml")

	command("help", "Show this help (also -h and --help).")

	heading("Environment:")
	arg(2, "INVOICY_WORKSPACE_DIR", "Workspace directory. Default: the current directory.")
	arg(2, "INVOICY_DEFAULT_PROFILE", "Profile for new when PROFILE is omitted. Default: none.")
	line("  " + dim("Precedence: argument or flag, then environment, then default."))

	heading("Examples:")
	line("  " + cyan("invoicy init"))
	line("  " + cyan("invoicy new mario --number INV-2026-10/02"))
	line("  " + cyan("invoicy validate invoices/invoice-2026-10--02.yml"))
	line("  " + cyan("invoicy render invoices/invoice-2026-10--02.yml"))
	return b.String()
}
