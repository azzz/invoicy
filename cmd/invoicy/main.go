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
	input, number, output, workspace string
	help                             bool
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
			if name != "--output" && name != "--number" && name != "--workspace" {
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
			switch name {
			case "--output":
				o.output = value
			case "--number":
				o.number = value
			default:
				o.workspace = value
			}
		} else {
			if o.input != "" || command == "init" {
				return o, fmt.Errorf("%s: unexpected argument %q", command, arg)
			}
			o.input = arg
		}
	}
	if !o.help && command != "init" && command != "new" && o.input == "" {
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
		_, err := io.WriteString(out, helpText(useColor(out)))
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
		_, err := io.WriteString(out, helpText(useColor(out)))
		return err
	}
	settings, err := config.SettingsFromEnv()
	if err != nil {
		return err
	}
	// The workspace is --workspace, else INVOICY_WORKSPACE_DIR, else the current
	// directory. Every relative path below is resolved from it.
	if o.workspace == "" {
		o.workspace = settings.WorkspaceDir
	}
	if o.workspace != "" && !filepath.IsAbs(o.workspace) {
		root = filepath.Join(root, o.workspace)
	} else if o.workspace != "" {
		root = o.workspace
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
		_, err := fmt.Fprintln(out, "Initialized workspace (profiles, invoices, output, signatures) with example YAML files and README.md (existing files preserved)")
		return err
	}
	if command == "new" {
		if o.input == "" {
			o.input = settings.DefaultProfile
		}
		if o.input == "" {
			return errors.New("new: expected a profile path (argument or INVOICY_DEFAULT_PROFILE)")
		}
		// A bare name such as "mario" means <profiles dir>/mario.yml.
		if !strings.ContainsAny(o.input, `/\.`) {
			o.input = filepath.Join(config.ProfilesDir, o.input+".yml")
		}
		s, err := invoice.New(root, o.input, o.number, now)
		if err != nil {
			return err
		}
		if o.output == "" {
			o.output = filepath.Join(config.InvoicesDir, invoice.Filename(s.Number, ".yml"))
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
		o.output = filepath.Join(config.OutputDir, invoice.PDFFilename(inv.Specification.Number))
	}
	output := pathInRoot(o.output)
	if err := protectSources(output, inv); err != nil {
		return err
	}
	data, err := render.PDF(inv)
	if err != nil {
		return err
	}
	if err := invoice.WriteFile(output, data, true); err != nil {
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
