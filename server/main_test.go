package main

import (
	"math"
	"testing"
	"time"

	"mine-and-die/server/logging"
)

func findPlayer(players []Player, id string) *Player {
	for i := range players {
		if players[i].ID == id {
			return &players[i]
		}
	}
	return nil
}

func hasFollowEffect(effects []*effectState, effectType, actorID string) bool {
	for _, eff := range effects {
		if eff == nil {
			continue
		}
		if eff.Type == effectType && eff.FollowActorID == actorID {
			return true
		}
	}
	return false
}

func newTestPlayerState(id string) *playerState {
	return &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				Facing:    defaultFacing,
				Inventory: NewInventory(),
				Health:    playerMaxHealth,
				MaxHealth: playerMaxHealth,
			},
		},
		lastHeartbeat: time.Now(),
		path:          playerPathState{ArriveRadius: defaultPlayerArriveRadius},
	}
}

func runAdvance(h *Hub, dt float64) ([]Player, []NPC, []Effect) {
	players, npcs, effects, _, _ := h.advance(time.Now(), dt)
	return players, npcs, effects
}

func newProjectileTestTemplate(id string) *ProjectileTemplate {
	return &ProjectileTemplate{
		Type:        id,
		Speed:       180,
		MaxDistance: 240,
		SpawnRadius: 0,
		SpawnOffset: 0,
		TravelMode:  TravelModeConfig{StraightLine: true},
		ImpactRules: ImpactRuleConfig{
			StopOnHit:    true,
			MaxTargets:   1,
			AffectsOwner: false,
		},
		Params: map[string]float64{
			"healthDelta": -5,
		},
	}
}

func advanceProjectileTicks(w *World, start time.Time, ticks int, dt float64) {
	current := start
	step := time.Duration(float64(time.Second) * dt)
	for i := 0; i < ticks; i++ {
		w.advanceEffects(current, dt)
		w.pruneEffects(current)
		current = current.Add(step)
	}
}

func advanceAndCollectEffectIDs(w *World, start time.Time, ticks int, dt float64, targetType string) map[string]struct{} {
	current := start
	step := time.Duration(float64(time.Second) * dt)
	seen := make(map[string]struct{})
	for i := 0; i < ticks; i++ {
		w.advanceEffects(current, dt)
		for _, eff := range w.effects {
			if eff.Type == targetType {
				seen[eff.ID] = struct{}{}
			}
		}
		w.pruneEffects(current)
		current = current.Add(step)
	}
	return seen
}

func registerTestProjectileTemplate(w *World, tpl *ProjectileTemplate) {
	if w == nil || tpl == nil {
		return
	}
	w.projectileTemplates[tpl.Type] = tpl
	if w.effectBehaviors == nil {
		w.effectBehaviors = make(map[string]effectBehavior)
	}
	w.effectBehaviors[tpl.Type] = healthDeltaBehavior("healthDelta", 0)
}

func TestHubJoinCreatesPlayer(t *testing.T) {
	hub := newHub()

	first := hub.Join()
	if first.ID == "" {
		t.Fatalf("expected join response to include an id")
	}
	if len(first.Players) != 1 {
		t.Fatalf("expected snapshot to contain 1 player, got %d", len(first.Players))
	}
	if p := findPlayer(first.Players, first.ID); p == nil {
		t.Fatalf("snapshot missing newly joined player %q", first.ID)
	} else if p.Facing == "" {
		t.Fatalf("expected joined player to include facing direction")
	} else if len(p.Inventory.Slots) == 0 {
		t.Fatalf("expected joined player to start with inventory items")
	} else if math.Abs(p.Health-p.MaxHealth) > 1e-6 {
		t.Fatalf("expected player to join at full health, got %.2f/%.2f", p.Health, p.MaxHealth)
	} else if math.Abs(p.MaxHealth-playerMaxHealth) > 1e-6 {
		t.Fatalf("expected max health %.2f, got %.2f", playerMaxHealth, p.MaxHealth)
	}

	second := hub.Join()
	if second.ID == first.ID {
		t.Fatalf("expected unique ids, both were %q", second.ID)
	}
	if len(second.Players) != 2 {
		t.Fatalf("expected snapshot to contain 2 players, got %d", len(second.Players))
	}
	if _, ok := hub.world.players[first.ID]; !ok {
		t.Fatalf("hub players map missing first player")
	}
	if _, ok := hub.world.players[second.ID]; !ok {
		t.Fatalf("hub players map missing second player")
	}

	if len(first.Obstacles) != len(hub.world.obstacles) {
		t.Fatalf("expected join response to include %d obstacles, got %d", len(hub.world.obstacles), len(first.Obstacles))
	}
	if len(first.Effects) != 0 {
		t.Fatalf("expected initial effect list to be empty, got %d", len(first.Effects))
	}
}

