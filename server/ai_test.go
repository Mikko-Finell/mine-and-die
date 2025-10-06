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
	if npc.Blackboard.BaseArriveRadius <= 0 {
		npc.Blackboard.BaseArriveRadius = npc.Blackboard.ArriveRadius
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
	npc.Blackboard.LastWaypointIndex = -1
	npc.Blackboard.WaypointArrivedIndex = -1

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

func TestGoblinAdvancesWhenWaypointBlocked(t *testing.T) {
	w, npc := newStaticAIWorld()
	if npc == nil {
		t.Fatalf("expected goblin NPC")
	}
	if len(npc.Waypoints) < 2 {
		t.Fatalf("expected patrol with at least two waypoints")
	}

	second := npc.Waypoints[1]
	w.obstacles = append(w.obstacles, Obstacle{
		ID:     "blocker",
		X:      second.X - 30,
		Y:      second.Y - 30,
		Width:  60,
		Height: 60,
	})

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)

	// Allow the goblin to advance to the second waypoint index before blocking behaviour kicks in.
	for tick := uint64(1); tick <= 40; tick++ {
		w.Step(tick, now, dt, nil)
		now = now.Add(time.Second / tickRate)
	}

	blockedIndex := npc.Blackboard.WaypointIndex
	if blockedIndex != 1 {
		t.Fatalf("expected goblin to target second waypoint, got %d", blockedIndex)
	}

	advanced := false
	for tick := uint64(41); tick <= 600; tick++ {
		w.Step(tick, now, dt, nil)
		now = now.Add(time.Second / tickRate)
		if npc.Blackboard.WaypointIndex != blockedIndex {
			advanced = true
			break
		}
	}

	if !advanced {
		t.Fatalf("expected goblin to advance past blocked waypoint; stall counter=%d best=%.2f dist=%.2f",
			npc.Blackboard.WaypointStall, npc.Blackboard.WaypointBestDist, npc.Blackboard.WaypointLastDist)
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

func TestReachedWaypointArrivalConditions(t *testing.T) {
	w, npc := newStaticAIWorld()
	if npc == nil {
		t.Fatalf("expected goblin NPC")
	}
	cfg := w.aiLibrary.ConfigByID(npc.AIConfigID)
	if cfg == nil {
		t.Fatalf("expected compiled AI config")
	}
	if len(npc.Waypoints) == 0 {
		t.Fatalf("expected at least one waypoint")
	}
	transition := aiCompiledTransition{conditionID: aiConditionReachedWaypoint}
	if len(cfg.reachedWaypointParams) > 0 {
		transition.paramIndex = 0
	}

	waypoint := npc.Waypoints[0]
	npc.Blackboard.WaypointIndex = 0
	npc.Blackboard.LastWaypointIndex = 0
	npc.X = waypoint.X + npc.Blackboard.ArriveRadius - 1
	npc.Y = waypoint.Y
	npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}
	npc.Blackboard.LastMoveDelta = moveSpeed / float64(tickRate)
	npc.intentX = -1
	npc.intentY = 0
	npc.Facing = FacingLeft

	if w.evaluateCondition(cfg, npc, &transition, 1, time.Unix(0, 0)) {
		t.Fatalf("expected arrival check to fail while moving quickly")
	}

	npc.Blackboard.LastMoveDelta = 0.05
	npc.intentX = 0
	npc.intentY = 0

	if !w.evaluateCondition(cfg, npc, &transition, 2, time.Unix(0, 0)) {
		t.Fatalf("expected arrival check to pass when slow and close")
	}
	if !npc.Blackboard.WaypointArrived {
		t.Fatalf("expected waypoint to be marked as arrived")
	}

	npc.X = waypoint.X + npc.Blackboard.ArriveRadius*1.4
	npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}
	npc.Blackboard.LastMoveDelta = 0

	if !w.evaluateCondition(cfg, npc, &transition, 3, time.Unix(0, 0)) {
		t.Fatalf("expected arrival to persist within hysteresis window")
	}

	npc.X = waypoint.X + npc.Blackboard.ArriveRadius*1.6
	npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}
	npc.Blackboard.LastMoveDelta = 0

	if w.evaluateCondition(cfg, npc, &transition, 4, time.Unix(0, 0)) {
		t.Fatalf("expected arrival to clear once outside hysteresis window")
	}
	if npc.Blackboard.WaypointArrived {
		t.Fatalf("expected waypoint arrival state to clear after leaving radius")
	}
}
