package catalog

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mine-and-die/server/effects/contract"
)

type memorySource struct {
	path string
	data []byte
	err  error
}

func (m memorySource) Load() ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return append([]byte(nil), m.data...), nil
}

func (m memorySource) Path() string {
	return m.path
}

func TestResolverLoadArray(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}
	entry := map[string]any{
		"id":         "attack",
		"contractId": "attack",
		"definition": map[string]any{
			"typeId":        "attack",
			"delivery":      "area",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "all-in-path",
			"lifetimeTicks": 1,
			"hooks":         map[string]any{"onSpawn": "swing"},
			"client": map[string]any{
				"sendSpawn":   true,
				"sendUpdates": false,
				"sendEnd":     false,
			},
			"end": map[string]any{"kind": 1},
		},
		"jsEffect":   "attack/basic",
		"parameters": map[string]any{"reach": 40},
	}
	data, err := json.Marshal([]map[string]any{entry})
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}

	resolver, err := NewResolver(reg, memorySource{path: "inline.json", data: data})
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}

	defs := resolver.DefinitionsByContractID()
	def, ok := defs["attack"]
	if !ok {
		t.Fatalf("expected definition for attack")
	}
	if def.Delivery != contract.DeliveryKindArea {
		t.Fatalf("expected delivery area, got %q", def.Delivery)
	}

	entrySnapshot, ok := resolver.Resolve("attack")
	if !ok {
		t.Fatalf("expected to resolve attack entry")
	}
	if entrySnapshot.ContractID != "attack" {
		t.Fatalf("expected contractId attack, got %q", entrySnapshot.ContractID)
	}
	if len(entrySnapshot.Blocks) == 0 {
		t.Fatalf("expected metadata blocks")
	}
	if _, ok := entrySnapshot.Blocks["jsEffect"]; !ok {
		t.Fatalf("expected jsEffect metadata block")
	}
	if entrySnapshot.Contract == nil || entrySnapshot.Contract.Owner != contract.LifecycleOwnerClient {
		t.Fatalf("expected entry contract owner to be client-managed")
	}
	if entrySnapshot.Definition == nil || !entrySnapshot.Definition.Client.ManagedByClient {
		t.Fatalf("expected managedByClient to be derived from contract ownership")
	}
}

