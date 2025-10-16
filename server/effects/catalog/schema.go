package catalog

import "mine-and-die/server/effects/contract"

// EntryDefinition models the JSON contract for designer-authored catalog entries.
// It is shared with the schema generator so we can produce a machine-readable
// document for validation and editor tooling.
type EntryDefinition struct {
	ID         string                    `json:"id" jsonschema:"title=Catalog entry id,pattern=^[a-z0-9\-]+$,description=Designer facing identifier for the entry"`
	ContractID string                    `json:"contractId" jsonschema:"title=Contract id,pattern=^[a-z0-9\-]+$,description=Registered contract identifier referenced by this entry"`
	Definition contract.EffectDefinition `json:"definition" jsonschema:"description=Authoritative contract definition resolved at runtime"`
	JSEffect   string                    `json:"jsEffect,omitempty" jsonschema:"description=Path to the client visual implementation"`
	Parameters map[string]int            `json:"parameters,omitempty" jsonschema:"description=Designer tunables forwarded to hooks and visuals"`
}

// FileDefinitions represents the contents of config/effects/definitions.json.
// The catalog loader accepts either arrays or objects; the schema models the
// canonical array format authored by designers.
type FileDefinitions []EntryDefinition
