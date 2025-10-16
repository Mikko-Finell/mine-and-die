package cli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"mine-and-die/tools/effectsgen/internal/pipeline"
)

func TestExecuteRequiresContractsFlag(t *testing.T) {
	err := Execute(io.Discard, io.Discard, []string{
		"--registry=server/effects/contract/registry.go",
		"--definitions=config/effects/definitions.json",
		"--out=client/generated/effect-contracts.ts",
	})

	if err == nil {
		t.Fatal("expected error when contracts flag missing")
	}
	if !strings.Contains(err.Error(), "--contracts") {
		t.Fatalf("expected missing contracts flag error, got %v", err)
	}
}

func TestExecuteReturnsPipelineError(t *testing.T) {
	err := Execute(io.Discard, io.Discard, []string{
		"--contracts=server/effects/contract",
		"--registry=server/effects/contract/registry.go",
		"--definitions=config/effects/definitions.json",
		"--out=client/generated/effect-contracts.ts",
	})

	if !errors.Is(err, pipeline.ErrNotImplemented) {
		t.Fatalf("expected pipeline.ErrNotImplemented, got %v", err)
	}
}
