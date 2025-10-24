package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type packageInfo struct {
	ImportPath string
	Imports    []string
}

func main() {
	cmd := exec.Command("go", "list", "-json", "./internal/net/...")
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitErr.Stderr)
		}
		fmt.Fprintf(os.Stderr, "depscheck: failed to list packages: %v\n", err)
		os.Exit(1)
	}

	decoder := json.NewDecoder(bytes.NewReader(output))

	var violations []string
	for {
		var pkg packageInfo
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			fmt.Fprintf(os.Stderr, "depscheck: failed to decode package info: %v\n", err)
			os.Exit(1)
		}

		for _, imp := range pkg.Imports {
			if strings.HasPrefix(imp, "mine-and-die/server/internal/sim/internal") {
				violations = append(violations, fmt.Sprintf("%s -> %s", pkg.ImportPath, imp))
			}
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		fmt.Fprintln(os.Stderr, "depscheck: found forbidden imports:")
		for _, violation := range violations {
			fmt.Fprintf(os.Stderr, "  %s\n", violation)
		}
		os.Exit(1)
	}
}