func TestWorldGenerationDeterministicWithSeed(t *testing.T) {
	cfg := defaultWorldConfig()
	cfg.Seed = "deterministic-test"

	w1 := newWorld(cfg, logging.NopPublisher{})
	w2 := newWorld(cfg, logging.NopPublisher{})

	if len(w1.obstacles) != len(w2.obstacles) {
		t.Fatalf("expected identical obstacle counts, got %d and %d", len(w1.obstacles), len(w2.obstacles))
	}
	for i := range w1.obstacles {
		if w1.obstacles[i] != w2.obstacles[i] {
			t.Fatalf("expected deterministic obstacles for seed, index %d differed: %#v vs %#v", i, w1.obstacles[i], w2.obstacles[i])
		}
	}

	cfg.Seed = "deterministic-test-alt"
	w3 := newWorld(cfg, logging.NopPublisher{})

	if len(w1.obstacles) != len(w3.obstacles) {
		return
	}
	identical := true
	for i := range w1.obstacles {
		if w1.obstacles[i] != w3.obstacles[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Fatalf("expected different seed to produce different obstacle layout")
	}
}

func TestUpdateIntentNormalizesVector(t *testing.T) {
	hub := newHub()
	playerID := "player-1"
	hub.world.players[playerID] = newTestPlayerState(playerID)

	ok := hub.UpdateIntent(playerID, 10, 0, string(FacingRight))
	if !ok {
		t.Fatalf("expected UpdateIntent to succeed for existing player")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	state := hub.world.players[playerID]
	if state.intentX <= 0 || state.intentX > 1 {
		t.Fatalf("expected normalized intentX in (0,1], got %f", state.intentX)
	}
	if state.intentY != 0 {
		t.Fatalf("expected intentY to remain 0, got %f", state.intentY)
	}
	if state.lastInput.IsZero() {
		t.Fatalf("expected lastInput to be recorded")
	}
	if state.Facing != FacingRight {
		t.Fatalf("expected facing to update to %q, got %q", FacingRight, state.Facing)
	}
}

func TestDeriveFacingFromMovement(t *testing.T) {
	tests := []struct {
		name       string
		dx, dy     float64
		fallback   FacingDirection
		wantFacing FacingDirection
	}{
		{name: "upwardsMovement", dx: 0, dy: -1, fallback: FacingRight, wantFacing: FacingUp},
		{name: "downwardsMovement", dx: 0, dy: 1, fallback: FacingLeft, wantFacing: FacingDown},
		{name: "leftMovement", dx: -1, dy: 0, fallback: FacingUp, wantFacing: FacingLeft},
		{name: "rightMovement", dx: 1, dy: 0, fallback: FacingUp, wantFacing: FacingRight},
		{name: "diagonalPrefersVertical", dx: 1, dy: -1, fallback: FacingDown, wantFacing: FacingUp},
		{name: "diagonalPrefersVerticalDown", dx: -1, dy: 1, fallback: FacingUp, wantFacing: FacingDown},
		{name: "zeroFallsBack", dx: 0, dy: 0, fallback: FacingLeft, wantFacing: FacingLeft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveFacing(tt.dx, tt.dy, tt.fallback); got != tt.wantFacing {
				t.Fatalf("deriveFacing(%f,%f,%q)=%q, want %q", tt.dx, tt.dy, tt.fallback, got, tt.wantFacing)
			}
		})
	}
}

func TestUpdateIntentDerivesFacingFromMovement(t *testing.T) {
	hub := newHub()
	playerID := "vector-facing"
	hub.world.players[playerID] = newTestPlayerState(playerID)

	cases := []struct {
		name string
		dx   float64
		dy   float64
		want FacingDirection
	}{
		{name: "conflictingKeysUp", dx: 0, dy: -1, want: FacingUp},
		{name: "conflictingKeysDown", dx: 0, dy: 1, want: FacingDown},
		{name: "conflictingKeysLeft", dx: -1, dy: 0, want: FacingLeft},
		{name: "conflictingKeysRight", dx: 1, dy: 0, want: FacingRight},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !hub.UpdateIntent(playerID, tc.dx, tc.dy, string(FacingRight)) {
				t.Fatalf("UpdateIntent failed for player")
			}
			runAdvance(hub, 1.0/float64(tickRate))
			state := hub.world.players[playerID]
			if state.Facing != tc.want {
				t.Fatalf("expected facing %q, got %q", tc.want, state.Facing)
			}
		})
	}

	if !hub.UpdateIntent(playerID, 0, 0, string(FacingLeft)) {
		t.Fatalf("UpdateIntent failed for stationary update")
	}
	runAdvance(hub, 1.0/float64(tickRate))
	if got := hub.world.players[playerID].Facing; got != FacingLeft {
		t.Fatalf("expected stationary facing update to respect client facing, got %q", got)
	}
}

func TestPlayerPathCommands(t *testing.T) {
	hub := newHub()
	playerID := "player-path"
	player := newTestPlayerState(playerID)
	hub.world.AddPlayer(player)

	if !hub.SetPlayerPath(playerID, 400, 320) {
		t.Fatalf("expected SetPlayerPath to succeed")
	}
	now := time.Now()
	_, _, _, _, _ = hub.advance(now, 1.0/float64(tickRate))

	state := hub.world.players[playerID]
	if state == nil {
		t.Fatalf("expected player to exist after path command")
	}
	if len(state.path.Path) == 0 {
		t.Fatalf("expected path to be populated for player")
	}
	if state.path.PathTarget.X == 0 && state.path.PathTarget.Y == 0 {
		t.Fatalf("expected path target to be recorded")
	}
	if state.intentX == 0 && state.intentY == 0 {
		t.Fatalf("expected path follower to set intent")
	}

	if !hub.UpdateIntent(playerID, 1, 0, string(FacingRight)) {
		t.Fatalf("expected UpdateIntent to succeed")
	}
	_, _, _, _, _ = hub.advance(now.Add(time.Second), 1.0/float64(tickRate))

	state = hub.world.players[playerID]
	if len(state.path.Path) != 0 {
		t.Fatalf("expected manual input to clear player path")
	}
	if state.intentX <= 0 {
		t.Fatalf("expected manual intent to persist after clearing path, got %f", state.intentX)
	}

	if !hub.SetPlayerPath(playerID, 200, 280) {
		t.Fatalf("expected second SetPlayerPath to succeed")
	}
	_, _, _, _, _ = hub.advance(now.Add(2*time.Second), 1.0/float64(tickRate))
	state = hub.world.players[playerID]
	if len(state.path.Path) == 0 {
		t.Fatalf("expected path to be populated after second command")
	}

	if !hub.ClearPlayerPath(playerID) {
		t.Fatalf("expected ClearPlayerPath to succeed")
	}
	_, _, _, _, _ = hub.advance(now.Add(3*time.Second), 1.0/float64(tickRate))
	state = hub.world.players[playerID]
	if len(state.path.Path) != 0 {
		t.Fatalf("expected player path to be cleared after explicit cancel")
	}
	if state.intentX != 0 || state.intentY != 0 {
		t.Fatalf("expected intent to reset after cancel, got (%f,%f)", state.intentX, state.intentY)
	}
}

func TestAdvanceMovesAndClampsPlayers(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	moverID := "mover"
	boundaryID := "boundary"

	moverState := newTestPlayerState(moverID)
	moverState.X = 80
	moverState.Y = 80
	moverState.intentX = 1
	moverState.intentY = 0
	moverState.lastHeartbeat = now
	hub.world.players[moverID] = moverState

	boundaryState := newTestPlayerState(boundaryID)
	boundaryState.X = worldWidth - playerHalf - 5
	boundaryState.Y = 100
	boundaryState.intentX = 1
	boundaryState.lastHeartbeat = now
	hub.world.players[boundaryID] = boundaryState

	players, _, _, _, toClose := hub.advance(now, 0.5)
	if len(toClose) != 0 {
		t.Fatalf("expected no subscribers to close, got %d", len(toClose))
	}

	mover := findPlayer(players, moverID)
	if mover == nil {
		t.Fatalf("updated snapshot missing mover")
	}
	expectedMoverX := 80 + moveSpeed*0.5
	if mover.X != expectedMoverX {
		t.Fatalf("expected mover X %.1f, got %.1f", expectedMoverX, mover.X)
	}

	boundary := findPlayer(players, boundaryID)
	if boundary == nil {
		t.Fatalf("updated snapshot missing boundary player")
	}
	expectedBoundaryX := worldWidth - playerHalf
	if boundary.X != expectedBoundaryX {
		t.Fatalf("expected boundary player to clamp to %.1f, got %.1f", expectedBoundaryX, boundary.X)
	}
}

