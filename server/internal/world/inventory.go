package world

// MutateActorInventory applies the provided mutation to the inventory, rolling
// back on error and triggering the supplied callbacks when the contents change.
// The clone function must return a deep copy suitable for comparison and
// rollback, and equal reports whether two snapshots are equivalent. When the
// mutation succeeds and produces a different inventory, the version counter is
// incremented and the emit callback runs with the updated value.
func MutateActorInventory[T any](
	inventory *T,
	version *uint64,
	mutate func(*T) error,
	clone func(T) T,
	equal func(T, T) bool,
	emit func(T),
) error {
	if inventory == nil || mutate == nil || clone == nil || equal == nil {
		return nil
	}

	before := clone(*inventory)

	if err := mutate(inventory); err != nil {
		*inventory = before
		return err
	}

	if equal(before, *inventory) {
		return nil
	}

	if version != nil {
		*version = *version + 1
	}

	if emit != nil {
		emit(*inventory)
	}

	return nil
}
