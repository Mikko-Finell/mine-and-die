package server

import (
	"encoding/json"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
)

type effectCatalogMetadata struct {
	ContractID      string
	Definition      *effectcontract.EffectDefinition
	Blocks          map[string]json.RawMessage
	ManagedByClient bool
}

func newEffectCatalogMetadata(entry effectcatalog.Entry) effectCatalogMetadata {
	managedByClient := false
	if entry.Definition != nil {
		managedByClient = entry.Definition.Client.ManagedByClient
	} else if entry.Contract != nil {
		managedByClient = entry.Contract.Owner == effectcontract.LifecycleOwnerClient
	}

	meta := effectCatalogMetadata{
		ContractID:      entry.ContractID,
		Blocks:          cloneRawMessageMap(entry.Blocks),
		ManagedByClient: managedByClient,
	}
	if entry.Definition != nil {
		defCopy := *entry.Definition
		meta.Definition = &defCopy
	}
	return meta
}

func (meta effectCatalogMetadata) clone() effectCatalogMetadata {
	cloned := effectCatalogMetadata{
		ContractID:      meta.ContractID,
		Blocks:          cloneRawMessageMap(meta.Blocks),
		ManagedByClient: meta.ManagedByClient,
	}
	if meta.Definition != nil {
		defCopy := *meta.Definition
		cloned.Definition = &defCopy
	}
	return cloned
}

func cloneEffectCatalogMetadataMap(src map[string]effectCatalogMetadata) map[string]effectCatalogMetadata {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]effectCatalogMetadata, len(src))
	for key, meta := range src {
		cloned[key] = meta.clone()
	}
	return cloned
}

func (meta effectCatalogMetadata) MarshalJSON() ([]byte, error) {
	payload := map[string]any{
		"contractId":      meta.ContractID,
		"managedByClient": meta.ManagedByClient,
	}
	if meta.Definition != nil {
		payload["definition"] = meta.Definition
	}
	blocks := make(map[string]json.RawMessage, len(meta.Blocks))
	for key, raw := range meta.Blocks {
		blocks[key] = cloneRawMessage(raw)
	}
	payload["blocks"] = blocks
	return json.Marshal(payload)
}

func cloneRawMessageMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		dst[key] = cloneRawMessage(value)
	}
	return dst
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}
