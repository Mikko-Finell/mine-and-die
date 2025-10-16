package main

import (
	"encoding/json"

	effectcatalog "mine-and-die/server/effects/catalog"
)

type effectCatalogMetadata struct {
	ContractID string                     `json:"contractId"`
	Blocks     map[string]json.RawMessage `json:"blocks,omitempty"`
}

func snapshotEffectCatalog(resolver *effectcatalog.Resolver) map[string]effectCatalogMetadata {
	if resolver == nil {
		return nil
	}
	entries := resolver.Entries()
	if len(entries) == 0 {
		return nil
	}
	snapshot := make(map[string]effectCatalogMetadata, len(entries))
	for id, entry := range entries {
		if id == "" {
			continue
		}
		meta := effectCatalogMetadata{ContractID: entry.ContractID}
		if len(entry.Blocks) > 0 {
			blocks := make(map[string]json.RawMessage, len(entry.Blocks))
			for key, value := range entry.Blocks {
				if len(value) == 0 {
					blocks[key] = nil
					continue
				}
				copied := make(json.RawMessage, len(value))
				copy(copied, value)
				blocks[key] = copied
			}
			meta.Blocks = blocks
		}
		snapshot[id] = meta
	}
	if len(snapshot) == 0 {
		return nil
	}
	return snapshot
}
