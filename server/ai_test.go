package main

import (
	"math"
	"testing"
	"time"

	stats "mine-and-die/server/stats"
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
	cfg := fullyFeaturedTestWorldConfig()
	cfg.Seed = "ai-test-static"
	cfg = cfg.normalized()
	w.config = cfg
	w.seed = cfg.Seed
	w.rng = newDeterministicRNG(w.seed, "world")

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
		stats:            stats.DefaultComponent(stats.ArchetypeGoblin),
		Type:             NPCTypeGoblin,
		ExperienceReward: 25,
		Waypoints: []vec2{
			{X: 360, Y: 260},
			{X: 480, Y: 260},
			{X: 480, Y: 380},
			{X: 360, Y: 380},
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
	npc.Blackboard.LastWaypointIndex = -1

	w.npcs[npc.ID] = npc
	return w, npc
}

func goblinStateID(t *testing.T, w *World, name string) uint8 {
	t.Helper()
	if w == nil || w.aiLibrary == nil {
		t.Fatalf("ai library not initialised")
	}
	cfg := w.aiLibrary.ConfigForType(NPCTypeGoblin)
	if cfg == nil {
		t.Fatalf("missing goblin ai config")
	}
	for idx, state := range cfg.stateNames {
		if state == name {
			return uint8(idx)
		}
	}
	t.Fatalf("failed to locate %s state in goblin config", name)
	return 0
}

func newRatTestWorld() (*World, *npcState) {
	w := &World{
		players:         make(map[string]*playerState),
		npcs:            make(map[string]*npcState),
		effects:         make([]*effectState, 0),
		effectBehaviors: newEffectBehaviors(),
		obstacles:       nil,
		aiLibrary:       globalAILibrary,
	}
	cfg := fullyFeaturedTestWorldConfig()
	cfg.Seed = "ai-test-rat"
	cfg = cfg.normalized()
	w.config = cfg
	w.seed = cfg.Seed
	w.rng = newDeterministicRNG(w.seed, "world")

	rat := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        "npc-rat-test",
				X:         420,
				Y:         360,
				Facing:    defaultFacing,
				Health:    18,
				MaxHealth: 18,
				Inventory: NewInventory(),
			},
		},
		stats:            stats.DefaultComponent(stats.ArchetypeRat),
		Type:             NPCTypeRat,
		ExperienceReward: 8,
		Home:             vec2{X: 420, Y: 360},
	}

	if w.aiLibrary != nil {
		if cfg := w.aiLibrary.ConfigForType(NPCTypeRat); cfg != nil {
			rat.AIConfigID = cfg.id
			rat.AIState = cfg.initialState
			cfg.applyDefaults(&rat.Blackboard)
		}
	}
	if rat.Blackboard.ArriveRadius <= 0 {
		rat.Blackboard.ArriveRadius = 10
	}
	if rat.Blackboard.PauseTicks == 0 {
		rat.Blackboard.PauseTicks = 20
	}
	if rat.Blackboard.StuckEpsilon <= 0 {
		rat.Blackboard.StuckEpsilon = 0.4
	}
	rat.Blackboard.WaypointIndex = 0
	rat.Blackboard.NextDecisionAt = 0
	rat.Blackboard.LastWaypointIndex = -1
	rat.Blackboard.LastPos = vec2{X: rat.X, Y: rat.Y}

	w.npcs[rat.ID] = rat
	return w, rat
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
	visited := map[int]bool{}
	leftStart := false
	returned := false

	for tick := uint64(1); tick <= 400; tick++ {
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)

		if npc.AIState == waitStateID {
			idx := int(npc.Blackboard.WaypointIndex % len(npc.Waypoints))
			visited[idx] = true
			if idx != 0 {
				leftStart = true
			}
			if idx == 0 && leftStart && len(visited) >= 3 {
				returned = true
				break
			}
		}
	}

	if !leftStart {
		t.Fatalf("expected goblin to leave starting waypoint")
	}
	if len(visited) < 3 {
		t.Fatalf("expected goblin to visit at least 3 waypoints, visited %d", len(visited))
	}
	if !returned {
		t.Fatalf("expected goblin to return to first waypoint after visiting patrol route")
	}
}

