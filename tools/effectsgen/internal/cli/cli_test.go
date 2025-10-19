package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mine-and-die/tools/effectsgen/internal/testutil"
)

func TestExecuteRequiresContractsFlag(t *testing.T) {
	err := Execute(io.Discard, io.Discard, []string{
		"--registry=server/effects/contract/registry.go",
		"--definitions=config/effects/definitions.json",
		"--out=client/generated/effect-contracts.ts",
		"--hash-go=server/effects/contract/effect_catalog_hash.generated.go",
		"--hash-go-pkg=contract",
		"--hash-ts=client/generated/effect-contracts-hash.ts",
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
	contractsDir, registryPath := testutil.WriteContractFixtures(t, tempDir)
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
	hashGoPath := filepath.Join(tempDir, "out", "effect_catalog_hash.generated.go")
	hashTSPath := filepath.Join(tempDir, "out", "effect-contracts-hash.ts")

	err := Execute(io.Discard, io.Discard, []string{
		"--contracts=" + contractsDir,
		"--registry=" + registryPath,
		"--definitions=" + definitionsPath,
		"--out=" + outputPath,
		"--hash-go=" + hashGoPath,
		"--hash-go-pkg=contract",
		"--hash-ts=" + hashTSPath,
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

	if _, err := os.Stat(hashGoPath); err != nil {
		t.Fatalf("expected Go hash output file to be created: %v", err)
	}
	if _, err := os.Stat(hashTSPath); err != nil {
		t.Fatalf("expected TypeScript hash output file to be created: %v", err)
	}
}
