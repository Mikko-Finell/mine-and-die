package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options defines the inputs required to execute the effects contract generation pipeline.
type Options struct {
	// ContractsDir points to the Go package containing contract payload structs.
	ContractsDir string
	// RegistryPath is the path to the Go source file that registers contract IDs.
	RegistryPath string
	// DefinitionsPath locates the JSON catalog definitions authored by designers.
	DefinitionsPath string
	// OutputPath identifies the target TypeScript file that will be generated.
	OutputPath string
}

// Run executes the effects contract generation pipeline. The current implementation loads the
// designer-authored catalog definitions and emits a typed TypeScript snapshot that mirrors the
// server metadata contract. Subsequent iterations will extend this to include generated payload
// interfaces and registry bindings.
func Run(opts Options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}

	entries, err := loadCatalogEntries(opts.DefinitionsPath)
	if err != nil {
		return err
	}

	module, err := generateEffectCatalogModule(entries)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("effectsgen: failed creating output directory: %w", err)
	}
	if err := os.WriteFile(opts.OutputPath, module, 0o644); err != nil {
		return fmt.Errorf("effectsgen: failed writing output %s: %w", opts.OutputPath, err)
	}

	return nil
}

func validateOptions(opts Options) error {
	if strings.TrimSpace(opts.ContractsDir) == "" {
		return fmt.Errorf("effectsgen: contracts directory is required")
	}
	if strings.TrimSpace(opts.RegistryPath) == "" {
		return fmt.Errorf("effectsgen: registry path is required")
	}
	if strings.TrimSpace(opts.DefinitionsPath) == "" {
		return fmt.Errorf("effectsgen: definitions path is required")
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return fmt.Errorf("effectsgen: output path is required")
	}

	info, err := os.Stat(opts.ContractsDir)
	if err != nil {
		return fmt.Errorf("effectsgen: unable to stat contracts directory %s: %w", opts.ContractsDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("effectsgen: contracts path %s is not a directory", opts.ContractsDir)
	}

	if err := ensureFileExists(opts.RegistryPath); err != nil {
		return fmt.Errorf("effectsgen: registry file error: %w", err)
	}
	if err := ensureFileExists(opts.DefinitionsPath); err != nil {
		return fmt.Errorf("effectsgen: definitions file error: %w", err)
	}

	return nil
}

func ensureFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("unable to stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}
