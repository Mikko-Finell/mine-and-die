package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"mine-and-die/tools/effectsgen/internal/pipeline"
)

func Execute(stdout io.Writer, stderr io.Writer, args []string) error {
	_ = stdout

	flagSet := flag.NewFlagSet("effectsgen", flag.ContinueOnError)
	flagSet.SetOutput(stderr)

	var contractsDir string
	var registryPath string
	var definitionsPath string
	var outputPath string

	flagSet.StringVar(&contractsDir, "contracts", "", "Path to the Go contracts package directory.")
	flagSet.StringVar(&registryPath, "registry", "", "Path to the Go registry source file.")
	flagSet.StringVar(&definitionsPath, "definitions", "", "Path to the JSON catalog definitions file.")
	flagSet.StringVar(&outputPath, "out", "", "Path to the generated TypeScript output file.")

	flagSet.Usage = func() {
		fmt.Fprintf(stderr, "Usage of %s:\n", flagSet.Name())
		flagSet.PrintDefaults()
	}

	if err := flagSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if contractsDir == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --contracts")
	}
	if registryPath == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --registry")
	}
	if definitionsPath == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --definitions")
	}
	if outputPath == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --out")
	}

	if extra := flagSet.Args(); len(extra) > 0 {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: unexpected arguments: %s", strings.Join(extra, " "))
	}

	options := pipeline.Options{
		ContractsDir:    contractsDir,
		RegistryPath:    registryPath,
		DefinitionsPath: definitionsPath,
		OutputPath:      outputPath,
	}

	return pipeline.Run(options)
}
