package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestExecuteRunsPipeline(t *testing.T) {
	tempDir := t.TempDir()
	contractsDir := filepath.Join(tempDir, "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("failed to create contracts dir: %v", err)
	}
	registryPath := filepath.Join(contractsDir, "registry.go")
	if err := os.WriteFile(registryPath, []byte("package contract\n"), 0o644); err != nil {
		t.Fatalf("failed to write registry stub: %v", err)
	}
	definitionsPath := filepath.Join(tempDir, "definitions.json")
	definitions := `[
  {
    "id": "fireball",
    "contractId": "fireball",
    "definition": {"typeId": "fireball"},
    "jsEffect": "projectile/fireball"
  }
]`
	if err := os.WriteFile(definitionsPath, []byte(definitions), 0o644); err != nil {
		t.Fatalf("failed to write definitions stub: %v", err)
	}
	outputPath := filepath.Join(tempDir, "out", "effect-contracts.ts")

	err := Execute(io.Discard, io.Discard, []string{
		"--contracts=" + contractsDir,
		"--registry=" + registryPath,
		"--definitions=" + definitionsPath,
		"--out=" + outputPath,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read generated output: %v", err)
	}
	if !strings.Contains(string(data), "export const effectCatalog") {
		t.Fatalf("expected generated file to contain effect catalog, got:\n%s", string(data))
	}
}
