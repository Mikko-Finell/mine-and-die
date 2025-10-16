//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/invopop/jsonschema"

	"mine-and-die/server/effects/catalog"
)

func main() {
	var outPath string
	flag.StringVar(&outPath, "out", "", "output path for the JSON schema")
	flag.Parse()

	if outPath == "" {
		log.Fatal("schema_generate: missing -out path")
	}

	schema, err := buildSchema()
	if err != nil {
		log.Fatalf("schema_generate: %v", err)
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("schema_generate: marshal schema: %v", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		log.Fatalf("schema_generate: create output dir: %v", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		log.Fatalf("schema_generate: write schema: %v", err)
	}
}

func buildSchema() (*jsonschema.Schema, error) {
	reflector := jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		DoNotReference:             true,
	}

	entrySchema := reflector.ReflectFromType(reflect.TypeOf(catalog.EntryDocument{}))
	if entrySchema == nil {
		return nil, fmt.Errorf("failed to reflect entry schema")
	}
	entrySchema.Version = ""
	entrySchema.Title = "Effect Catalog Entry"
	entrySchema.Description = "Designer-authored payload that references an authoritative contract definition."
	entrySchema.AdditionalProperties = &jsonschema.Schema{}

	arraySchema := &jsonschema.Schema{
		Type:        "array",
		Title:       "Array Catalog",
		Description: "Effect catalog expressed as an array of entry objects.",
		Items:       entrySchema,
	}

	objectSchema := &jsonschema.Schema{
		Type:                 "object",
		Title:                "Object Catalog",
		Description:          "Effect catalog expressed as an object keyed by entry ID.",
		AdditionalProperties: entrySchema,
	}

	root := &jsonschema.Schema{
		Version:     jsonschema.Version,
		Title:       "Mine & Die Effect Catalog",
		Description: "Designer-authored effect compositions consumed by the Mine & Die runtime.",
		OneOf: []*jsonschema.Schema{
			arraySchema,
			objectSchema,
		},
	}

	return root, nil
}
