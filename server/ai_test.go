package main

import (
	"math"
	"testing"
	"time"
)

func newStaticAIWorld() (*World, *npcState) {
	w := &World{
		players:         make(map[string]*playerState),
		npcs:            make(map[string]*npcState),
		effects:         make([]*effectState, 0),
		effectBehaviors: newEffectBehaviors(),
		obstacles:       nil,
		aiLibrary:       globalAILibrary,
	}

	npc := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        "npc-test",
				X:         360,
				Y:         260,
				Facing:    defaultFacing,
				Health:    60,
				MaxHealth: 60,
				Inventory: NewInventory(),
			},
		},
		Type:             NPCTypeGoblin,
		ExperienceReward: 25,
		Waypoints: []vec2{
			{X: 360, Y: 260},
			{X: 480, Y: 260},
		},
	}

	if w.aiLibrary != nil {
		if cfg := w.aiLibrary.ConfigForType(NPCTypeGoblin); cfg != nil {
			npc.AIConfigID = cfg.id
			npc.AIState = cfg.initialState
			cfg.applyDefaults(&npc.Blackboard)
		}
	}
	if npc.Blackboard.ArriveRadius <= 0 {
		npc.Blackboard.ArriveRadius = 16
	}
	if npc.Blackboard.PauseTicks == 0 {
		npc.Blackboard.PauseTicks = 30
	}
	if npc.Blackboard.StuckEpsilon <= 0 {
		npc.Blackboard.StuckEpsilon = 0.5
	}
	npc.Blackboard.WaypointIndex = 0
	npc.Blackboard.NextDecisionAt = 0
	npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}

	w.npcs[npc.ID] = npc
	return w, npc
}

func TestGoblinPatrolsBetweenWaypoints(t *testing.T) {
	w, npc := newStaticAIWorld()
	if npc == nil {
		t.Fatalf("expected goblin NPC")
	}
	if len(npc.Waypoints) < 2 {
		t.Fatalf("expected at least two waypoints for patrol")
	}

	var waitStateID uint8 = 255
	if cfg := w.aiLibrary.ConfigByID(npc.AIConfigID); cfg != nil {
		for idx, name := range cfg.stateNames {
			if name == "Wait" {
				waitStateID = uint8(idx)
				break
			}
		}
	}
	if waitStateID == 255 {
		t.Fatalf("failed to locate wait state in config")
	}

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	reachedSecond := false
	returned := false

	for tick := uint64(1); tick <= 400; tick++ {
		w.Step(tick, now, dt, nil)
		now = now.Add(time.Second / tickRate)

		if npc.AIState == waitStateID {
			if tick > 20 && npc.Blackboard.WaypointIndex%len(npc.Waypoints) == 0 {
				reachedSecond = true
			}
			if reachedSecond && npc.Blackboard.WaypointIndex%len(npc.Waypoints) == 1 {
				returned = true
				break
			}
		}
	}

	if !reachedSecond {
		t.Fatalf("expected goblin to reach second waypoint")
	}
	if !returned {
		t.Fatalf("expected goblin to return to first waypoint after waiting")
	}
}

func TestAISimulationDeterminism(t *testing.T) {
	w1, npc1 := newStaticAIWorld()
	w2, npc2 := newStaticAIWorld()
	if npc1 == nil || npc2 == nil {
		t.Fatalf("expected goblins in both worlds")
	}

	dt := 1.0 / float64(tickRate)
	now1 := time.Unix(0, 0)
	now2 := time.Unix(0, 0)

	for step := 0; step < 180; step++ {
		tick := uint64(step + 1)
		w1.Step(tick, now1, dt, nil)
		w2.Step(tick, now2, dt, nil)
		now1 = now1.Add(time.Second / tickRate)
		now2 = now2.Add(time.Second / tickRate)

		if math.Abs(npc1.X-npc2.X) > 1e-6 || math.Abs(npc1.Y-npc2.Y) > 1e-6 {
			t.Fatalf("npc positions diverged at tick %d", tick)
		}
	}
}
