package invoice

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/*
var workspaceTemplates embed.FS

// Init creates workspace directories and examples without replacing existing files.
func Init(root string) error {
	for _, name := range []string{"profiles", "invoices", "output"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	for _, file := range []struct{ source, destination string }{
		{"profiles.yml", "profiles/profiles.yml"},
		{"invoice.yml", "invoices/invoice.yml"},
		{"README.md", "README.md"},
	} {
		path := filepath.Join(root, file.destination)
		if _, err := os.Lstat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", path, err)
		}
		data, err := workspaceTemplates.ReadFile("templates/" + file.source)
		if err != nil {
			return err
		}
		if err := WriteAtomic(path, data, false); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}
