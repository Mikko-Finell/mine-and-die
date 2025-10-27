package effects

import (
	"math"
	"testing"

	runtime "mine-and-die/server/internal/effects/runtime"
)

func TestSetPositionUpdatesCoordinates(t *testing.T) {
	state := runtime.State{X: 1, Y: 2}
	callbackCalled := false

	changed := SetPosition(&state, 3, 4, func(oldX, oldY float64) bool {
		callbackCalled = true
		if oldX != 1 || oldY != 2 {
			t.Fatalf("expected old coordinates (1,2), got (%.2f, %.2f)", oldX, oldY)
		}
		if state.X != 3 || state.Y != 4 {
			t.Fatalf("expected coordinates updated before callback, got (%.2f, %.2f)", state.X, state.Y)
		}
		return true
	})

	if !changed {
		t.Fatalf("expected mutation to be applied")
	}
	if !callbackCalled {
		t.Fatalf("expected updateIndex callback to run")
	}
	if state.X != 3 || state.Y != 4 {
		t.Fatalf("expected coordinates (3,4), got (%.2f, %.2f)", state.X, state.Y)
	}
}

func TestSetPositionNoChangeSkipsCallback(t *testing.T) {
	state := runtime.State{X: 5, Y: 6}
	changed := SetPosition(&state, 5, 6, func(oldX, oldY float64) bool {
		t.Fatalf("expected callback not to run when position is unchanged")
		return true
	})
	if changed {
		t.Fatalf("expected mutation to be ignored for identical coordinates")
	}
}

func TestSetPositionRollbackOnFailure(t *testing.T) {
	state := runtime.State{X: 7, Y: 8}

	changed := SetPosition(&state, 9, 10, func(oldX, oldY float64) bool {
		if state.X != 9 || state.Y != 10 {
			t.Fatalf("expected coordinates updated before failure, got (%.2f, %.2f)", state.X, state.Y)
		}
		state.X = oldX
		state.Y = oldY
		return false
	})

	if changed {
		t.Fatalf("expected mutation to be rolled back on failure")
	}
	if state.X != 7 || state.Y != 8 {
		t.Fatalf("expected coordinates restored to (7,8), got (%.2f, %.2f)", state.X, state.Y)
	}
}

func TestSetParamInitializesMap(t *testing.T) {
	state := runtime.State{}
	changed := SetParam(&state, "speed", 1.5)
	if !changed {
		t.Fatalf("expected parameter mutation to be applied")
	}
	if state.Params["speed"] != 1.5 {
		t.Fatalf("expected parameter 'speed' to equal 1.5, got %.2f", state.Params["speed"])
	}
}

func TestSetParamSkipsWhenUnchanged(t *testing.T) {
	state := runtime.State{Params: map[string]float64{"lifetime": 5}}
	delta := EffectParamEpsilon / 2
	changed := SetParam(&state, "lifetime", 5+delta)
	if changed {
		t.Fatalf("expected parameter mutation to be ignored within epsilon")
	}
	if math.Abs(state.Params["lifetime"]-5) > 1e-9 {
		t.Fatalf("expected parameter to remain 5, got %.2f", state.Params["lifetime"])
	}
}

func TestSetParamRejectsEmptyKey(t *testing.T) {
	state := runtime.State{Params: map[string]float64{}}
	if SetParam(&state, "", 1) {
		t.Fatalf("expected empty key mutation to be ignored")
	}
}