func TestAdvanceRemovesStalePlayers(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	staleID := "stale"
	staleState := newTestPlayerState(staleID)
	staleState.X = 100
	staleState.Y = 100
	staleState.lastHeartbeat = time.Now().Add(-disconnectAfter - time.Second)
	hub.world.players[staleID] = staleState

	players, _, _, _, toClose := hub.advance(time.Now(), 0)
	if len(toClose) != 0 {
		t.Fatalf("expected no subscribers returned when none registered")
	}
	if findPlayer(players, staleID) != nil {
		t.Fatalf("stale player still present after advance")
	}
	if _, ok := hub.world.players[staleID]; ok {
		t.Fatalf("stale player still in hub map")
	}
}

func TestMeleeAttackCreatesEffectAndRespectsCooldown(t *testing.T) {
	hub := newHub()
	attackerID := "attacker"
	attackerState := newTestPlayerState(attackerID)
	attackerState.X = 200
	attackerState.Y = 200
	attackerState.Facing = FacingRight
	attackerState.lastHeartbeat = time.Now()
	attackerState.cooldowns = make(map[string]time.Time)
	hub.world.players[attackerID] = attackerState

	if !hub.HandleAction(attackerID, effectTypeAttack) {
		t.Fatalf("expected attack action to be recognized")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	hub.mu.Lock()
	if len(hub.world.effects) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected exactly one effect after first attack, got %d", len(hub.world.effects))
	}
	first := hub.world.effects[0]
	if first.Type != effectTypeAttack {
		hub.mu.Unlock()
		t.Fatalf("expected effect type %q, got %q", effectTypeAttack, first.Type)
	}
	if first.Width <= 0 || first.Height <= 0 {
		hub.mu.Unlock()
		t.Fatalf("expected non-zero effect bounds, got width=%f height=%f", first.Width, first.Height)
	}
	firstStart := first.Start
	hub.mu.Unlock()

	hub.HandleAction(attackerID, effectTypeAttack)
	runAdvance(hub, 1.0/float64(tickRate))
	hub.mu.Lock()
	if len(hub.world.effects) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected cooldown to prevent new effect, have %d", len(hub.world.effects))
	}
	hub.world.players[attackerID].cooldowns[effectTypeAttack] = time.Now().Add(-meleeAttackCooldown)
	hub.mu.Unlock()

	hub.HandleAction(attackerID, effectTypeAttack)
	runAdvance(hub, 1.0/float64(tickRate))
	hub.mu.Lock()
	if len(hub.world.effects) != 2 {
		hub.mu.Unlock()
		t.Fatalf("expected second effect after cooldown reset, have %d", len(hub.world.effects))
	}
	second := hub.world.effects[1]
	hub.mu.Unlock()

	if second.ID == first.ID {
		t.Fatalf("expected unique effect IDs, both were %q", second.ID)
	}
	if second.Start < firstStart {
		t.Fatalf("expected second effect start (%d) to be >= first (%d)", second.Start, firstStart)
	}
}

