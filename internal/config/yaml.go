package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

// ReadYAML decodes one document, rejecting unknown fields and numeric amounts.
func ReadYAML(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("%s: YAML: %w", path, err)
	}
	if err := checkAmounts(&root, ""); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(value); err != nil {
		return fmt.Errorf("%s: YAML: %w", path, err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("expected exactly one YAML document")
		}
		return fmt.Errorf("%s: YAML: %w", path, err)
	}
	return nil
}

func checkAmounts(n *yaml.Node, path string) error {
	if n.Kind == yaml.AliasNode {
		return fmt.Errorf("%s: YAML aliases are not supported", path)
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			key, v := n.Content[i].Value, n.Content[i+1]
			field := key
			if path != "" {
				field = path + "." + key
			}
			if key == "amount" && (v.Kind != yaml.ScalarNode || v.Tag != "!!str") {
				return fmt.Errorf("%s: expected a quoted decimal string (line %d)", field, v.Line)
			}
			if err := checkAmounts(v, field); err != nil {
				return err
			}
		}
	} else {
		for i, child := range n.Content {
			field := path
			if n.Kind == yaml.SequenceNode {
				field = fmt.Sprintf("%s[%d]", path, i)
			}
			if err := checkAmounts(child, field); err != nil {
				return err
			}
		}
	}
	return nil
}