func TestGoblinPursuesPlayerWithinRange(t *testing.T) {
	w, npc := newStaticAIWorld()
	if npc == nil {
		t.Fatalf("expected goblin NPC")
	}

	player := &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        "player-target",
				X:         npc.X + 200,
				Y:         npc.Y,
				Facing:    defaultFacing,
				Health:    baselinePlayerMaxHealth,
				MaxHealth: baselinePlayerMaxHealth,
				Inventory: NewInventory(),
			},
		},
		stats: stats.DefaultComponent(stats.ArchetypePlayer),
	}
	w.players[player.ID] = player

	pursueStateID := goblinStateID(t, w, "Pursue")

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)

	pursueTick := uint64(0)
	for tick := uint64(1); tick <= 200; tick++ {
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)

		if npc.AIState == pursueStateID {
			pursueTick = tick
			break
		}
	}

	if pursueTick == 0 {
		t.Fatalf("expected goblin to enter pursue state when player nearby")
	}

	if npc.Blackboard.TargetActorID != player.ID {
		t.Fatalf("expected goblin to track nearby player, tracking %q", npc.Blackboard.TargetActorID)
	}
	target := npc.Blackboard.PathTarget
	if math.Abs(target.X-player.X) > 1e-3 || math.Abs(target.Y-player.Y) > 1e-3 {
		t.Fatalf("expected goblin to set path toward player (target=%.2f,%.2f player=%.2f,%.2f)", target.X, target.Y, player.X, player.Y)
	}

	distAtPursue := math.Hypot(npc.X-player.X, npc.Y-player.Y)

	for offset := uint64(1); offset <= 90; offset++ {
		tick := pursueTick + offset
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)
	}

	finalDist := math.Hypot(npc.X-player.X, npc.Y-player.Y)
	if finalDist >= distAtPursue-1e-3 {
		t.Fatalf("expected goblin to close distance while pursuing (%.2f -> %.2f)", distAtPursue, finalDist)
	}
}

func TestGoblinReturnsToPatrolAfterLosingPlayer(t *testing.T) {
	w, npc := newStaticAIWorld()
	if npc == nil {
		t.Fatalf("expected goblin NPC")
	}

	player := &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        "player-escape",
				X:         npc.X + 200,
				Y:         npc.Y,
				Facing:    defaultFacing,
				Health:    baselinePlayerMaxHealth,
				MaxHealth: baselinePlayerMaxHealth,
				Inventory: NewInventory(),
			},
		},
		stats: stats.DefaultComponent(stats.ArchetypePlayer),
	}
	w.players[player.ID] = player

	pursueStateID := goblinStateID(t, w, "Pursue")
	patrolStateID := goblinStateID(t, w, "Patrol")

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)

	pursueTick := uint64(0)
	for tick := uint64(1); tick <= 200; tick++ {
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)

		if npc.AIState == pursueStateID {
			pursueTick = tick
			break
		}
	}
	if pursueTick == 0 {
		t.Fatalf("expected goblin to enter pursue state when player nearby")
	}

	player.X = npc.X + 1000
	player.Y = npc.Y + 1000

	patrolTick := uint64(0)
	for tick := pursueTick + 1; tick <= pursueTick+200; tick++ {
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)

		if npc.AIState == patrolStateID {
			patrolTick = tick
			break
		}
	}

	if patrolTick == 0 {
		t.Fatalf("expected goblin to return to patrol after losing player")
	}

	w.Step(patrolTick+1, now, dt, nil, nil)
	now = now.Add(time.Second / tickRate)

	if len(npc.Waypoints) == 0 {
		t.Fatalf("expected goblin to have patrol waypoints")
	}
	idx := npc.Blackboard.WaypointIndex
	if idx < 0 {
		idx = 0
	}
	waypoint := npc.Waypoints[idx%len(npc.Waypoints)]
	target := npc.Blackboard.PathTarget
	if math.Hypot(target.X-waypoint.X, target.Y-waypoint.Y) > 1 {
		t.Fatalf("expected goblin to resume patrol path target (target=%.2f,%.2f waypoint=%.2f,%.2f)", target.X, target.Y, waypoint.X, waypoint.Y)
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
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)
	}

	blockedIndex := npc.Blackboard.WaypointIndex
	if blockedIndex != 1 {
		t.Fatalf("expected goblin to target second waypoint, got %d", blockedIndex)
	}

	advanced := false
	for tick := uint64(41); tick <= 600; tick++ {
		w.Step(tick, now, dt, nil, nil)
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
		w1.Step(tick, now1, dt, nil, nil)
		w2.Step(tick, now2, dt, nil, nil)
		now1 = now1.Add(time.Second / tickRate)
		now2 = now2.Add(time.Second / tickRate)

		if math.Abs(npc1.X-npc2.X) > 1e-6 || math.Abs(npc1.Y-npc2.Y) > 1e-6 {
			t.Fatalf("npc positions diverged at tick %d", tick)
		}
	}
}

