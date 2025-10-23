package items

// CloneGroundItems returns a deep copy of the provided ground item slice.
func CloneGroundItems(items []GroundItem) []GroundItem {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]GroundItem, len(items))
	copy(cloned, items)
	return cloned
}