func TestMeleeAttackDealsDamage(t *testing.T) {
	hub := newHub()
	now := time.Now()
	attackerID := "attacker"
	targetID := "target"

	attackerState := newTestPlayerState(attackerID)
	attackerState.X = 200
	attackerState.Y = 200
	attackerState.Facing = FacingRight
	attackerState.lastHeartbeat = now
	attackerState.cooldowns = make(map[string]time.Time)
	hub.world.players[attackerID] = attackerState

	targetState := newTestPlayerState(targetID)
	targetState.X = 200 + playerHalf + meleeAttackReach/2
	targetState.Y = 200
	targetState.Facing = FacingLeft
	targetState.lastHeartbeat = now
	hub.world.players[targetID] = targetState

	if !hub.HandleAction(attackerID, effectTypeAttack) {
		t.Fatalf("expected melee attack to execute")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	hub.mu.Lock()
	target := hub.world.players[targetID]
	if target == nil {
		hub.mu.Unlock()
		t.Fatalf("expected target to remain in hub")
	}
	expected := playerMaxHealth - meleeAttackDamage
	if math.Abs(target.Health-expected) > 1e-6 {
		hub.mu.Unlock()
		t.Fatalf("expected target health %.1f, got %.1f", expected, target.Health)
	}
	hub.mu.Unlock()
}

func TestMeleeAttackCanDefeatGoblin(t *testing.T) {
	hub := newHub()

	var goblin *npcState
	for _, npc := range hub.world.npcs {
		if npc.Type == NPCTypeGoblin {
			goblin = npc
			break
		}
	}
	if goblin == nil {
		t.Fatalf("expected seeded goblin NPC")
	}

	attackerID := "hero"
	attackerState := newTestPlayerState(attackerID)
	attackerState.X = goblin.X - playerHalf - meleeAttackReach/2
	attackerState.Y = goblin.Y
	attackerState.Facing = FacingRight
	attackerState.cooldowns = make(map[string]time.Time)
	hub.world.players[attackerID] = attackerState

	for i := 0; i < 6; i++ {
		if !hub.HandleAction(attackerID, effectTypeAttack) {
			t.Fatalf("expected melee attack to trigger")
		}
		runAdvance(hub, 1.0/float64(tickRate))
		if i < 5 {
			hub.mu.Lock()
			hub.world.players[attackerID].cooldowns[effectTypeAttack] = time.Now().Add(-meleeAttackCooldown)
			hub.mu.Unlock()
		}
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()

	if _, alive := hub.world.npcs[goblin.ID]; alive {
		t.Fatalf("expected goblin %q to be removed after defeat", goblin.ID)
	}

	if _, ok := hub.world.npcs[goblin.ID]; ok {
		t.Fatalf("expected goblin %q to be removed after defeat", goblin.ID)
	}
}

func TestMeleeAttackAgainstGoldOreAwardsCoin(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = []Obstacle{{
		ID:     "gold-node",
		Type:   obstacleTypeGoldOre,
		X:      180,
		Y:      200,
		Width:  40,
		Height: 40,
	}}

	minerID := "miner"
	minerState := newTestPlayerState(minerID)
	minerState.X = 200
	minerState.Y = 186
	minerState.Facing = FacingDown
	minerState.lastHeartbeat = time.Now()
	minerState.cooldowns = make(map[string]time.Time)
	hub.world.players[minerID] = minerState

	if !hub.HandleAction(minerID, effectTypeAttack) {
		t.Fatalf("expected melee attack to trigger")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	hub.mu.Lock()
	defer hub.mu.Unlock()

	state, ok := hub.world.players[minerID]
	if !ok {
		t.Fatalf("expected miner to remain registered")
	}
	if len(state.Inventory.Slots) != 1 {
		t.Fatalf("expected inventory to contain a single slot, got %d", len(state.Inventory.Slots))
	}
	slot := state.Inventory.Slots[0]
	if slot.Item.Type != ItemTypeGold {
		t.Fatalf("expected slot to contain gold, got %q", slot.Item.Type)
	}
	if slot.Item.Quantity != 1 {
		t.Fatalf("expected 1 gold coin, got %d", slot.Item.Quantity)
	}
}

func TestUpdateHeartbeatRecordsRTT(t *testing.T) {
	hub := newHub()
	playerID := "player"
	hub.world.players[playerID] = newTestPlayerState(playerID)

	received := time.Now()
	clientSent := received.Add(-45 * time.Millisecond).UnixMilli()

	rtt, ok := hub.UpdateHeartbeat(playerID, received, clientSent)
	if !ok {
		t.Fatalf("expected heartbeat update to succeed")
	}
	if rtt <= 0 {
		t.Fatalf("expected positive RTT, got %s", rtt)
	}

	runAdvance(hub, 1.0/float64(tickRate))

	state := hub.world.players[playerID]
	if state.lastHeartbeat != received {
		t.Fatalf("lastHeartbeat not updated: want %v, got %v", received, state.lastHeartbeat)
	}
	if got := state.lastRTT; got != rtt {
		t.Fatalf("state lastRTT mismatch: want %s, got %s", rtt, got)
	}
}

func TestDiagnosticsSnapshotIncludesHeartbeatData(t *testing.T) {
	hub := newHub()
	playerID := "diag"
	now := time.Now()
	diagState := newTestPlayerState(playerID)
	diagState.X = 120
	diagState.Y = 140
	diagState.lastHeartbeat = now
	diagState.lastRTT = 30 * time.Millisecond
	hub.world.players[playerID] = diagState

	snapshot := hub.DiagnosticsSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected diagnostics snapshot with 1 player, got %d", len(snapshot))
	}
	entry := snapshot[0]
	if entry.ID != playerID {
		t.Fatalf("expected diagnostics entry for %q, got %q", playerID, entry.ID)
	}
	if entry.LastHeartbeat != now.UnixMilli() {
		t.Fatalf("expected last heartbeat %d, got %d", now.UnixMilli(), entry.LastHeartbeat)
	}
	if entry.RTTMillis != 30 {
		t.Fatalf("expected RTTMillis 30, got %d", entry.RTTMillis)
	}
}

func TestPlayerStopsAtObstacle(t *testing.T) {
	hub := newHub()
	now := time.Now()

	hub.world.obstacles = []Obstacle{{
		ID:     "obstacle-test",
		X:      160,
		Y:      40,
		Width:  80,
		Height: 160,
	}}

	playerID := "block"
	blockState := newTestPlayerState(playerID)
	blockState.X = 100
	blockState.Y = 120
	blockState.intentX = 1
	blockState.intentY = 0
	blockState.lastHeartbeat = now
	hub.world.players[playerID] = blockState

	players, _, _, _, _ := hub.advance(now, 1)
	blocker := findPlayer(players, playerID)
	if blocker == nil {
		t.Fatalf("expected player in snapshot")
	}

	maxX := hub.world.obstacles[0].X - playerHalf
	if blocker.X > maxX+1e-6 {
		t.Fatalf("expected player to stop before obstacle at %.2f, got %.2f", maxX, blocker.X)
	}
}

func TestLavaAppliesBurningCondition(t *testing.T) {
	hub := newHub()
	now := time.Now()

	hub.world.obstacles = []Obstacle{{
		ID:     "lava-test",
		Type:   obstacleTypeLava,
		X:      200,
		Y:      200,
		Width:  80,
		Height: 80,
	}}

	playerID := "walker"
	walkerState := newTestPlayerState(playerID)
	walkerState.X = 220
	walkerState.Y = 220
	walkerState.lastHeartbeat = now
	hub.world.players[playerID] = walkerState

	players, _, _, _, _ := hub.advance(now, 1.0)

	damaged := findPlayer(players, playerID)
	if damaged == nil {
		t.Fatalf("expected player snapshot")
	}

	expected := playerMaxHealth - lavaDamagePerSecond*burningTickInterval.Seconds()
	if math.Abs(damaged.Health-expected) > 1e-6 {
		t.Fatalf("expected burning to deal %.2f damage, got health %.2f", lavaDamagePerSecond*burningTickInterval.Seconds(), damaged.Health)
	}

	state := hub.world.players[playerID]
	if state == nil {
		t.Fatalf("expected player state tracked")
	}
	if len(state.conditions) == 0 {
		t.Fatalf("expected burning condition to be applied")
	}

	if !hasFollowEffect(hub.world.effects, "fire", playerID) {
		t.Fatalf("expected fire effect following player")
	}

	// Move the player out of lava and ensure burning persists.
	state.X = 40
	state.Y = 40

	healthAfterFirstTick := damaged.Health
	stepNow := now
	for i := 0; i < 3; i++ {
		stepNow = stepNow.Add(burningTickInterval)
		players, _, _, _, _ = hub.advance(stepNow, burningTickInterval.Seconds())
	}

	cooled := findPlayer(players, playerID)
	if cooled == nil {
		t.Fatalf("expected updated player snapshot")
	}
	if cooled.Health >= healthAfterFirstTick {
		t.Fatalf("expected burning to continue after leaving lava, health %.2f vs %.2f", cooled.Health, healthAfterFirstTick)
	}
}

func TestPlayersSeparateWhenColliding(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	firstID := "first"
	secondID := "second"

	firstState := newTestPlayerState(firstID)
	firstState.X = 300
	firstState.Y = 200
	firstState.intentX = 1
	firstState.intentY = 0
	firstState.lastHeartbeat = now
	hub.world.players[firstID] = firstState

	secondState := newTestPlayerState(secondID)
	secondState.X = 300 + playerHalf/2
	secondState.Y = 200
	secondState.intentX = -1
	secondState.intentY = 0
	secondState.lastHeartbeat = now
	hub.world.players[secondID] = secondState

	players, _, _, _, _ := hub.advance(now, 1)

	first := findPlayer(players, firstID)
	second := findPlayer(players, secondID)
	if first == nil || second == nil {
		t.Fatalf("expected both players in snapshot")
	}

	dx := second.X - first.X
	dy := second.Y - first.Y
	distance := math.Hypot(dx, dy)
	minSeparation := playerHalf * 2
	if distance+1e-6 < minSeparation {
		t.Fatalf("expected players separated by at least %.2f, got %.2f", minSeparation, distance)
	}
}

func TestTriggerFireballCreatesProjectile(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	shooterID := "shooter"
	now := time.Now()

	shooterState := newTestPlayerState(shooterID)
	shooterState.X = 200
	shooterState.Y = 200
	shooterState.Facing = FacingRight
	shooterState.lastHeartbeat = now
	shooterState.cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = shooterState

	if !hub.HandleAction(shooterID, effectTypeFireball) {
		t.Fatalf("expected fireball action to be recognized")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	hub.mu.Lock()
	if len(hub.world.effects) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected 1 effect in world, got %d", len(hub.world.effects))
	}
	eff := hub.world.effects[0]
	hub.mu.Unlock()

	if eff.Type != effectTypeFireball {
		t.Fatalf("expected effect type %q, got %q", effectTypeFireball, eff.Type)
	}
	if eff.Projectile == nil {
		t.Fatalf("expected projectile state to be populated")
	}
	expectedRange := fireballRange - fireballSpeed/float64(tickRate)
	if math.Abs(eff.Projectile.RemainingRange-expectedRange) > 1e-6 {
		t.Fatalf("expected remaining range %.2f, got %.2f", expectedRange, eff.Projectile.RemainingRange)
	}
	if eff.Projectile.VelocityUnitX <= 0 || eff.Projectile.VelocityUnitY != 0 {
		t.Fatalf("expected projectile to move right, got velocity (%.2f, %.2f)", eff.Projectile.VelocityUnitX, eff.Projectile.VelocityUnitY)
	}
}

func TestFireballDealsDamageOnHit(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	shooterID := "caster"
	targetID := "victim"

	shooterX := 200.0
	shooterY := 200.0
	travel := fireballSpeed / float64(tickRate)
	spawnOffset := playerHalf + fireballSpawnGap + fireballSize/2

	casterState := newTestPlayerState(shooterID)
	casterState.X = shooterX
	casterState.Y = shooterY
	casterState.Facing = FacingRight
	casterState.lastHeartbeat = now
	casterState.cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = casterState

	victimState := newTestPlayerState(targetID)
	victimState.X = shooterX + spawnOffset + travel/2
	victimState.Y = shooterY
	victimState.Facing = FacingLeft
	victimState.lastHeartbeat = now
	hub.world.players[targetID] = victimState

	if !hub.HandleAction(shooterID, effectTypeFireball) {
		t.Fatalf("expected fireball to be created")
	}

	dt := 1.0 / float64(tickRate)
	step := time.Second / time.Duration(tickRate)
	current := now
	for i := 0; i < 3; i++ {
		_, _, _, _, _ = hub.advance(current, dt)
		current = current.Add(step)
	}

	hub.mu.Lock()
	target := hub.world.players[targetID]
	if target == nil {
		hub.mu.Unlock()
		t.Fatalf("expected target to remain in hub")
	}
	expected := playerMaxHealth - fireballDamage - lavaDamagePerSecond*burningTickInterval.Seconds()
	if math.Abs(target.Health-expected) > 1e-6 {
		hub.mu.Unlock()
		t.Fatalf("expected target health %.1f, got %.1f", expected, target.Health)
	}
	if target.conditions == nil || target.conditions[ConditionBurning] == nil {
		hub.mu.Unlock()
		t.Fatalf("expected fireball hit to apply burning condition")
	}
	hub.mu.Unlock()
}

func TestHealthDeltaHealingClampsToMax(t *testing.T) {
	hub := newHub()
	playerID := "patient"
	state := newTestPlayerState(playerID)
	state.X = 160
	state.Y = 160
	state.Health = playerMaxHealth - 30
	state.lastHeartbeat = time.Now()
	hub.world.players[playerID] = state

	heal := &effectState{Effect: Effect{Type: effectTypeAttack, Owner: "healer", Params: map[string]float64{"healthDelta": 50}}}

	hub.world.applyEffectHitPlayer(heal, state, time.Now())

	if math.Abs(state.Health-playerMaxHealth) > 1e-6 {
		t.Fatalf("expected healing to clamp to max %.1f, got %.1f", playerMaxHealth, state.Health)
	}
}

func TestHealthDamageClampsToZero(t *testing.T) {
	hub := newHub()
	playerID := "fragile"
	state := newTestPlayerState(playerID)
	state.X = 180
	state.Y = 180
	state.Health = 5
	state.lastHeartbeat = time.Now()
	hub.world.players[playerID] = state

	blast := &effectState{Effect: Effect{Type: effectTypeAttack, Owner: "boom", Params: map[string]float64{"healthDelta": -50}}}

	hub.world.applyEffectHitPlayer(blast, state, time.Now())

	if state.Health != 0 {
		t.Fatalf("expected damage to clamp to zero health, got %.1f", state.Health)
	}
}

func TestProjectileExplodeOnImpactSpawnsAreaEffect(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	shooterID := "impact-shooter"
	shooter := newTestPlayerState(shooterID)
	shooter.X = 200
	shooter.Y = 200
	shooter.Facing = FacingRight
	shooter.lastHeartbeat = now
	shooter.cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = shooter

	targetID := "impact-target"
	target := newTestPlayerState(targetID)
	target.X = shooter.X + playerHalf + 4 + 10
	target.Y = shooter.Y
	target.lastHeartbeat = now
	hub.world.players[targetID] = target

	tpl := &ProjectileTemplate{
		Type:        "test-impact",
		Speed:       160,
		MaxDistance: 200,
		SpawnRadius: 4,
		SpawnOffset: playerHalf + 4,
		TravelMode:  TravelModeConfig{StraightLine: true},
		ImpactRules: ImpactRuleConfig{
			StopOnHit: true,
			ExplodeOnImpact: &ExplosionSpec{
				EffectType: "impact-aoe",
				Radius:     12,
				Duration:   500 * time.Millisecond,
				Params:     map[string]float64{"healthDelta": -5},
			},
		},
		Params: map[string]float64{"healthDelta": -5},
	}
	hub.world.projectileTemplates[tpl.Type] = tpl

	eff, ok := hub.world.spawnProjectile(shooterID, tpl.Type, now)
	if !ok || eff == nil {
		t.Fatalf("expected projectile spawn to succeed")
	}

	dt := 1.0 / float64(tickRate)
	step := time.Second / time.Duration(tickRate)
	current := now
	for i := 0; i < 5; i++ {
		hub.world.advanceEffects(current, dt)
		hub.world.pruneEffects(current)
		current = current.Add(step)
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()

	var aoe *effectState
	for _, active := range hub.world.effects {
		if active.Type == "impact-aoe" {
			aoe = active
			break
		}
	}
	if aoe == nil {
		t.Fatalf("expected impact explosion to spawn area effect")
	}
	if len(hub.world.effects) != 1 {
		t.Fatalf("expected only the area effect to remain, found %d", len(hub.world.effects))
	}
	expectedWidth := tpl.ImpactRules.ExplodeOnImpact.Radius * 2
	if math.Abs(aoe.Width-expectedWidth) > 1e-6 {
		t.Fatalf("expected area width %.1f, got %.1f", expectedWidth, aoe.Width)
	}
	if aoe.Duration != tpl.ImpactRules.ExplodeOnImpact.Duration.Milliseconds() {
		t.Fatalf("expected duration %d ms, got %d", tpl.ImpactRules.ExplodeOnImpact.Duration.Milliseconds(), aoe.Duration)
	}
	radiusParam := aoe.Params["radius"]
	if math.Abs(radiusParam-tpl.ImpactRules.ExplodeOnImpact.Radius) > 1e-6 {
		t.Fatalf("expected radius param %.1f, got %.1f", tpl.ImpactRules.ExplodeOnImpact.Radius, radiusParam)
	}
}

func TestProjectileExplodeOnExpirySpawnsAreaEffect(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	shooterID := "expiry-shooter"
	shooter := newTestPlayerState(shooterID)
	shooter.X = 180
	shooter.Y = 220
	shooter.Facing = FacingRight
	shooter.lastHeartbeat = now
	shooter.cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = shooter

	tpl := &ProjectileTemplate{
		Type:        "test-expiry",
		Speed:       120,
		MaxDistance: 60,
		SpawnRadius: 6,
		SpawnOffset: playerHalf + 6,
		TravelMode:  TravelModeConfig{StraightLine: true},
		ImpactRules: ImpactRuleConfig{
			StopOnHit:       true,
			ExplodeOnExpiry: &ExplosionSpec{EffectType: "expiry-aoe", Radius: 18, Duration: 400 * time.Millisecond},
		},
	}
	hub.world.projectileTemplates[tpl.Type] = tpl

	eff, ok := hub.world.spawnProjectile(shooterID, tpl.Type, now)
	if !ok || eff == nil {
		t.Fatalf("expected projectile spawn to succeed")
	}

	dt := 1.0 / float64(tickRate)
	step := time.Second / time.Duration(tickRate)
	current := now
	for i := 0; i < 10; i++ {
		hub.world.advanceEffects(current, dt)
		hub.world.pruneEffects(current)
		current = current.Add(step)
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()

	var aoe *effectState
	for _, active := range hub.world.effects {
		if active.Type == "expiry-aoe" {
			aoe = active
			break
		}
	}
	if aoe == nil {
		t.Fatalf("expected expiry explosion to spawn area effect")
	}
	if len(hub.world.effects) != 1 {
		t.Fatalf("expected only expiry area effect to remain, found %d", len(hub.world.effects))
	}
	expectedX := shooter.X + tpl.SpawnOffset + tpl.MaxDistance - tpl.ImpactRules.ExplodeOnExpiry.Radius
	if math.Abs(aoe.X-expectedX) > 1 {
		t.Fatalf("expected expiry effect near %.1f, got %.1f", expectedX, aoe.X)
	}
	if aoe.Duration != tpl.ImpactRules.ExplodeOnExpiry.Duration.Milliseconds() {
		t.Fatalf("expected duration %d ms, got %d", tpl.ImpactRules.ExplodeOnExpiry.Duration.Milliseconds(), aoe.Duration)
	}
	if math.Abs(aoe.Params["radius"]-tpl.ImpactRules.ExplodeOnExpiry.Radius) > 1e-6 {
		t.Fatalf("expected radius param %.1f, got %.1f", tpl.ImpactRules.ExplodeOnExpiry.Radius, aoe.Params["radius"])
	}
}

func TestFireballExpiresOnObstacleCollision(t *testing.T) {
	hub := newHub()
	now := time.Now()

	shooterID := "caster"
	casterState := newTestPlayerState(shooterID)
	casterState.X = 200
	casterState.Y = 200
	casterState.Facing = FacingRight
	casterState.lastHeartbeat = now
	casterState.cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = casterState

	hub.world.obstacles = []Obstacle{{
		ID:     "wall",
		X:      260,
		Y:      180,
		Width:  40,
		Height: 40,
	}}

	if !hub.HandleAction(shooterID, effectTypeFireball) {
		t.Fatalf("expected fireball to be created")
	}

	step := time.Second / time.Duration(tickRate)
	dt := 1.0 / float64(tickRate)
	expired := false
	current := now

	runAdvance(hub, dt)

	for i := 0; i < tickRate*2; i++ {
		current = current.Add(step)
		_, _, _, _, _ = hub.advance(current, dt)
		if len(hub.world.effects) == 0 {
			expired = true
			break
		}
	}

	if !expired {
		t.Fatalf("expected fireball to expire after hitting obstacle")
	}
}

func TestProjectileStopPolicies(t *testing.T) {
	t.Run("piercesWhenStopDisabled", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("piercer")
		shooter.X = 150
		shooter.Y = 200
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		first := newTestPlayerState("pierce-target-1")
		first.X = shooter.X + 40
		first.Y = shooter.Y
		first.lastHeartbeat = now
		hub.world.players[first.ID] = first

		second := newTestPlayerState("pierce-target-2")
		second.X = shooter.X + 80
		second.Y = shooter.Y
		second.lastHeartbeat = now
		hub.world.players[second.ID] = second

		tpl := newProjectileTestTemplate("pierce-stop")
		tpl.ImpactRules.StopOnHit = false
		tpl.ImpactRules.MaxTargets = 0
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		advanceProjectileTicks(hub.world, now, 3, dt)

		if eff.Projectile == nil {
			t.Fatalf("expected projectile state to persist")
		}
		if eff.Projectile.ExpiryResolved {
			t.Fatalf("expected projectile to remain active after first hit")
		}
		if eff.Projectile.HitCount == 0 {
			t.Fatalf("expected projectile to record at least one hit")
		}

		advanceProjectileTicks(hub.world, now.Add(time.Duration(float64(time.Second)*dt)*3), 4, dt)

		if eff.Projectile.HitCount < 2 {
			t.Fatalf("expected projectile to hit multiple targets, got %d", eff.Projectile.HitCount)
		}
		if eff.Projectile.ExpiryResolved {
			t.Fatalf("expected projectile to remain active after multiple hits")
		}
		if len(hub.world.effects) != 1 {
			t.Fatalf("expected piercing projectile to remain in world, have %d effects", len(hub.world.effects))
		}
	})

	t.Run("stopsWhenConfigured", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("stopper")
		shooter.X = 220
		shooter.Y = 220
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		victim := newTestPlayerState("stop-target")
		victim.X = shooter.X + 36
		victim.Y = shooter.Y
		victim.lastHeartbeat = now
		hub.world.players[victim.ID] = victim

		tpl := newProjectileTestTemplate("stop-on-hit")
		tpl.ImpactRules.StopOnHit = true
		tpl.ImpactRules.MaxTargets = 0
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		advanceProjectileTicks(hub.world, now, 2, dt)

		if len(hub.world.effects) != 0 {
			t.Fatalf("expected projectile to expire after first hit, have %d effects", len(hub.world.effects))
		}
		if eff.Projectile == nil || !eff.Projectile.ExpiryResolved {
			t.Fatalf("expected projectile stop to mark expiry resolved")
		}
	})
}

func TestProjectileMaxTargetsLimit(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	shooter := newTestPlayerState("max-targets-shooter")
	shooter.X = 180
	shooter.Y = 180
	shooter.Facing = FacingRight
	shooter.lastHeartbeat = now
	shooter.cooldowns = make(map[string]time.Time)
	hub.world.players[shooter.ID] = shooter

	first := newTestPlayerState("max-targets-1")
	first.X = shooter.X + 36
	first.Y = shooter.Y
	first.lastHeartbeat = now
	hub.world.players[first.ID] = first

	second := newTestPlayerState("max-targets-2")
	second.X = shooter.X + 72
	second.Y = shooter.Y
	second.lastHeartbeat = now
	hub.world.players[second.ID] = second

	third := newTestPlayerState("max-targets-3")
	third.X = shooter.X + 108
	third.Y = shooter.Y
	third.lastHeartbeat = now
	hub.world.players[third.ID] = third

	tpl := newProjectileTestTemplate("max-targets")
	tpl.ImpactRules.StopOnHit = false
	tpl.ImpactRules.MaxTargets = 2
	registerTestProjectileTemplate(hub.world, tpl)

	eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
	if !spawned || eff == nil {
		t.Fatalf("expected projectile spawn to succeed")
	}

	dt := 1.0 / float64(tickRate)
	advanceProjectileTicks(hub.world, now, 10, dt)

	if len(hub.world.effects) != 0 {
		t.Fatalf("expected projectile to expire after reaching max targets, found %d effects", len(hub.world.effects))
	}
	if eff.Projectile == nil || eff.Projectile.HitCount != 2 {
		t.Fatalf("expected projectile to record 2 hits, got %+v", eff.Projectile)
	}

	if !(first.Health < playerMaxHealth) {
		t.Fatalf("expected first target health to drop below %.1f, got %.1f", playerMaxHealth, first.Health)
	}
	if !(second.Health < playerMaxHealth) {
		t.Fatalf("expected second target health to drop below %.1f, got %.1f", playerMaxHealth, second.Health)
	}
	if math.Abs(third.Health-playerMaxHealth) > 1e-6 {
		t.Fatalf("expected third target to remain unharmed at %.1f, got %.1f", playerMaxHealth, third.Health)
	}
}

func TestProjectileObstacleImpactExplosion(t *testing.T) {
	hub := newHub()
	now := time.Now()

	shooter := newTestPlayerState("impact-shooter")
	shooter.X = 160
	shooter.Y = 200
	shooter.Facing = FacingRight
	shooter.lastHeartbeat = now
	shooter.cooldowns = make(map[string]time.Time)
	hub.world.players[shooter.ID] = shooter

	hub.world.obstacles = []Obstacle{{
		ID:     "wall",
		X:      shooter.X + 60,
		Y:      shooter.Y - 20,
		Width:  30,
		Height: 40,
	}}

	tpl := newProjectileTestTemplate("impact-explosion")
	tpl.ImpactRules.ExplodeOnImpact = &ExplosionSpec{EffectType: "impact-aoe", Radius: 10, Duration: 300 * time.Millisecond}
	registerTestProjectileTemplate(hub.world, tpl)

	eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
	if !spawned || eff == nil {
		t.Fatalf("expected projectile spawn to succeed")
	}

	dt := 1.0 / float64(tickRate)
	ids := advanceAndCollectEffectIDs(hub.world, now, 8, dt, "impact-aoe")
	if len(ids) != 1 {
		t.Fatalf("expected exactly one impact explosion id, got %d", len(ids))
	}
	if eff.Projectile == nil || !eff.Projectile.ExpiryResolved {
		t.Fatalf("expected projectile to resolve after obstacle impact")
	}
}

func TestProjectileExpiryExplosionPolicy(t *testing.T) {
	t.Run("onlyOnWhiff", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("expiry-whiff")
		shooter.X = 180
		shooter.Y = 240
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		tpl := newProjectileTestTemplate("expiry-whiff-template")
		tpl.ImpactRules.StopOnHit = false
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.ExplodeOnExpiry = &ExplosionSpec{EffectType: "expiry-aoe", Radius: 14, Duration: 400 * time.Millisecond}
		tpl.ImpactRules.ExpiryOnlyIfNoHits = true
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		ids := advanceAndCollectEffectIDs(hub.world, now, 25, dt, "expiry-aoe")
		if len(ids) != 1 {
			t.Fatalf("expected single expiry explosion on whiff, got %d", len(ids))
		}
		if eff.Projectile == nil || !eff.Projectile.ExpiryResolved {
			t.Fatalf("expected projectile to resolve after range expiry")
		}
	})

	t.Run("suppressedAfterHit", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("expiry-hit")
		shooter.X = 140
		shooter.Y = 200
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		target := newTestPlayerState("expiry-target")
		target.X = shooter.X + 36
		target.Y = shooter.Y
		target.lastHeartbeat = now
		hub.world.players[target.ID] = target

		tpl := newProjectileTestTemplate("expiry-hit-template")
		tpl.ImpactRules.StopOnHit = false
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.ExplodeOnExpiry = &ExplosionSpec{EffectType: "expiry-aoe", Radius: 12, Duration: 200 * time.Millisecond}
		tpl.ImpactRules.ExpiryOnlyIfNoHits = true
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		ids := advanceAndCollectEffectIDs(hub.world, now, 25, dt, "expiry-aoe")
		if len(ids) != 0 {
			t.Fatalf("did not expect expiry explosion after projectile hit, saw %d ids", len(ids))
		}
		if eff.Projectile == nil || eff.Projectile.HitCount == 0 {
			t.Fatalf("expected projectile to have recorded a hit")
		}
	})

	t.Run("alwaysOnExpiry", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("expiry-always")
		shooter.X = 120
		shooter.Y = 260
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		target := newTestPlayerState("expiry-always-target")
		target.X = shooter.X + 48
		target.Y = shooter.Y
		target.lastHeartbeat = now
		hub.world.players[target.ID] = target

		tpl := newProjectileTestTemplate("expiry-always-template")
		tpl.ImpactRules.StopOnHit = false
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.ExplodeOnExpiry = &ExplosionSpec{EffectType: "expiry-aoe", Radius: 16, Duration: 250 * time.Millisecond}
		tpl.ImpactRules.ExpiryOnlyIfNoHits = false
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		ids := advanceAndCollectEffectIDs(hub.world, now, 25, dt, "expiry-aoe")
		if len(ids) != 1 {
			t.Fatalf("expected expiry explosion regardless of hits, got %d", len(ids))
		}
		if eff.Projectile == nil || eff.Projectile.HitCount == 0 {
			t.Fatalf("expected projectile to have hit before expiry")
		}
	})
}