func TestRatWandersAndPicksNewDestinations(t *testing.T) {
	w, rat := newRatTestWorld()
	if rat == nil {
		t.Fatalf("expected rat NPC")
	}

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	startX := rat.X
	startY := rat.Y
	moved := false
	targets := make(map[[2]int]struct{})

	for tick := uint64(1); tick <= 400; tick++ {
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)

		if !moved {
			dx := rat.X - startX
			dy := rat.Y - startY
			if math.Hypot(dx, dy) > 15 {
				moved = true
			}
		}

		target := rat.Blackboard.PathTarget
		if target.X != 0 || target.Y != 0 {
			key := [2]int{int(math.Round(target.X)), int(math.Round(target.Y))}
			targets[key] = struct{}{}
		}
	}

	if !moved {
		t.Fatalf("expected rat to wander away from its starting point")
	}
	if len(targets) < 2 {
		t.Fatalf("expected rat to select multiple roam targets, saw %d", len(targets))
	}
}

func TestRatFleesFromNearbyThreat(t *testing.T) {
	w, rat := newRatTestWorld()
	if rat == nil {
		t.Fatalf("expected rat NPC")
	}

	player := &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        "player-threat",
				X:         rat.X + 20,
				Y:         rat.Y,
				Facing:    defaultFacing,
				Health:    baselinePlayerMaxHealth,
				MaxHealth: baselinePlayerMaxHealth,
				Inventory: NewInventory(),
			},
		},
		stats: stats.DefaultComponent(stats.ArchetypePlayer),
	}
	w.players[player.ID] = player

	var fleeStateID uint8 = 255
	if cfg := w.aiLibrary.ConfigForType(NPCTypeRat); cfg != nil {
		for idx, name := range cfg.stateNames {
			if name == "Flee" {
				fleeStateID = uint8(idx)
				break
			}
		}
	}
	if fleeStateID == 255 {
		t.Fatalf("failed to locate flee state in rat config")
	}

	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	entered := false
	entryTick := uint64(0)

	for tick := uint64(1); tick <= 240; tick++ {
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)
		if rat.AIState == fleeStateID {
			entered = true
			entryTick = tick
			break
		}
	}

	if !entered {
		t.Fatalf("expected rat to enter flee state when threatened")
	}

	entryDist := math.Hypot(rat.X-player.X, rat.Y-player.Y)
	for offset := uint64(1); offset <= 90; offset++ {
		tick := entryTick + offset
		w.Step(tick, now, dt, nil, nil)
		now = now.Add(time.Second / tickRate)
	}
	finalDist := math.Hypot(rat.X-player.X, rat.Y-player.Y)
	if finalDist <= entryDist+1e-3 {
		t.Fatalf("expected rat to increase distance from threat (%.2f -> %.2f)", entryDist, finalDist)
	}
}
