package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"mine-and-die/server/effects/contract"
)

type source interface {
	Load() ([]byte, error)
	Path() string
}

type fileSource struct {
	path string
}

func (f fileSource) Load() ([]byte, error) {
	return os.ReadFile(f.path)
}

func (f fileSource) Path() string {
	return f.path
}

// Entry captures the resolved catalog data for a single designer-authored effect.
// It exposes the referenced contract ID, the parsed EffectDefinition, and any
// additional JSON blocks that were present on disk.
type Entry struct {
	ID         string
	ContractID string
	Contract   *contract.Definition
	Definition *contract.EffectDefinition
	Blocks     map[string]json.RawMessage
}

// EntryDocument represents a single catalog entry as it appears on disk. The
// struct is exported so tooling (e.g. schema generators) can reflect over the
// configuration contract shared with designers.
type EntryDocument struct {
	ID         string                     `json:"id" jsonschema:"title=Catalog Entry ID,description=Designer-facing identifier that maps to gameplay intents.,pattern=^[a-z0-9-]+$,minLength=1,required"`
	ContractID string                     `json:"contractId" jsonschema:"title=Contract ID,description=Identifier from the Go contract registry this entry references.,pattern=^[a-z0-9-]+$,minLength=1,required"`
	Definition contract.EffectDefinition  `json:"definition" jsonschema:"title=Effect Definition,description=Canonical gameplay configuration resolved by the runtime.,required"`
	Blocks     map[string]json.RawMessage `json:"-" jsonschema:"-"`
}

func (e Entry) clone() Entry {
	clone := Entry{
		ID:         e.ID,
		ContractID: e.ContractID,
		Blocks:     cloneRawMap(e.Blocks),
	}
	if e.Contract != nil {
		contractCopy := *e.Contract
		clone.Contract = &contractCopy
	}
	if e.Definition != nil {
		defCopy := *e.Definition
		clone.Definition = &defCopy
	}
	return clone
}

func cloneRawMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		if len(value) == 0 {
			dst[key] = nil
			continue
		}
		copied := make(json.RawMessage, len(value))
		copy(copied, value)
		dst[key] = copied
	}
	return dst
}

// Resolver merges one or more catalog sources into a stable lookup table.
// Call Reload to pick up on-disk changes (used for dev hot reload).
type Resolver struct {
	mu       sync.RWMutex
	sources  []source
	registry map[string]contract.Definition
	entries  map[string]Entry
}

// DefaultPaths returns the canonical catalog locations relative to the server
// module root. Callers may pass these to Load.
func DefaultPaths() []string {
	candidates := []string{
		filepath.Join("config", "effects", "definitions.json"),
		filepath.Join("..", "config", "effects", "definitions.json"),
	}

	paths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if _, duplicate := seen[cleaned]; duplicate {
			continue
		}
		seen[cleaned] = struct{}{}
		paths = append(paths, cleaned)
	}

	if len(paths) == 0 {
		return []string{filepath.Join("config", "effects", "definitions.json")}
	}
	return paths
}

// Load constructs a Resolver backed by the provided contract registry and
// catalog file paths.
func Load(reg contract.Registry, paths ...string) (*Resolver, error) {
	sources := make([]source, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		sources = append(sources, fileSource{path: trimmed})
	}
	return NewResolver(reg, sources...)
}