func TestProjectileBoundsAndLifetimeExpiry(t *testing.T) {
	t.Run("outOfBounds", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("bounds-shooter")
		shooter.X = worldWidth - playerHalf - 30
		shooter.Y = 300
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		tpl := newProjectileTestTemplate("bounds-expiry")
		tpl.ImpactRules.StopOnHit = false
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.ExplodeOnExpiry = &ExplosionSpec{EffectType: "bounds-aoe", Radius: 18, Duration: 150 * time.Millisecond}
		registerTestProjectileTemplate(hub.world, tpl)

		_, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		ids := advanceAndCollectEffectIDs(hub.world, now, 8, dt, "bounds-aoe")
		if len(ids) != 1 {
			t.Fatalf("expected boundary expiry explosion once, found %d", len(ids))
		}
	})

	t.Run("lifetimeExpiry", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("lifetime-shooter")
		shooter.X = 200
		shooter.Y = 320
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		tpl := newProjectileTestTemplate("lifetime-expiry")
		tpl.Lifetime = 200 * time.Millisecond
		tpl.MaxDistance = 0
		tpl.ImpactRules.StopOnHit = false
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.ExplodeOnExpiry = &ExplosionSpec{EffectType: "lifetime-aoe", Radius: 12, Duration: 120 * time.Millisecond}
		registerTestProjectileTemplate(hub.world, tpl)

		_, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		ids := advanceAndCollectEffectIDs(hub.world, now, 8, dt, "lifetime-aoe")
		if len(ids) != 1 {
			t.Fatalf("expected lifetime expiry explosion once, found %d", len(ids))
		}
	})
}

