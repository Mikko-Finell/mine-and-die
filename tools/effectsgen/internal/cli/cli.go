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
	var hashGoPath string
	var hashGoPackage string
	var hashTSPath string

	flagSet.StringVar(&contractsDir, "contracts", "", "Path to the Go contracts package directory.")
	flagSet.StringVar(&registryPath, "registry", "", "Path to the Go registry source file.")
	flagSet.StringVar(&definitionsPath, "definitions", "", "Path to the JSON catalog definitions file.")
	flagSet.StringVar(&outputPath, "out", "", "Path to the generated TypeScript output file.")
	flagSet.StringVar(&hashGoPath, "hash-go", "", "Path to the generated Go catalog hash file.")
	flagSet.StringVar(&hashGoPackage, "hash-go-pkg", "", "Go package name for the generated catalog hash file.")
	flagSet.StringVar(&hashTSPath, "hash-ts", "", "Path to the generated TypeScript catalog hash file.")

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
	if hashGoPath == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --hash-go")
	}
	if hashGoPackage == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --hash-go-pkg")
	}
	if hashTSPath == "" {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: missing required flag --hash-ts")
	}

	if extra := flagSet.Args(); len(extra) > 0 {
		flagSet.Usage()
		return fmt.Errorf("effectsgen: unexpected arguments: %s", strings.Join(extra, " "))
	}

	options := pipeline.Options{
		ContractsDir:     contractsDir,
		RegistryPath:     registryPath,
		DefinitionsPath:  definitionsPath,
		OutputPath:       outputPath,
		HashGoOutputPath: hashGoPath,
		HashGoPackage:    hashGoPackage,
		HashTSOutputPath: hashTSPath,
	}

	return pipeline.Run(options)
}
