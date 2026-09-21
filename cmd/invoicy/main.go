package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azzz/invoicy/internal/config"
	"github.com/azzz/invoicy/internal/invoice"
	"github.com/azzz/invoicy/internal/render"
)

const usage = `Usage:
  invoicy init
  invoicy new PROFILE [--number NUMBER] [--output FILE]
  invoicy validate SPECIFICATION
  invoicy render SPECIFICATION [--output FILE]

Profile paths are relative to the current working directory.
new uses today's local date and INV-YYYY-MM/01 unless --number is given.
Output parent directories must exist. new never replaces an existing file.
`

func main() {
	root, err := os.Getwd()
	if err == nil {
		err = run(os.Args[1:], root, time.Now(), os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "invoicy:", err)
		os.Exit(1)
	}
}

type options struct {
	input, number, output string
	help                  bool
}

func parse(command string, args []string) (options, error) {
	var o options
	seen := make(map[string]bool)
	positional := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !positional && arg == "--" {
			positional = true
			continue
		}
		if !positional && (arg == "--help" || arg == "-h") {
			o.help = true
			continue
		}
		if !positional && strings.HasPrefix(arg, "-") {
			name, value, inline := strings.Cut(arg, "=")
			if name != "--output" && name != "--number" {
				return o, fmt.Errorf("%s: unknown option %s", command, name)
			}
			if (name == "--number" && command != "new") || (name == "--output" && command != "new" && command != "render") {
				return o, fmt.Errorf("%s: option %s is not supported", command, name)
			}
			if seen[name] {
				return o, fmt.Errorf("%s: repeated option %s", command, name)
			}
			seen[name] = true
			if !inline {
				i++
				if i >= len(args) || strings.HasPrefix(args[i], "--") {
					return o, fmt.Errorf("%s: %s requires a value", command, name)
				}
				value = args[i]
			}
			if value == "" {
				return o, fmt.Errorf("%s: %s must not be empty", command, name)
			}
			if name == "--output" {
				o.output = value
			} else {
				o.number = value
			}
		} else {
			if o.input != "" || command == "init" {
				return o, fmt.Errorf("%s: unexpected argument %q", command, arg)
			}
			o.input = arg
		}
	}
	if !o.help && command != "init" && o.input == "" {
		return o, fmt.Errorf("%s: expected an input path", command)
	}
	return o, nil
}

func run(args []string, root string, now time.Time, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("expected a command; use invoicy --help")
	}
	command := args[0]
	if command == "--help" || command == "-h" || command == "help" {
		_, err := io.WriteString(out, usage)
		return err
	}
	if command != "init" && command != "new" && command != "validate" && command != "render" {
		return fmt.Errorf("unknown command %q; use invoicy --help", command)
	}
	o, err := parse(command, args[1:])
	if err != nil {
		return err
	}
	if o.help {
		_, err := io.WriteString(out, usage)
		return err
	}
	pathInRoot := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(root, path)
	}
	if command == "init" {
		if err := invoice.Init(root); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, "Initialized workspace with example YAML files and README.md (existing files preserved)")
		return err
	}
	if command == "new" {
		s, err := invoice.New(root, o.input, o.number, now)
		if err != nil {
			return err
		}
		if o.output == "" {
			o.output = filepath.Join("invoices", invoice.Filename(s.Number, ".yml"))
		}
		inv, err := invoice.Resolve(root, o.output, s)
		if err != nil {
			return err
		}
		if err := render.Check(inv); err != nil {
			return err
		}
		if err := invoice.WriteSpecification(pathInRoot(o.output), s); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, o.output)
		return err
	}
	inv, err := invoice.Load(root, o.input)
	if err != nil {
		return err
	}
	if command == "validate" {
		if err := render.Check(inv); err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%s: valid (%s %s)\n", o.input, inv.Profile.Currency, config.FormatMoney(inv.Total))
		return err
	}
	if o.output == "" {
		o.output = filepath.Join("output", invoice.PDFFilename(inv.Specification.Number))
	}
	output := pathInRoot(o.output)
	if err := protectSources(output, inv); err != nil {
		return err
	}
	data, err := render.PDF(inv)
	if err != nil {
		return err
	}
	if err := invoice.WriteAtomic(output, data, true); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, o.output)
	return err
}

func protectSources(output string, inv *invoice.Invoice) error {
	for _, source := range []string{inv.Source, inv.ProfileSource, inv.SignatureSource} {
		if source == "" {
			continue
		}
		a, err := filepath.Abs(output)
		if err != nil {
			return err
		}
		b, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		x, xe := os.Stat(a)
		y, ye := os.Stat(b)
		if a == b || (xe == nil && ye == nil && os.SameFile(x, y)) {
			return fmt.Errorf("%s: output must not replace an input file", output)
		}
	}
	return nil
}