func TestProjectileSpawnDefaults(t *testing.T) {
	hub := newHub()
	hub.world.obstacles = nil
	now := time.Now()

	shooter := newTestPlayerState("spawn-default")
	shooter.X = 260
	shooter.Y = 260
	shooter.Facing = FacingRight
	shooter.lastHeartbeat = now
	shooter.cooldowns = make(map[string]time.Time)
	hub.world.players[shooter.ID] = shooter

	tpl := newProjectileTestTemplate("spawn-default-template")
	tpl.SpawnRadius = 0
	tpl.SpawnOffset = 0
	tpl.CollisionShape = CollisionShapeConfig{UseRect: true, RectWidth: 0, RectHeight: 0}
	registerTestProjectileTemplate(hub.world, tpl)

	eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
	if !spawned || eff == nil {
		t.Fatalf("expected projectile spawn to succeed")
	}

	if eff.Width < 1 || eff.Height < 1 {
		t.Fatalf("expected projectile dimensions to be at least 1x1, got %.2fx%.2f", eff.Width, eff.Height)
	}

	expectedCenter := shooter.X + ownerHalfExtent(&shooter.actorState) + sanitizedSpawnRadius(tpl.SpawnRadius)
	center := eff.X + eff.Width/2
	if math.Abs(center-expectedCenter) > 1e-6 {
		t.Fatalf("expected spawn center %.2f, got %.2f", expectedCenter, center)
	}
}

