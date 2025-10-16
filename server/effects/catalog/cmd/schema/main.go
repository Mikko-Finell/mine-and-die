package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"

	"mine-and-die/server/effects/catalog"
)

func main() {
	var outPath string
	flag.StringVar(&outPath, "out", "", "path to write the JSON schema")
	flag.Parse()

	if outPath == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(1)
	}

	schema := buildSchema()

	if err := writeSchema(outPath, schema); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write schema: %v\n", err)
		os.Exit(1)
	}
}

func buildSchema() *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: true,
	}
	schema := reflector.Reflect(new(catalog.FileDefinitions))
	schema.Title = "Mine & Die Effect Catalog"
	schema.Description = "Validates designer-authored entries in config/effects/definitions.json"
	return schema
}

func writeSchema(outPath string, schema *jsonschema.Schema) error {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schema: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create schema directory: %w", err)
	}

	tmpPath := outPath + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write temp schema: %w", err)
	}

	if err := os.Rename(tmpPath, outPath); err != nil {
		return fmt.Errorf("replace schema: %w", err)
	}

	return nil
}
