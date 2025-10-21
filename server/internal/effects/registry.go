package effects

import "time"

// Registry coordinates effect registration bookkeeping across the shared
// slice, ID map, and spatial index used by legacy callers. The struct allows
// the legacy world to delegate registration semantics to this package while
// keeping storage ownership external.
type Registry struct {
	// Effects stores the ordered effect instances. The pointer allows the
	// helper to append without owning the underlying slice.
	Effects *[]*State
	// ByID provides random access to effects by identifier. The pointer
	// allows lazy allocation when registration occurs.
	ByID *map[string]*State
	// Index maintains spatial occupancy information for collision queries.
	Index *SpatialIndex
	// RecordSpatialOverflow reports when the spatial index rejects an
	// effect because a cell would exceed capacity.
	RecordSpatialOverflow func(effectType string)
}

// RegisterEffect appends the provided effect to the registry slice, indexes it
// by ID, and inserts it into the spatial index. It returns false when the
// effect is invalid or when the spatial index rejects the update.
func RegisterEffect(reg Registry, effect *State) bool {
	if effect == nil || effect.ID == "" {
		return false
	}
	if reg.Effects == nil || reg.ByID == nil {
		return false
	}
	if reg.Index != nil {
		if !reg.Index.Upsert(effect) {
			if reg.RecordSpatialOverflow != nil {
				reg.RecordSpatialOverflow(effect.Type)
			}
			return false
		}
	}

	if *reg.ByID == nil {
		*reg.ByID = make(map[string]*State)
	}
	*reg.Effects = append(*reg.Effects, effect)
	(*reg.ByID)[effect.ID] = effect
	return true
}

// UnregisterEffect removes the provided effect from the registry's lookup
// structures and spatial index. It ignores nil arguments and empty IDs.
func UnregisterEffect(reg Registry, effect *State) {
	if effect == nil || effect.ID == "" {
		return
	}
	if reg.Index != nil {
		reg.Index.Remove(effect.ID)
	}
	if reg.ByID != nil && *reg.ByID != nil {
		delete(*reg.ByID, effect.ID)
	}
}

// FindByID returns the registered effect with the provided identifier.
// It returns nil when the registry lacks a map, the ID is empty, or the effect
// is not present.
func FindByID(reg Registry, id string) *State {
	if id == "" || reg.ByID == nil || *reg.ByID == nil {
		return nil
	}
	return (*reg.ByID)[id]
}

// PruneExpired removes effects whose `ExpiresAt` timestamp is not after the
// provided clock reading. It compacts the registry slice in-place to preserve
// existing allocation backing while returning the expired entries to callers so
// they can perform domain-specific cleanup.
func PruneExpired(reg Registry, now time.Time) []*State {
	if reg.Effects == nil || reg.ByID == nil {
		return nil
	}
	effects := *reg.Effects
	if len(effects) == 0 {
		return nil
	}

	kept := effects[:0]
	var expired []*State
	for _, eff := range effects {
		if now.Before(eff.ExpiresAt) {
			kept = append(kept, eff)
			continue
		}
		expired = append(expired, eff)
		UnregisterEffect(reg, eff)
	}
	*reg.Effects = kept
	return expired
}