func TestResolverObjectSyntax(t *testing.T) {
	reg := contract.Registry{
		{ID: "fireball", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}
	entry := map[string]any{
		"contractId": "fireball",
		"definition": map[string]any{
			"typeId":        "fireball",
			"delivery":      "area",
			"shape":         "circle",
			"motion":        "linear",
			"impact":        "first-hit",
			"lifetimeTicks": 45,
			"hooks":         map[string]any{"onSpawn": "projectile", "onTick": "projectile"},
			"client":        map[string]any{"sendSpawn": true, "sendUpdates": true, "sendEnd": true},
			"end":           map[string]any{"kind": 0},
		},
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	data := []byte("{" + "\"fireball\":" + string(payload) + "}")

	resolver, err := NewResolver(reg, memorySource{path: "object.json", data: data})
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}

	def, ok := resolver.DefinitionsByContractID()["fireball"]
	if !ok || def.Motion != contract.MotionKindLinear {
		t.Fatalf("expected fireball definition with linear motion")
	}

	if _, ok := resolver.Resolve("fireball"); !ok {
		t.Fatalf("expected resolve to succeed")
	}
}

func TestResolverReloadOverrides(t *testing.T) {
	reg := contract.Registry{
		{ID: "burning", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}

	first := memorySource{path: "base.json", data: mustMarshal([]map[string]any{{
		"id":         "burning",
		"contractId": "burning",
		"definition": map[string]any{
			"typeId":        "burning",
			"delivery":      "target",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "first-hit",
			"lifetimeTicks": 1,
			"hooks":         map[string]any{"onSpawn": "ignite"},
			"client":        map[string]any{"sendSpawn": true, "sendUpdates": false, "sendEnd": true},
			"end":           map[string]any{"kind": 1},
		},
	}})}
	second := memorySource{path: "override.json", data: mustMarshal([]map[string]any{{
		"id":         "burning",
		"contractId": "burning",
		"definition": map[string]any{
			"typeId":        "burning",
			"delivery":      "target",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "first-hit",
			"lifetimeTicks": 3,
			"hooks":         map[string]any{"onSpawn": "ignite"},
			"client":        map[string]any{"sendSpawn": true, "sendUpdates": false, "sendEnd": true},
			"end":           map[string]any{"kind": 1},
		},
	}})}

	resolver, err := NewResolver(reg, first, second)
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}

	def := resolver.DefinitionsByContractID()["burning"]
	if def.LifetimeTicks != 3 {
		t.Fatalf("expected override lifetime 3, got %d", def.LifetimeTicks)
	}

	// Mutate the override source to confirm Reload picks up changes.
	second.data = mustMarshal([]map[string]any{{
		"id":         "burning",
		"contractId": "burning",
		"definition": map[string]any{
			"typeId":        "burning",
			"delivery":      "target",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "first-hit",
			"lifetimeTicks": 6,
			"hooks":         map[string]any{"onSpawn": "ignite"},
			"client":        map[string]any{"sendSpawn": true, "sendUpdates": false, "sendEnd": true},
			"end":           map[string]any{"kind": 1},
		},
	}})

	resolver.mu.Lock()
	resolver.sources[1] = second
	resolver.mu.Unlock()

	if err := resolver.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if resolver.DefinitionsByContractID()["burning"].LifetimeTicks != 6 {
		t.Fatalf("expected lifetime 6 after reload")
	}
}

func TestResolverEnforcesClientOwnershipInvariants(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload, Owner: contract.LifecycleOwnerClient},
	}
	base := map[string]any{
		"id":         "attack",
		"contractId": "attack",
		"definition": map[string]any{
			"typeId":        "attack",
			"delivery":      "area",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "all-in-path",
			"lifetimeTicks": 1,
			"hooks":         map[string]any{"onSpawn": "swing"},
			"client": map[string]any{
				"sendSpawn":   true,
				"sendUpdates": false,
				"sendEnd":     false,
			},
			"end": map[string]any{"kind": 1},
		},
	}

	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{
			name: "updates-enabled",
			mutate: func(def map[string]any) {
				client := def["client"].(map[string]any)
				client["sendUpdates"] = true
			},
			want: "enables sendUpdates",
		},
		{
			name: "end-enabled",
			mutate: func(def map[string]any) {
				client := def["client"].(map[string]any)
				client["sendEnd"] = true
			},
			want: "enables sendEnd",
		},
		{
			name: "lifetime-not-one",
			mutate: func(def map[string]any) {
				def["lifetimeTicks"] = 2
			},
			want: "lifetimeTicks to 2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var entry map[string]any
			if err := json.Unmarshal(mustMarshal(base), &entry); err != nil {
				t.Fatalf("failed cloning entry: %v", err)
			}
			definition := entry["definition"].(map[string]any)
			tc.mutate(definition)
			data := mustMarshal([]map[string]any{entry})

			resolver, err := NewResolver(reg, memorySource{path: "managed.json", data: data})
			if err == nil {
				t.Fatalf("expected NewResolver to fail validation")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error to contain %q, got %v", tc.want, err)
			}
			if resolver != nil {
				t.Fatalf("expected resolver to be nil when validation fails")
			}
		})
	}
}

func TestResolverRejectsManagedByClientField(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}

	entry := map[string]any{
		"id":         "attack",
		"contractId": "attack",
		"definition": map[string]any{
			"typeId":        "attack",
			"delivery":      "area",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "all-in-path",
			"lifetimeTicks": 1,
			"client": map[string]any{
				"sendSpawn":       true,
				"sendUpdates":     false,
				"sendEnd":         false,
				"managedByClient": true,
			},
		},
	}

	data := mustMarshal([]map[string]any{entry})

	resolver, err := NewResolver(reg, memorySource{path: "managed.json", data: data})
	if err == nil {
		t.Fatalf("expected NewResolver to fail when managedByClient is declared explicitly")
	}
	if !strings.Contains(err.Error(), "must not set definition.client.managedByClient") {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolver != nil {
		t.Fatalf("expected resolver to be nil on error")
	}
}

func TestResolverRejectsDuplicateIDs(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}

	duplicate := mustMarshal([]map[string]any{
		{
			"id":         "attack",
			"contractId": "attack",
			"definition": map[string]any{
				"typeId":        "attack",
				"delivery":      "area",
				"shape":         "rect",
				"motion":        "instant",
				"impact":        "first-hit",
				"lifetimeTicks": 1,
				"client":        map[string]any{"sendSpawn": true, "sendEnd": true},
				"end":           map[string]any{"kind": 1},
			},
		},
		{
			"id":         "attack",
			"contractId": "attack",
			"definition": map[string]any{
				"typeId":        "attack",
				"delivery":      "area",
				"shape":         "rect",
				"motion":        "instant",
				"impact":        "first-hit",
				"lifetimeTicks": 2,
				"client":        map[string]any{"sendSpawn": true, "sendEnd": true},
				"end":           map[string]any{"kind": 1},
			},
		},
	})

	resolver, err := NewResolver(reg, memorySource{path: "duplicate.json", data: duplicate})
	if err == nil {
		t.Fatalf("expected NewResolver to fail due to duplicate ids")
	}
	if resolver != nil {
		t.Fatalf("expected resolver to be nil when duplicates are present")
	}
}

