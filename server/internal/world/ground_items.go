package world

// SetGroundItemPosition updates the ground item's coordinates when they change.
// Returns true when the mutation was applied.
func SetGroundItemPosition(x, y *float64, newX, newY float64) bool {
	if x == nil || y == nil {
		return false
	}

	if PositionsEqual(*x, *y, newX, newY) {
		return false
	}

	*x = newX
	*y = newY
	return true
}

// SetGroundItemQuantity clamps the quantity to zero or greater and updates the
// stored value when it changes. Returns true when the mutation was applied.
func SetGroundItemQuantity(qty *int, newQty int) bool {
	if qty == nil {
		return false
	}

	if newQty < 0 {
		newQty = 0
	}

	if *qty == newQty {
		return false
	}

	*qty = newQty
	return true
}
