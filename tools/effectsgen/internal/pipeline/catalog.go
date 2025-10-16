package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type catalogEntry struct {
	ID         string
	ContractID string
	Definition json.RawMessage
	Blocks     map[string]json.RawMessage
}

func loadCatalogEntries(path string) ([]catalogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("effectsgen: definitions file %s not found", path)
		}
		return nil, fmt.Errorf("effectsgen: failed reading definitions %s: %w", path, err)
	}

	entries, err := decodeCatalog(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("effectsgen: failed parsing definitions %s: %w", path, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return entries, nil
}

func decodeCatalog(r io.Reader) ([]catalogEntry, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	switch trimmed[0] {
	case '[':
		var docs []catalogDocument
		if err := json.Unmarshal(trimmed, &docs); err != nil {
			return nil, err
		}
		return toCatalogEntries(docs)
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
		docs := make([]catalogDocument, 0, len(ids))
		for _, id := range ids {
			var doc catalogDocument
			if err := json.Unmarshal(object[id], &doc); err != nil {
				return nil, fmt.Errorf("entry %q: %w", id, err)
			}
			if doc.ID == "" {
				doc.ID = id
			} else if doc.ID != id {
				return nil, fmt.Errorf("entry id %q does not match key %q", doc.ID, id)
			}
			docs = append(docs, doc)
		}
		return toCatalogEntries(docs)
	default:
		return nil, fmt.Errorf("unexpected json token %q", string(trimmed[:1]))
	}
}

func toCatalogEntries(docs []catalogDocument) ([]catalogEntry, error) {
	entries := make([]catalogEntry, 0, len(docs))
	for _, doc := range docs {
		id := strings.TrimSpace(doc.ID)
		if id == "" {
			return nil, fmt.Errorf("catalog entry missing id")
		}
		contractID := strings.TrimSpace(doc.ContractID)
		if contractID == "" {
			return nil, fmt.Errorf("catalog entry %q missing contractId", id)
		}
		blocks := make(map[string]json.RawMessage, len(doc.Blocks))
		for key, value := range doc.Blocks {
			blocks[key] = cloneRaw(value)
		}
		entry := catalogEntry{
			ID:         id,
			ContractID: contractID,
			Definition: cloneRaw(doc.Definition),
			Blocks:     blocks,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}

type catalogDocument struct {
	ID         string                     `json:"id"`
	ContractID string                     `json:"contractId"`
	Definition json.RawMessage            `json:"definition"`
	Blocks     map[string]json.RawMessage `json:"-"`
}

func (d *catalogDocument) UnmarshalJSON(data []byte) error {
	type alias catalogDocument
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var blocks map[string]json.RawMessage
	if err := json.Unmarshal(data, &blocks); err != nil {
		return err
	}
	delete(blocks, "id")
	delete(blocks, "contractId")
	delete(blocks, "definition")
	decoded.Blocks = blocks
	*d = catalogDocument(decoded)
	return nil
}
