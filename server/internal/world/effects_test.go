package world

import (
	"math"
	"testing"
)

func TestSetEffectPositionUpdatesCoordinates(t *testing.T) {
	x := 1.0
	y := 2.0
	callbackCalled := false

	changed := SetEffectPosition(&x, &y, 3, 4, func(oldX, oldY float64) bool {
		callbackCalled = true
		if oldX != 1 || oldY != 2 {
			t.Fatalf("expected old coordinates (1,2), got (%.2f, %.2f)", oldX, oldY)
		}
		if x != 3 || y != 4 {
			t.Fatalf("expected coordinates updated before callback, got (%.2f, %.2f)", x, y)
		}
		return true
	})

	if !changed {
		t.Fatalf("expected mutation to be applied")
	}
	if !callbackCalled {
		t.Fatalf("expected updateIndex callback to run")
	}
	if x != 3 || y != 4 {
		t.Fatalf("expected coordinates (3,4), got (%.2f, %.2f)", x, y)
	}
}

func TestSetEffectPositionNoChangeSkipsCallback(t *testing.T) {
	x := 5.0
	y := 6.0
	changed := SetEffectPosition(&x, &y, 5, 6, func(oldX, oldY float64) bool {
		t.Fatalf("expected callback not to run when position is unchanged")
		return true
	})
	if changed {
		t.Fatalf("expected mutation to be ignored for identical coordinates")
	}
}

func TestSetEffectPositionRollbackOnFailure(t *testing.T) {
	x := 7.0
	y := 8.0

	changed := SetEffectPosition(&x, &y, 9, 10, func(oldX, oldY float64) bool {
		if x != 9 || y != 10 {
			t.Fatalf("expected coordinates updated before failure, got (%.2f, %.2f)", x, y)
		}
		x = oldX
		y = oldY
		return false
	})

	if changed {
		t.Fatalf("expected mutation to be rolled back on failure")
	}
	if x != 7 || y != 8 {
		t.Fatalf("expected coordinates restored to (7,8), got (%.2f, %.2f)", x, y)
	}
}

func TestSetEffectParamInitializesMap(t *testing.T) {
	var params map[string]float64
	changed := SetEffectParam(&params, "speed", 1.5)
	if !changed {
		t.Fatalf("expected parameter mutation to be applied")
	}
	if params["speed"] != 1.5 {
		t.Fatalf("expected parameter 'speed' to equal 1.5, got %.2f", params["speed"])
	}
}

func TestSetEffectParamSkipsWhenUnchanged(t *testing.T) {
	params := map[string]float64{"lifetime": 5}
	delta := EffectParamEpsilon / 2
	changed := SetEffectParam(&params, "lifetime", 5+delta)
	if changed {
		t.Fatalf("expected parameter mutation to be ignored within epsilon")
	}
	if math.Abs(params["lifetime"]-5) > 1e-9 {
		t.Fatalf("expected parameter to remain 5, got %.2f", params["lifetime"])
	}
}

func TestSetEffectParamRejectsEmptyKey(t *testing.T) {
	params := map[string]float64{}
	if SetEffectParam(&params, "", 1) {
		t.Fatalf("expected empty key mutation to be ignored")
	}
}
