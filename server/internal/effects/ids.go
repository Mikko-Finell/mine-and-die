package effects

// AliveEffectIDsFromStates returns the identifiers for the provided effect
// states, filtering out nil entries or empty IDs. The returned slice is a new
// allocation so callers may mutate it safely.
func AliveEffectIDsFromStates(effects []*State) []string {
	if len(effects) == 0 {
		return nil
	}
	ids := make([]string, 0, len(effects))
	for _, eff := range effects {
		if eff == nil || eff.ID == "" {
			continue
		}
		ids = append(ids, eff.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	cloned := make([]string, len(ids))
	copy(cloned, ids)
	return cloned
}

// CloneAliveEffectIDs returns a deep copy of the provided effect identifier
// slice, dropping any empty IDs so callers receive only valid entries.
func CloneAliveEffectIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		filtered = append(filtered, id)
	}
	if len(filtered) == 0 {
		return nil
	}
	return cloneStringSlice(filtered)
}
