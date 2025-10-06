package main

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func newRatTestWorld(seed int64) (*World, *npcState) {
	w := &World{
		players:         make(map[string]*playerState),
		npcs:            make(map[string]*npcState),
		effects:         make([]*effectState, 0),
		effectBehaviors: newEffectBehaviors(),
		obstacles:       nil,
		aiLibrary:       globalAILibrary,
		rng:             rand.New(rand.NewSource(seed)),
	}

	rat := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        "rat-test",
				X:         320,
				Y:         320,
				Facing:    defaultFacing,
				Health:    24,
				MaxHealth: 24,
				Inventory: NewInventory(),
			},
		},
		Type:             NPCTypeRat,
		ExperienceReward: 5,
	}
	rat.wanderOrigin = vec2{X: rat.X, Y: rat.Y}
	rat.wanderTarget = rat.wanderOrigin
	rat.Blackboard.NextDecisionAt = 0
	rat.Blackboard.LastWaypointIndex = -1
	rat.Blackboard.LastPos = vec2{X: rat.X, Y: rat.Y}

	w.npcs[rat.ID] = rat
	return w, rat
}

func TestRatWandersWhenUndisturbed(t *testing.T) {
	w, rat := newRatTestWorld(7)
	startX, startY := rat.X, rat.Y
	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)

	moved := false
	for tick := uint64(1); tick <= 240; tick++ {
		w.Step(tick, now, dt, nil)
		now = now.Add(time.Second / tickRate)
		if math.Hypot(rat.X-startX, rat.Y-startY) > 2 {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatalf("expected wandering rat to leave starting position; final offset=%.2f", math.Hypot(rat.X-startX, rat.Y-startY))
	}
}

func TestRatFleesFromNearbyActor(t *testing.T) {
	w, rat := newRatTestWorld(11)
	player := &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        "player-threat",
				X:         rat.X + 40,
				Y:         rat.Y,
				Facing:    defaultFacing,
				Health:    playerMaxHealth,
				MaxHealth: playerMaxHealth,
				Inventory: NewInventory(),
			},
		},
		lastHeartbeat: time.Now(),
	}
	w.players[player.ID] = player

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	startDist := math.Hypot(rat.X-player.X, rat.Y-player.Y)
	escaped := false
	for tick := uint64(1); tick <= 90; tick++ {
		w.Step(tick, now, dt, nil)
		now = now.Add(time.Second / tickRate)
		dist := math.Hypot(rat.X-player.X, rat.Y-player.Y)
		if dist > startDist+5 {
			escaped = true
			break
		}
	}
	if !escaped {
		t.Fatalf("expected rat to flee from nearby player; start=%.2f dist=%.2f", startDist, math.Hypot(rat.X-player.X, rat.Y-player.Y))
	}

	delete(w.players, player.ID)

	goblinWorld, rat2 := newRatTestWorld(19)
	goblin := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        "goblin-threat",
				X:         rat2.X + 48,
				Y:         rat2.Y,
				Facing:    defaultFacing,
				Health:    60,
				MaxHealth: 60,
				Inventory: NewInventory(),
			},
		},
		Type:             NPCTypeGoblin,
		ExperienceReward: 25,
	}
	goblin.Blackboard.LastPos = vec2{X: goblin.X, Y: goblin.Y}
	goblinWorld.npcs[goblin.ID] = goblin

	dt = 1.0 / float64(tickRate)
	now = time.Unix(0, 0)
	startDist = math.Hypot(rat2.X-goblin.X, rat2.Y-goblin.Y)
	escaped = false
	for tick := uint64(1); tick <= 90; tick++ {
		goblinWorld.Step(tick, now, dt, nil)
		now = now.Add(time.Second / tickRate)
		dist := math.Hypot(rat2.X-goblin.X, rat2.Y-goblin.Y)
		if dist > startDist+5 {
			escaped = true
			break
		}
	}
	if !escaped {
		t.Fatalf("expected rat to flee from nearby non-rat NPC; start=%.2f dist=%.2f", startDist, math.Hypot(rat2.X-goblin.X, rat2.Y-goblin.Y))
	}
}
