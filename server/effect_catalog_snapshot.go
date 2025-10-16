package main

import effectcatalog "mine-and-die/server/effects/catalog"

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
		meta.Blocks = cloneRawMessageMap(entry.Blocks)
		snapshot[id] = meta
	}
	if len(snapshot) == 0 {
		return nil
	}
	return snapshot
}
