package pipeline

import "errors"

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

// ErrNotImplemented indicates that the generator pipeline has not been implemented yet.
var ErrNotImplemented = errors.New("effectsgen: generator pipeline not implemented yet")

// Run executes the effects contract generation pipeline. The implementation will populate
// generated client bindings once the remaining stages are complete.
func Run(Options) error {
	return ErrNotImplemented
}