// NewResolver constructs a Resolver from arbitrary sources. Tests can supply
// in-memory sources while production code uses fileSource.
func NewResolver(reg contract.Registry, sources ...source) (*Resolver, error) {
	index, err := reg.Index()
	if err != nil {
		return nil, fmt.Errorf("catalog: invalid registry: %w", err)
	}
	r := &Resolver{
		sources:  append([]source(nil), sources...),
		registry: index,
		entries:  make(map[string]Entry),
	}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// Reload re-parses all catalog sources. Later sources override earlier ones to
// support local overlays during development.
func (r *Resolver) Reload() error {
	if r == nil {
		return nil
	}
	entries := make(map[string]Entry)
	for _, src := range r.sources {
		data, err := src.Load()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("catalog: failed loading %s: %w", src.Path(), err)
		}
		documents, err := decodeEntries(data)
		if err != nil {
			return fmt.Errorf("catalog: failed parsing %s: %w", src.Path(), err)
		}
		seen := make(map[string]struct{}, len(documents))
		for _, fe := range documents {
			id := strings.TrimSpace(fe.ID)
			if id == "" {
				return fmt.Errorf("catalog: entry missing id in %s", src.Path())
			}
			if _, dup := seen[id]; dup {
				return fmt.Errorf("catalog: duplicate id %q in %s", id, src.Path())
			}
			seen[id] = struct{}{}

			contractID := strings.TrimSpace(fe.ContractID)
			if contractID == "" {
				return fmt.Errorf("catalog: entry %q missing contractId", id)
			}
			contractDef, ok := r.registry[contractID]
			if !ok {
				return fmt.Errorf("catalog: entry %q references unknown contractId %q", id, contractID)
			}

			def := fe.Definition
			if strings.TrimSpace(def.TypeID) == "" {
				def.TypeID = contractID
			}

			owner := contractDef.Owner
			if owner != contract.LifecycleOwnerClient && !def.Client.SendUpdates && !def.Client.SendEnd && def.LifetimeTicks == 1 {
				owner = contract.LifecycleOwnerClient
			}

			if def.Client.ManagedByClient {
				return fmt.Errorf("catalog: entry %q must not set definition.client.managedByClient; ownership is derived from the contract registry", id)
			}

			if owner == contract.LifecycleOwnerClient {
				if def.Client.SendUpdates {
					return fmt.Errorf("catalog: entry %q enables sendUpdates but the contract is client-owned", id)
				}
				if def.Client.SendEnd {
					return fmt.Errorf("catalog: entry %q enables sendEnd but the contract is client-owned", id)
				}
				if def.LifetimeTicks != 1 {
					return fmt.Errorf("catalog: entry %q sets lifetimeTicks to %d but client-owned contracts must use 1", id, def.LifetimeTicks)
				}
				def.Client.SendUpdates = false
				def.Client.SendEnd = false
				def.LifetimeTicks = 1
				def.Client.ManagedByClient = true
			} else {
				def.Client.ManagedByClient = false
			}

			entry := Entry{
				ID:         id,
				ContractID: contractID,
				Blocks:     fe.Blocks,
			}
			contractCopy := contractDef
			contractCopy.Owner = owner
			entry.Contract = &contractCopy
			defCopy := def
			entry.Definition = &defCopy

			entries[id] = entry
		}
	}

	r.mu.Lock()
	r.entries = entries
	r.mu.Unlock()
	return nil
}

// Resolve returns the catalog entry for the provided designer ID.
func (r *Resolver) Resolve(id string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[id]
	if !ok {
		return Entry{}, false
	}
	return entry.clone(), true
}

// ContractID returns the contract identifier associated with a catalog entry.
func (r *Resolver) ContractID(id string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[id]
	if !ok {
		return "", false
	}
	return entry.ContractID, true
}

// DefinitionsByContractID returns the effect definitions keyed by contract ID.
func (r *Resolver) DefinitionsByContractID() map[string]*contract.EffectDefinition {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make(map[string]*contract.EffectDefinition, len(r.entries))
	for _, entry := range r.entries {
		if entry.Definition == nil {
			continue
		}
		defCopy := *entry.Definition
		defs[entry.ContractID] = &defCopy
	}
	return defs
}

// Entries returns a cloned snapshot of the catalog entries keyed by designer ID.
func (r *Resolver) Entries() map[string]Entry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Entry, len(r.entries))
	for id, entry := range r.entries {
		out[id] = entry.clone()
	}
	return out
}

func (e *EntryDocument) UnmarshalJSON(data []byte) error {
	type rawEntry EntryDocument
	var alias rawEntry
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	var blocks map[string]json.RawMessage
	if err := json.Unmarshal(data, &blocks); err != nil {
		return err
	}
	delete(blocks, "id")
	delete(blocks, "contractId")
	delete(blocks, "definition")
	alias.Blocks = blocks
	*e = EntryDocument(alias)
	return nil
}

func decodeEntries(data []byte) ([]EntryDocument, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	switch trimmed[0] {
	case '[':
		var entries []EntryDocument
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, err
		}
		return entries, nil
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(object))
		for id := range object {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		entries := make([]EntryDocument, 0, len(ids))
		for _, id := range ids {
			var entry EntryDocument
			if err := json.Unmarshal(object[id], &entry); err != nil {
				return nil, fmt.Errorf("entry %q: %w", id, err)
			}
			if entry.ID == "" {
				entry.ID = id
			} else if entry.ID != id {
				return nil, fmt.Errorf("entry id %q does not match key %q", entry.ID, id)
			}
			entries = append(entries, entry)
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("unexpected json token %q", string(trimmed[:1]))
	}
}
