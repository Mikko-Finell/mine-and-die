package world

// MutateActorEquipment applies the provided mutation to the equipment, rolling
// back on error and triggering the supplied callbacks when the contents change.
// The clone function must return a deep copy suitable for comparison and
// rollback, and equal reports whether two snapshots are equivalent. When the
// mutation succeeds and produces a different equipment loadout, the version
// counter is incremented and the emit callback runs with the updated value.
func MutateActorEquipment[T any](
	equipment *T,
	version *uint64,
	mutate func(*T) error,
	clone func(T) T,
	equal func(T, T) bool,
	emit func(T),
) error {
	if equipment == nil || mutate == nil || clone == nil || equal == nil {
		return nil
	}

	before := clone(*equipment)

	if err := mutate(equipment); err != nil {
		*equipment = before
		return err
	}

	if equal(before, *equipment) {
		return nil
	}

	if version != nil {
		*version = *version + 1
	}

	if emit != nil {
		emit(*equipment)
	}

	return nil
}
