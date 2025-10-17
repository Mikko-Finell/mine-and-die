package main

import (
	"encoding/json"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
)

type effectCatalogMetadata struct {
	ContractID string
	Definition *effectcontract.EffectDefinition
	Blocks     map[string]json.RawMessage
}

func newEffectCatalogMetadata(entry effectcatalog.Entry) effectCatalogMetadata {
	meta := effectCatalogMetadata{
		ContractID: entry.ContractID,
		Blocks:     cloneRawMessageMap(entry.Blocks),
	}
	if entry.Definition != nil {
		defCopy := *entry.Definition
		meta.Definition = &defCopy
	}
	return meta
}

func (meta effectCatalogMetadata) clone() effectCatalogMetadata {
	cloned := effectCatalogMetadata{
		ContractID: meta.ContractID,
		Blocks:     cloneRawMessageMap(meta.Blocks),
	}
	if meta.Definition != nil {
		defCopy := *meta.Definition
		cloned.Definition = &defCopy
	}
	return cloned
}

func (meta effectCatalogMetadata) MarshalJSON() ([]byte, error) {
	payload := make(map[string]any, len(meta.Blocks)+2)
	payload["contractId"] = meta.ContractID
	if meta.Definition != nil {
		payload["definition"] = meta.Definition
	}
	for key, raw := range meta.Blocks {
		payload[key] = cloneRawMessage(raw)
	}
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
