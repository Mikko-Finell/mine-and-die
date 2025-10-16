package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	contractGoMod    = "module example.com/contracts\n\ngo 1.24.3\n"
	contractTypesSrc = `package contract

type Payload interface {
        payloadMarker()
}

type ContractPayload struct{}

func (ContractPayload) payloadMarker() {}

type payloadSentinel struct{}

func (payloadSentinel) payloadMarker() {}

var NoPayload Payload = payloadSentinel{}

type LifecycleOwner int

const (
        LifecycleOwnerServer LifecycleOwner = iota
        LifecycleOwnerClient
)

type DeliveryKind string

const (
        DeliveryKindMelee  DeliveryKind = "melee"
        DeliveryKindRanged DeliveryKind = "ranged"
)

type EndResult uint8

const (
        EndResultUnknown EndResult = iota
        EndResultCancelled
)

type Definition struct {
        ID     string
        Spawn  Payload
        Update Payload
        End    Payload
        Owner  LifecycleOwner
}

type Registry []Definition
`
	contractPayloadsSrc = `package contract

type Coordinates struct {
        X int ` + "`json:\"x\"`" + `
        Y int ` + "`json:\"y\"`" + `
}

type AttackSpawnPayload struct {
        ContractPayload
        InstanceID string       ` + "`json:\"instanceId\"`" + `
        Location   Coordinates  ` + "`json:\"location\"`" + `
        Delivery   DeliveryKind ` + "`json:\"delivery\"`" + `
}

type AttackUpdatePayload struct {
        ContractPayload
        Seq    *int                  ` + "`json:\"seq,omitempty\"`" + `
        Params map[string]int        ` + "`json:\"params,omitempty\"`" + `
        Origin *Coordinates          ` + "`json:\"origin,omitempty\"`" + `
}

type AttackEndPayload struct {
        ContractPayload
        Reason string ` + "`json:\"reason\"`" + `
        Notes  string ` + "`json:\"notes,omitempty\" effect:\"optional\"`" + `
        Result EndResult ` + "`json:\"result\"`" + `
}

type FireballSpawnPayload = AttackSpawnPayload

type FireballUpdatePayload = AttackUpdatePayload

type FireballEndPayload = AttackEndPayload
`
	contractRegistrySrc = `package contract

const (
        EffectIDAttack   = "attack"
        EffectIDFireball = "fireball"
)

var BuiltInRegistry = Registry{
        {
                ID:     EffectIDAttack,
                Spawn:  (*AttackSpawnPayload)(nil),
                Update: (*AttackUpdatePayload)(nil),
                End:    (*AttackEndPayload)(nil),
                Owner:  LifecycleOwnerClient,
        },
        {
                ID:     EffectIDFireball,
                Spawn:  (*FireballSpawnPayload)(nil),
                Update: (*FireballUpdatePayload)(nil),
                End:    (*FireballEndPayload)(nil),
        },
}
`
)

// WriteContractFixtures materialises a minimal contract package under dir and returns the
// contracts directory alongside the registry file path.
func WriteContractFixtures(t testing.TB, dir string) (contractsDir, registryPath string) {
	t.Helper()

	contractsDir = filepath.Join(dir, "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("failed to create contracts dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(contractsDir, "go.mod"), []byte(contractGoMod), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "types.go"), []byte(contractTypesSrc), 0o644); err != nil {
		t.Fatalf("failed to write types.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "payloads.go"), []byte(contractPayloadsSrc), 0o644); err != nil {
		t.Fatalf("failed to write payloads.go: %v", err)
	}

	registryPath = filepath.Join(contractsDir, "registry.go")
	if err := os.WriteFile(registryPath, []byte(contractRegistrySrc), 0o644); err != nil {
		t.Fatalf("failed to write registry.go: %v", err)
	}

	return contractsDir, registryPath
}
