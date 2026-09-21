package invoice

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// WriteSpecification atomically creates a specification without replacing files.
func WriteSpecification(path string, s Specification) error {
	if _, err := s.Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("%s: YAML: %w", path, err)
	}
	return WriteAtomic(path, data, false)
}

// WriteAtomic publishes complete bytes in an existing directory.
func WriteAtomic(path string, data []byte, replace bool) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".invoicy-*")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if replace {
		err = os.Rename(f.Name(), path)
	} else {
		err = os.Link(f.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
