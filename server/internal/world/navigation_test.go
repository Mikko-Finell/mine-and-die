package world

import (
	"reflect"
	"testing"
)

func TestDynamicBlockerPositions(t *testing.T) {
	actors := []PathActor{
		{ID: "keep", Position: Vec2{X: 10, Y: 20}},
		{ID: "drop", Position: Vec2{X: 30, Y: 40}},
	}

	positions := DynamicBlockerPositions(actors, "drop")
	if len(positions) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(positions))
	}
	if positions[0] != actors[0].Position {
		t.Fatalf("expected blocker %+v, got %+v", actors[0].Position, positions[0])
	}

	if out := DynamicBlockerPositions(nil, "drop"); out != nil {
		t.Fatalf("expected nil slice for empty input, got %v", out)
	}

	if out := DynamicBlockerPositions(actors[:1], "keep"); out != nil {
		t.Fatalf("expected nil after filtering, got %v", out)
	}
}

func TestConvertPathClones(t *testing.T) {
	original := []Vec2{{X: 1, Y: 2}, {X: 3, Y: 4}}
	cloned := ConvertPath(original)

	if len(cloned) != len(original) {
		t.Fatalf("expected clone length %d, got %d", len(original), len(cloned))
	}
	original[0].X = 99
	if cloned[0].X == original[0].X {
		t.Fatalf("clone shares storage with original: %+v vs %+v", cloned[0], original[0])
	}
}

func TestComputeNavigationPathMatchesManual(t *testing.T) {
	start := Vec2{X: 128, Y: 128}
	target := Vec2{X: 256, Y: 128}
	actors := []PathActor{
		{ID: "blocker", Position: Vec2{X: 192, Y: 128}},
	}
	base := ComputePathRequest{
		Start:     start,
		Target:    target,
		Width:     DefaultWidth,
		Height:    DefaultHeight,
		Obstacles: nil,
	}

	for _, tc := range []struct {
		name     string
		ignoreID string
	}{
		{name: "with-blocker", ignoreID: ""},
		{name: "ignored", ignoreID: "blocker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manual := base
			manual.Blockers = DynamicBlockerPositions(actors, tc.ignoreID)
			manualPath, manualGoal, manualOK := ComputePathFrom(manual)

			path, goal, ok := ComputeNavigationPath(base, actors, tc.ignoreID)

			if manualOK != ok {
				t.Fatalf("ok mismatch: manual=%v computed=%v", manualOK, ok)
			}
			if manualGoal != goal {
				t.Fatalf("goal mismatch: manual=%+v computed=%+v", manualGoal, goal)
			}
			if !reflect.DeepEqual(ConvertPath(manualPath), path) {
				t.Fatalf("path mismatch: manual=%+v computed=%+v", manualPath, path)
			}
		})
	}
}