func TestProjectileOwnerImmunity(t *testing.T) {
	t.Run("selfHitsPrevented", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("self-safe")
		shooter.X = 180
		shooter.Y = 340
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		tpl := newProjectileTestTemplate("self-safe-template")
		tpl.SpawnOffset = -4
		tpl.ImpactRules.StopOnHit = true
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.AffectsOwner = false
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		advanceProjectileTicks(hub.world, now, 2, dt)

		if eff.Projectile == nil {
			t.Fatalf("expected projectile state to exist")
		}
		if eff.Projectile.HitCount != 0 {
			t.Fatalf("expected no self hits when owner immune, got %d", eff.Projectile.HitCount)
		}
		if math.Abs(shooter.Health-playerMaxHealth) > 1e-6 {
			t.Fatalf("expected shooter health to remain %.1f, got %.1f", playerMaxHealth, shooter.Health)
		}
	})

	t.Run("selfHitsAllowed", func(t *testing.T) {
		hub := newHub()
		hub.world.obstacles = nil
		now := time.Now()

		shooter := newTestPlayerState("self-hit")
		shooter.X = 200
		shooter.Y = 360
		shooter.Facing = FacingRight
		shooter.lastHeartbeat = now
		shooter.cooldowns = make(map[string]time.Time)
		hub.world.players[shooter.ID] = shooter

		tpl := newProjectileTestTemplate("self-hit-template")
		tpl.SpawnOffset = -4
		tpl.ImpactRules.StopOnHit = true
		tpl.ImpactRules.MaxTargets = 0
		tpl.ImpactRules.AffectsOwner = true
		registerTestProjectileTemplate(hub.world, tpl)

		eff, spawned := hub.world.spawnProjectile(shooter.ID, tpl.Type, now)
		if !spawned || eff == nil {
			t.Fatalf("expected projectile spawn to succeed")
		}

		dt := 1.0 / float64(tickRate)
		advanceProjectileTicks(hub.world, now, 2, dt)

		if eff.Projectile == nil || eff.Projectile.HitCount == 0 {
			t.Fatalf("expected projectile to record self hit when allowed")
		}
		if !(shooter.Health < playerMaxHealth) {
			t.Fatalf("expected shooter health to drop below %.1f after self hit, got %.1f", playerMaxHealth, shooter.Health)
		}
	})
}
