package invoice

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// WriteSpecification creates a specification without replacing files.
func WriteSpecification(path string, s Specification) error {
	if _, err := s.Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("%s: YAML: %w", path, err)
	}
	return WriteFile(path, data, false)
}

// WriteFile writes data to path in an existing directory. It refuses to replace
// an existing file unless replace is set and removes the file if writing fails.
func WriteFile(path string, data []byte, replace bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !replace {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
