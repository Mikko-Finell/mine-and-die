package world

import "testing"

func TestApplyPlayerPositionMutationsCommitsMovedPlayers(t *testing.T) {
	players := []PositionCommit{{
		ID:      "player-1",
		Current: Vec2{X: 0, Y: 0},
	}, {
		ID:      "player-2",
		Current: Vec2{X: 5, Y: 5},
	}}
	initial := map[string]Vec2{
		"player-1": {X: 0, Y: 0},
		"player-2": {X: 5, Y: 5},
	}
	proposed := map[string]Vec2{
		"player-1": {X: 1, Y: 0},
	}

	var committed []Vec2
	ApplyPlayerPositionMutations(players, initial, proposed, func(id string, x, y float64) {
		if id != "player-1" {
			t.Fatalf("unexpected commit for %s", id)
		}
		committed = append(committed, Vec2{X: x, Y: y})
	})

	if len(committed) != 1 {
		t.Fatalf("expected one commit, got %d", len(committed))
	}
	if committed[0] != (Vec2{X: 1, Y: 0}) {
		t.Fatalf("unexpected commit payload: %+v", committed[0])
	}
}

func TestApplyPlayerPositionMutationsSkipsUnchanged(t *testing.T) {
	players := []PositionCommit{{
		ID:      "player-1",
		Current: Vec2{X: 2, Y: 2},
	}}
	initial := map[string]Vec2{"player-1": {X: 2, Y: 2}}
	proposed := map[string]Vec2{"player-1": {X: 2, Y: 2}}

	called := false
	ApplyPlayerPositionMutations(players, initial, proposed, func(string, float64, float64) {
		called = true
	})
	if called {
		t.Fatalf("expected commit to be skipped for unchanged position")
	}
}

func TestApplyNPCPositionMutationsUsesFallback(t *testing.T) {
	npcs := []PositionCommit{{
		ID:      "npc-1",
		Current: Vec2{X: 4, Y: 4},
	}}
	initial := map[string]Vec2{}
	proposed := map[string]Vec2{"npc-1": {X: 6, Y: 4}}

	var committed []Vec2
	ApplyNPCPositionMutations(npcs, initial, proposed, func(id string, x, y float64) {
		if id != "npc-1" {
			t.Fatalf("unexpected commit for %s", id)
		}
		committed = append(committed, Vec2{X: x, Y: y})
	})

	if len(committed) != 1 {
		t.Fatalf("expected one commit, got %d", len(committed))
	}
	if committed[0] != (Vec2{X: 6, Y: 4}) {
		t.Fatalf("unexpected commit payload: %+v", committed[0])
	}
}
