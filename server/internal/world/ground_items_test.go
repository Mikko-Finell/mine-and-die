package world

import "testing"

func TestSetGroundItemPositionUpdatesCoordinates(t *testing.T) {
	x := 1.0
	y := 2.0

	if !SetGroundItemPosition(&x, &y, 3, 4) {
		t.Fatalf("expected position mutation to be applied")
	}

	if x != 3 || y != 4 {
		t.Fatalf("expected coordinates (3,4), got (%.2f, %.2f)", x, y)
	}
}

func TestSetGroundItemPositionSkipsWhenUnchanged(t *testing.T) {
	x := 5.0
	y := 6.0

	if SetGroundItemPosition(&x, &y, 5, 6) {
		t.Fatalf("expected mutation to be ignored when coordinates match")
	}
}

func TestSetGroundItemPositionNilPointers(t *testing.T) {
	if SetGroundItemPosition(nil, nil, 1, 2) {
		t.Fatalf("expected mutation to fail for nil coordinates")
	}
}

func TestSetGroundItemQuantityUpdatesValue(t *testing.T) {
	qty := 3

	if !SetGroundItemQuantity(&qty, 5) {
		t.Fatalf("expected quantity mutation to be applied")
	}

	if qty != 5 {
		t.Fatalf("expected quantity to equal 5, got %d", qty)
	}
}

func TestSetGroundItemQuantityClampsNegative(t *testing.T) {
	qty := 7

	if !SetGroundItemQuantity(&qty, -4) {
		t.Fatalf("expected mutation to be applied when clamping negative values")
	}

	if qty != 0 {
		t.Fatalf("expected quantity to clamp to 0, got %d", qty)
	}
}

func TestSetGroundItemQuantitySkipsWhenUnchanged(t *testing.T) {
	qty := 9

	if SetGroundItemQuantity(&qty, 9) {
		t.Fatalf("expected mutation to be ignored when quantity is unchanged")
	}
}

func TestSetGroundItemQuantityNilPointer(t *testing.T) {
	if SetGroundItemQuantity(nil, 1) {
		t.Fatalf("expected mutation to fail for nil quantity pointer")
	}
}