func TestResolverRejectsUnknownContractID(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}

	payload := mustMarshal([]map[string]any{{
		"id":         "fireball",
		"contractId": "fireball",
		"definition": map[string]any{
			"typeId":        "fireball",
			"delivery":      "area",
			"shape":         "circle",
			"motion":        "linear",
			"impact":        "first-hit",
			"lifetimeTicks": 30,
			"client":        map[string]any{"sendSpawn": true, "sendEnd": true},
			"end":           map[string]any{"kind": 0},
		},
	}})

	resolver, err := NewResolver(reg, memorySource{path: "unknown.json", data: payload})
	if err == nil {
		t.Fatalf("expected NewResolver to fail for unknown contract id")
	}
	if resolver != nil {
		t.Fatalf("expected resolver to be nil when contract id is unknown")
	}
}

func TestLoadIgnoresMissingFiles(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	resolver, err := Load(reg, missing)
	if err != nil {
		t.Fatalf("Load returned error for missing path: %v", err)
	}
	if resolver == nil {
		t.Fatalf("expected resolver to be created even when files are missing")
	}
	if entries := resolver.Entries(); len(entries) != 0 {
		t.Fatalf("expected no entries when sources are missing, got %d", len(entries))
	}
}

func TestEntriesReturnClones(t *testing.T) {
	reg := contract.Registry{
		{ID: "attack", Spawn: contract.NoPayload, Update: contract.NoPayload, End: contract.NoPayload},
	}
	payload := mustMarshal([]map[string]any{{
		"id":         "attack",
		"contractId": "attack",
		"definition": map[string]any{
			"typeId":        "attack",
			"delivery":      "area",
			"shape":         "rect",
			"motion":        "instant",
			"impact":        "first-hit",
			"lifetimeTicks": 1,
			"client":        map[string]any{"sendSpawn": true, "sendEnd": true},
			"end":           map[string]any{"kind": 1},
		},
	}})

	resolver, err := NewResolver(reg, memorySource{path: "catalog.json", data: payload})
	if err != nil {
		t.Fatalf("failed to construct resolver: %v", err)
	}

	entries := resolver.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected single entry, got %d", len(entries))
	}
	entry := entries["attack"]
	if entry.Blocks == nil {
		entry.Blocks = make(map[string]json.RawMessage)
	}
	entry.Blocks["mutated"] = json.RawMessage(`"yes"`)
	entry.ContractID = "mutated"

	snapshot := resolver.Entries()
	if snapshot["attack"].ContractID != "attack" {
		t.Fatalf("expected resolver entries to remain unchanged after mutation")
	}
	if _, ok := snapshot["attack"].Blocks["mutated"]; ok {
		t.Fatalf("expected cloned blocks to prevent external mutation")
	}
}

func TestResolverRejectsUnknownContract(t *testing.T) {
	reg := contract.Registry{}
	entry := mustMarshal([]map[string]any{{
		"id":         "missing",
		"contractId": "unknown",
		"definition": map[string]any{"typeId": "missing"},
	}})
	if _, err := NewResolver(reg, memorySource{path: "bad.json", data: entry}); err == nil {
		t.Fatalf("expected error for unknown contract")
	}
}

func TestDefaultPaths(t *testing.T) {
	paths := DefaultPaths()
	if len(paths) == 0 {
		t.Fatalf("expected default paths to include at least one candidate")
	}

	expected := map[string]bool{
		filepath.Join("config", "effects", "definitions.json"):       false,
		filepath.Join("..", "config", "effects", "definitions.json"): false,
	}

	for _, path := range paths {
		if filepath.Base(path) != "definitions.json" {
			t.Fatalf("unexpected default path %q", path)
		}
		if _, ok := expected[path]; ok {
			expected[path] = true
		}
	}

	if !expected[filepath.Join("config", "effects", "definitions.json")] {
		t.Fatalf("expected config/effects/definitions.json to be included in default paths")
	}
	if !expected[filepath.Join("..", "config", "effects", "definitions.json")] {
		t.Fatalf("expected ../config/effects/definitions.json to be included in default paths")
	}
}

func TestDefaultPathsResolveFromRepoRoot(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to determine caller path")
	}

	packageDir := filepath.Dir(file)
	repoRoot := filepath.Clean(filepath.Join(packageDir, "..", "..", ".."))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("failed to change directory to repo root: %v", err)
	}

	paths := DefaultPaths()
	var resolved bool
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			t.Fatalf("stat %q failed: %v", path, statErr)
		}
		if info.IsDir() {
			continue
		}
		resolved = true
		break
	}

	if !resolved {
		t.Fatalf("expected at least one default path to resolve from repo root; paths=%v", paths)
	}
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
