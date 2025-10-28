package effects

import (
	"testing"

	state "mine-and-die/server/internal/world/state"
)

func TestProjectileOwnerSnapshotFacingFallback(t *testing.T) {
	owner := ProjectileOwnerSnapshot{X: 10, Y: 20}

	if facing := owner.Facing(); facing != string(state.DefaultFacing) {
		t.Fatalf("expected facing fallback, got %q", facing)
	}

	vx, vy := owner.FacingVector()
	expectedX, expectedY := state.FacingToVector(state.DefaultFacing)
	if vx != expectedX || vy != expectedY {
		t.Fatalf("unexpected facing vector: got (%f,%f) want (%f,%f)", vx, vy, expectedX, expectedY)
	}

	x, y := owner.Position()
	if x != owner.X || y != owner.Y {
		t.Fatalf("unexpected position: got (%f,%f) want (%f,%f)", x, y, owner.X, owner.Y)
	}
}

func TestProjectileOwnerSnapshotFacingPreserved(t *testing.T) {
	owner := ProjectileOwnerSnapshot{X: 1, Y: 2, FacingValue: string(state.FacingLeft)}

	if facing := owner.Facing(); facing != string(state.FacingLeft) {
		t.Fatalf("expected facing to be preserved, got %q", facing)
	}

	vx, vy := owner.FacingVector()
	expectedX, expectedY := state.FacingToVector(state.FacingLeft)
	if vx != expectedX || vy != expectedY {
		t.Fatalf("unexpected facing vector: got (%f,%f) want (%f,%f)", vx, vy, expectedX, expectedY)
	}
}
