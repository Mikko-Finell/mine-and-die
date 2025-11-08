package server

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
	simpaches "mine-and-die/server/internal/sim/patches"
	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func findPlayer(players []Player, id string) *Player {
	for i := range players {
		if players[i].ID == id {
			return &players[i]
		}
	}
	return nil
}

func marshalStateLegacy(h *Hub, players []Player, npcs []NPC, triggers []EffectTrigger, groundItems []itemspkg.GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error) {
	simPlayers := simPlayersFromLegacy(players)
	simNPCs := simNPCsFromLegacy(npcs)
	var simTriggers []sim.EffectTrigger
	if len(triggers) > 0 {
		simTriggers = internaleffects.SimEffectTriggersFromLegacy(triggers)
	}
	clonedGroundItems := itemspkg.CloneGroundItems(groundItems)
	return h.marshalState(simPlayers, simNPCs, simTriggers, clonedGroundItems, drainPatches, includeSnapshot)
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

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newTestPlayerState(id string) *playerState {
	return &playerState{
		ActorState: actorState{
			Actor: Actor{
				ID:        id,
				Facing:    defaultFacing,
				Inventory: NewInventory(),
				Equipment: NewEquipment(),
				Health:    baselinePlayerMaxHealth,
				MaxHealth: baselinePlayerMaxHealth,
			},
		},
		Stats:         stats.DefaultComponent(stats.ArchetypePlayer),
		LastHeartbeat: time.Now(),
		Path:          playerPathState{ArriveRadius: defaultPlayerArriveRadius},
	}
}

func runAdvance(h *Hub, dt float64) ([]Player, []NPC) {
	players, npcs, _, _, _ := h.advance(time.Now(), dt)
	return players, npcs
}

type recordingSimEngine struct {
	mu              sync.Mutex
	snapshot        sim.Snapshot
	snapshotPatches []sim.Patch
	calls           int
	gate            chan struct{}
	gateOnce        sync.Once
}

func newRecordingSimEngine(snapshot sim.Snapshot) *recordingSimEngine {
	return &recordingSimEngine{snapshot: snapshot, gate: make(chan struct{})}
}

func (e *recordingSimEngine) Deps() sim.Deps { return sim.Deps{} }

func (e *recordingSimEngine) Enqueue(sim.Command) (bool, string) { return true, "" }

func (e *recordingSimEngine) Pending() int { return 0 }

func (e *recordingSimEngine) DrainCommands() []sim.Command { return nil }

func (e *recordingSimEngine) Advance(ctx sim.LoopTickContext) sim.LoopStepResult {
	snapshot := e.Snapshot()
	return sim.LoopStepResult{Tick: ctx.Tick, Now: ctx.Now, Delta: ctx.Delta, Snapshot: snapshot}
}

func (e *recordingSimEngine) Run(stop <-chan struct{}) {
	if stop == nil {
		return
	}
	<-stop
}

func (e *recordingSimEngine) Apply([]sim.Command) error { return nil }

func (e *recordingSimEngine) Step() {}

func (e *recordingSimEngine) Snapshot() sim.Snapshot {
	e.mu.Lock()
	callIndex := e.calls + 1
	snapshot := e.snapshot
	gate := e.gate
	e.mu.Unlock()

	if callIndex > 1 {
		<-gate
	}
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return snapshot
}

func (e *recordingSimEngine) DrainPatches() []sim.Patch { return nil }

func (e *recordingSimEngine) SnapshotPatches() []sim.Patch {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.snapshotPatches) == 0 {
		return nil
	}
	patches := make([]sim.Patch, len(e.snapshotPatches))
	copy(patches, e.snapshotPatches)
	return patches
}

func (e *recordingSimEngine) RestorePatches([]sim.Patch) {}

func (e *recordingSimEngine) DrainEffectEvents() sim.EffectEventBatch {
	return sim.EffectEventBatch{}
}

func (e *recordingSimEngine) SnapshotEffectEvents() sim.EffectEventBatch {
	return sim.EffectEventBatch{}
}

func (e *recordingSimEngine) RestoreEffectEvents(sim.EffectEventBatch) {}

func (e *recordingSimEngine) ConsumeEffectResyncHint() (sim.EffectResyncSignal, bool) {
	return sim.EffectResyncSignal{}, false
}

func (e *recordingSimEngine) RecordKeyframe(sim.Keyframe) sim.KeyframeRecordResult {
	return sim.KeyframeRecordResult{}
}

func (e *recordingSimEngine) KeyframeBySequence(uint64) (sim.Keyframe, bool) {
	return sim.Keyframe{}, false
}

func (e *recordingSimEngine) KeyframeWindow() (int, uint64, uint64) {
	return 0, 0, 0
}

func (e *recordingSimEngine) Calls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

func (e *recordingSimEngine) AllowFurtherSnapshots() {
	e.gateOnce.Do(func() { close(e.gate) })
}

func (e *recordingSimEngine) SetSnapshotPatches(patches []sim.Patch) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(patches) == 0 {
		e.snapshotPatches = nil
		return
	}
	cloned := make([]sim.Patch, len(patches))
	copy(cloned, patches)
	e.snapshotPatches = cloned
}

func TestHubJoinCreatesPlayer(t *testing.T) {
	hub := newHubWithFullWorld()

	first := hub.Join()
	if first.ID == "" {
		t.Fatalf("expected join response to include an id")
	}
	if len(first.Players) != 1 {
		t.Fatalf("expected snapshot to contain 1 player, got %d", len(first.Players))
	}
	playersLegacy := legacyPlayersFromSim(first.Players)
	if p := findPlayer(playersLegacy, first.ID); p == nil {
		t.Fatalf("snapshot missing newly joined player %q", first.ID)
	} else if p.Facing == "" {
		t.Fatalf("expected joined player to include facing direction")
	} else if len(p.Inventory.Slots) == 0 {
		t.Fatalf("expected joined player to start with inventory items")
	} else if math.Abs(p.Health-p.MaxHealth) > 1e-6 {
		t.Fatalf("expected player to join at full health, got %.2f/%.2f", p.Health, p.MaxHealth)
	} else if math.Abs(p.MaxHealth-baselinePlayerMaxHealth) > 1e-6 {
		t.Fatalf("expected max health %.2f, got %.2f", baselinePlayerMaxHealth, p.MaxHealth)
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
}

func TestMovementEmitsPlayerPositionPatch(t *testing.T) {
	if raceEnabled {
		t.Skip("movement broadcast harness spawns background goroutines that trigger the race detector")
	}
	hub := newHubWithFullWorld()
	joined := hub.Join()
	playerID := joined.ID

	hub.engine.Enqueue(sim.Command{
		ActorID: playerID,
		Type:    sim.CommandMove,
		Move: &sim.MoveCommand{
			DX:     1,
			DY:     0,
			Facing: toSimFacing(FacingRight),
		},
	})

	dt := 1.0 / float64(tickRate)
	now := time.Now()
	players, npcs, triggers, groundItems, _ := hub.advance(now, dt)

	hub.mu.Lock()
	player := hub.world.players[playerID]
	expectedX := player.X
	expectedY := player.Y
	patches := hub.world.snapshotPatchesLocked()
	var posPatch *Patch
	var facingPatch *Patch
	var intentPatch *Patch
	for i := range patches {
		patch := patches[i]
		switch patch.Kind {
		case PatchPlayerPos:
			posPatch = &patch
		case PatchPlayerFacing:
			facingPatch = &patch
		case PatchPlayerIntent:
			intentPatch = &patch
		}
	}
	if posPatch == nil {
		hub.mu.Unlock()
		t.Fatalf("expected position patch before broadcast")
	}
	if facingPatch == nil {
		hub.mu.Unlock()
		t.Fatalf("expected facing patch before broadcast")
	}
	if intentPatch == nil {
		hub.mu.Unlock()
		t.Fatalf("expected intent patch before broadcast")
	}
	facingPayload, ok := facingPatch.Payload.(PlayerFacingPayload)
	if !ok {
		hub.mu.Unlock()
		t.Fatalf("expected facing payload to be PlayerFacingPayload, got %T", facingPatch.Payload)
	}
	if facingPayload.Facing != sim.FacingDirection(FacingRight) {
		hub.mu.Unlock()
		t.Fatalf("expected facing payload to be %q, got %q", FacingRight, facingPayload.Facing)
	}
	intentPayload, ok := intentPatch.Payload.(PlayerIntentPayload)
	if !ok {
		hub.mu.Unlock()
		t.Fatalf("expected intent payload to be PlayerIntentPayload, got %T", intentPatch.Payload)
	}
	if math.Abs(intentPayload.DX-1) > 1e-6 || math.Abs(intentPayload.DY) > 1e-6 {
		hub.mu.Unlock()
		t.Fatalf("expected intent payload to be (1,0), got (%.6f, %.6f)", intentPayload.DX, intentPayload.DY)
	}
	hub.mu.Unlock()

	data, _, err := marshalStateLegacy(hub, players, npcs, triggers, groundItems, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var decoded stateMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}
	var decodedPos map[string]any
	var decodedFacing map[string]any
	var decodedIntent map[string]any
	for _, patch := range decoded.Patches {
		if patch.EntityID != playerID {
			continue
		}
		payload, ok := patch.Payload.(map[string]any)
		if !ok {
			t.Fatalf("expected payload to decode as map, got %T", patch.Payload)
		}
		switch patch.Kind {
		case sim.PatchPlayerPos:
			decodedPos = payload
		case sim.PatchPlayerFacing:
			decodedFacing = payload
		case sim.PatchPlayerIntent:
			decodedIntent = payload
		}
	}
	if decodedPos == nil {
		t.Fatalf("expected position patch after broadcast")
	}
	if decodedFacing == nil {
		t.Fatalf("expected facing patch after broadcast")
	}
	if decodedIntent == nil {
		t.Fatalf("expected intent patch after broadcast")
	}
	x, ok := decodedPos["x"].(float64)
	if !ok {
		t.Fatalf("expected payload.x to be float64, got %T", decodedPos["x"])
	}
	y, ok := decodedPos["y"].(float64)
	if !ok {
		t.Fatalf("expected payload.y to be float64, got %T", decodedPos["y"])
	}
	if facing, ok := decodedFacing["facing"].(string); !ok || facing != string(FacingRight) {
		t.Fatalf("expected facing payload to be %q, got %v", FacingRight, decodedFacing["facing"])
	}
	dx, ok := decodedIntent["dx"].(float64)
	if !ok {
		t.Fatalf("expected intent payload dx to be float64, got %T", decodedIntent["dx"])
	}
	dy, ok := decodedIntent["dy"].(float64)
	if !ok {
		t.Fatalf("expected intent payload dy to be float64, got %T", decodedIntent["dy"])
	}
	if math.Abs(x-expectedX) > 1e-6 || math.Abs(y-expectedY) > 1e-6 {
		t.Fatalf("expected payload coords (%.6f, %.6f), got (%.6f, %.6f)", expectedX, expectedY, x, y)
	}
	if math.Abs(dx-1) > 1e-6 || math.Abs(dy) > 1e-6 {
		t.Fatalf("expected intent payload to be (1,0), got (%.6f, %.6f)", dx, dy)
	}

	hub.mu.Lock()
	if remaining := hub.world.snapshotPatchesLocked(); len(remaining) != 0 {
		hub.mu.Unlock()
		t.Fatalf("expected journal to be drained after broadcast, got %d patches", len(remaining))
	}
	hub.mu.Unlock()
}

func TestWorldGenerationDeterministicWithSeed(t *testing.T) {
	cfg := fullyFeaturedTestWorldConfig()
	cfg.Seed = "deterministic-test"

	w1 := newTestWorld(cfg, logging.NopPublisher{})
	w2 := newTestWorld(cfg, logging.NopPublisher{})

	if len(w1.obstacles) != len(w2.obstacles) {
		t.Fatalf("expected identical obstacle counts, got %d and %d", len(w1.obstacles), len(w2.obstacles))
	}
	for i := range w1.obstacles {
		if w1.obstacles[i] != w2.obstacles[i] {
			t.Fatalf("expected deterministic obstacles for seed, index %d differed: %#v vs %#v", i, w1.obstacles[i], w2.obstacles[i])
		}
	}

	cfg.Seed = "deterministic-test-alt"
	w3 := newTestWorld(cfg, logging.NopPublisher{})

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
	hub := newHubWithFullWorld()
	playerID := "player-1"
	hub.world.players[playerID] = newTestPlayerState(playerID)

	if _, ok, _ := hub.UpdateIntent(playerID, 10, 0, string(FacingRight)); !ok {
		t.Fatalf("expected UpdateIntent to succeed for existing player")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	state := hub.world.players[playerID]
	if state.IntentX <= 0 || state.IntentX > 1 {
		t.Fatalf("expected normalized intentX in (0,1], got %f", state.IntentX)
	}
	if state.IntentY != 0 {
		t.Fatalf("expected intentY to remain 0, got %f", state.IntentY)
	}
	if state.LastInput.IsZero() {
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
	hub := newHubWithFullWorld()
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
			if _, ok, _ := hub.UpdateIntent(playerID, tc.dx, tc.dy, string(FacingRight)); !ok {
				t.Fatalf("UpdateIntent failed for player")
			}
			runAdvance(hub, 1.0/float64(tickRate))
			state := hub.world.players[playerID]
			if state.Facing != tc.want {
				t.Fatalf("expected facing %q, got %q", tc.want, state.Facing)
			}
		})
	}

	if _, ok, _ := hub.UpdateIntent(playerID, 0, 0, string(FacingLeft)); !ok {
		t.Fatalf("UpdateIntent failed for stationary update")
	}
	runAdvance(hub, 1.0/float64(tickRate))
	if got := hub.world.players[playerID].Facing; got != FacingLeft {
		t.Fatalf("expected stationary facing update to respect client facing, got %q", got)
	}
}

func TestPlayerPathCommands(t *testing.T) {
	hub := newHubWithFullWorld()
	playerID := "player-path"
	player := newTestPlayerState(playerID)
	hub.world.AddPlayer(player)

	if _, ok, _ := hub.SetPlayerPath(playerID, 400, 320); !ok {
		t.Fatalf("expected SetPlayerPath to succeed")
	}
	now := time.Now()
	_, _, _, _, _ = hub.advance(now, 1.0/float64(tickRate))

	state := hub.world.players[playerID]
	if state == nil {
		t.Fatalf("expected player to exist after path command")
	}
	if len(state.Path.Path) == 0 {
		t.Fatalf("expected path to be populated for player")
	}
	if state.Path.PathTarget.X == 0 && state.Path.PathTarget.Y == 0 {
		t.Fatalf("expected path target to be recorded")
	}
	if state.IntentX == 0 && state.IntentY == 0 {
		t.Fatalf("expected path follower to set intent")
	}

	if _, ok, _ := hub.UpdateIntent(playerID, 1, 0, string(FacingRight)); !ok {
		t.Fatalf("expected UpdateIntent to succeed")
	}
	_, _, _, _, _ = hub.advance(now.Add(time.Second), 1.0/float64(tickRate))

	state = hub.world.players[playerID]
	if len(state.Path.Path) != 0 {
		t.Fatalf("expected manual input to clear player path")
	}
	if state.IntentX <= 0 {
		t.Fatalf("expected manual intent to persist after clearing path, got %f", state.IntentX)
	}

	if _, ok, _ := hub.SetPlayerPath(playerID, 200, 280); !ok {
		t.Fatalf("expected second SetPlayerPath to succeed")
	}
	_, _, _, _, _ = hub.advance(now.Add(2*time.Second), 1.0/float64(tickRate))
	state = hub.world.players[playerID]
	if len(state.Path.Path) == 0 {
		t.Fatalf("expected path to be populated after second command")
	}

	if _, ok, _ := hub.ClearPlayerPath(playerID); !ok {
		t.Fatalf("expected ClearPlayerPath to succeed")
	}
	_, _, _, _, _ = hub.advance(now.Add(3*time.Second), 1.0/float64(tickRate))
	state = hub.world.players[playerID]
	if len(state.Path.Path) != 0 {
		t.Fatalf("expected player path to be cleared after explicit cancel")
	}
	if state.IntentX != 0 || state.IntentY != 0 {
		t.Fatalf("expected intent to reset after cancel, got (%f,%f)", state.IntentX, state.IntentY)
	}
}

func TestAdvanceMovesAndClampsPlayers(t *testing.T) {
	hub := newHubWithFullWorld()
	hub.world.obstacles = nil
	now := time.Now()

	moverID := "mover"
	boundaryID := "boundary"

	moverState := newTestPlayerState(moverID)
	moverState.X = 80
	moverState.Y = 80
	moverState.LastHeartbeat = now
	hub.world.AddPlayer(moverState)
	hub.world.SetIntent(moverID, 1, 0)

	boundaryState := newTestPlayerState(boundaryID)
	boundaryState.X = hub.world.width() - playerHalf - 5
	boundaryState.Y = 100
	boundaryState.LastHeartbeat = now
	hub.world.AddPlayer(boundaryState)
	hub.world.SetIntent(boundaryID, 1, 0)

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
	expectedBoundaryX := hub.world.width() - playerHalf
	if boundary.X != expectedBoundaryX {
		t.Fatalf("expected boundary player to clamp to %.1f, got %.1f", expectedBoundaryX, boundary.X)
	}
}

func TestWorldRespectsConfiguredDimensions(t *testing.T) {
	cfg := fullyFeaturedTestWorldConfig()
	cfg.Width = 960
	cfg.Height = 540
	w := newTestWorld(cfg, logging.NopPublisher{})
	w.obstacles = nil

	mover := newTestPlayerState("custom-bound")
	mover.X = cfg.Width - playerHalf - 2
	mover.Y = cfg.Height / 2
	mover.IntentX = 1
	moveActorWithObstacles(&mover.ActorState, 1, nil, w.width(), w.height())
	expectedClamp := cfg.Width - playerHalf
	if math.Abs(mover.X-expectedClamp) > 1e-6 {
		t.Fatalf("expected mover to clamp at %.1f, got %.6f", expectedClamp, mover.X)
	}

	pathPlayer := newTestPlayerState("custom-path")
	pathPlayer.X = playerHalf
	pathPlayer.Y = playerHalf
	w.players[pathPlayer.ID] = pathPlayer
	target := vec2{X: cfg.Width + 200, Y: cfg.Height + 200}
	if !w.ensurePlayerPath(pathPlayer, target, 0) {
		t.Fatalf("expected ensurePlayerPath to succeed for uncluttered world")
	}
	if math.Abs(pathPlayer.Path.PathTarget.X-(cfg.Width-playerHalf)) > 1e-6 {
		t.Fatalf("expected path target X to clamp to %.1f, got %.6f", cfg.Width-playerHalf, pathPlayer.Path.PathTarget.X)
	}
	if math.Abs(pathPlayer.Path.PathTarget.Y-(cfg.Height-playerHalf)) > 1e-6 {
		t.Fatalf("expected path target Y to clamp to %.1f, got %.6f", cfg.Height-playerHalf, pathPlayer.Path.PathTarget.Y)
	}

	grid := newNavGrid(nil, w.width(), w.height())
	expectedCols := int(math.Ceil(w.width() / navCellSize))
	expectedRows := int(math.Ceil(w.height() / navCellSize))
	if grid.cols != expectedCols || grid.rows != expectedRows {
		t.Fatalf("expected grid to be %dx%d, got %dx%d", expectedCols, expectedRows, grid.cols, grid.rows)
	}
}

func TestEnsurePlayerPathProducesDiagonalWaypoint(t *testing.T) {
	width := navCellSize * 4
	height := navCellSize * 4
	w := &World{
		players: make(map[string]*playerState),
		npcs:    make(map[string]*npcState),
		journal: newJournal(0, 0),
		config:  worldConfig{Width: width, Height: height},
	}
	grid := newNavGrid(nil, width, height)
	if grid == nil {
		t.Fatalf("expected navigation grid to initialize")
	}

	player := newTestPlayerState("diagonal-player")
	start := grid.worldPos(1, 1)
	player.X = start.X
	player.Y = start.Y
	w.players[player.ID] = player

	goal := grid.worldPos(2, 2)
	target := vec2{X: goal.X, Y: goal.Y}
	if !w.ensurePlayerPath(player, target, 0) {
		t.Fatalf("expected ensurePlayerPath to succeed with open grid")
	}
	if len(player.Path.Path) == 0 {
		t.Fatalf("expected path to contain waypoints")
	}
	if len(player.Path.Path) != 1 {
		t.Fatalf("expected a single diagonal step, got %d", len(player.Path.Path))
	}

	waypoint := player.Path.Path[0]
	dx := waypoint.X - player.X
	dy := waypoint.Y - player.Y
	if math.Abs(dx) <= 1 || math.Abs(dy) <= 1 {
		t.Fatalf("expected diagonal waypoint, got dx=%.2f dy=%.2f", dx, dy)
	}

	w.followPlayerPath(player, 0)
	if math.Abs(player.IntentX) <= 1e-6 || math.Abs(player.IntentY) <= 1e-6 {
		t.Fatalf("expected followPlayerPath to emit diagonal intent, got dx=%.2f dy=%.2f", player.IntentX, player.IntentY)
	}
}

func TestAdvanceRemovesStalePlayers(t *testing.T) {
	hub := newHubWithFullWorld()
	hub.world.obstacles = nil
	staleID := "stale"
	staleState := newTestPlayerState(staleID)
	staleState.X = 100
	staleState.Y = 100
	staleState.LastHeartbeat = time.Now().Add(-disconnectAfter - time.Second)
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
	hub := newHubWithFullWorld()
	attackerID := "attacker"
	attackerState := newTestPlayerState(attackerID)
	attackerState.X = 200
	attackerState.Y = 200
	attackerState.Facing = FacingRight
	attackerState.LastHeartbeat = time.Now()
	attackerState.Cooldowns = make(map[string]time.Time)
	hub.world.players[attackerID] = attackerState

	if _, ok, _ := hub.HandleAction(attackerID, effectTypeAttack); !ok {
		t.Fatalf("expected attack action to be recognized")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	hub.mu.Lock()
	batch := hub.world.SnapshotEffectEvents()
	if len(batch.Spawns) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected exactly one effect spawn after first attack, got %d", len(batch.Spawns))
	}
	firstSpawn := batch.Spawns[0]
	if firstSpawn.Instance.DefinitionID != effectTypeAttack {
		hub.mu.Unlock()
		t.Fatalf("expected spawn definition %q, got %q", effectTypeAttack, firstSpawn.Instance.DefinitionID)
	}
	if firstSpawn.Instance.ID == "" {
		hub.mu.Unlock()
		t.Fatalf("expected spawn instance id to be set")
	}
	firstTick := firstSpawn.Tick
	hub.mu.Unlock()

	_, _, _ = hub.HandleAction(attackerID, effectTypeAttack)
	runAdvance(hub, 1.0/float64(tickRate))
	hub.mu.Lock()
	batch = hub.world.SnapshotEffectEvents()
	if len(batch.Spawns) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected cooldown to prevent new spawn, have %d", len(batch.Spawns))
	}
	hub.world.players[attackerID].Cooldowns[effectTypeAttack] = time.Now().Add(-meleeAttackCooldown)
	hub.mu.Unlock()

	_, _, _ = hub.HandleAction(attackerID, effectTypeAttack)
	runAdvance(hub, 1.0/float64(tickRate))
	hub.mu.Lock()
	batch = hub.world.SnapshotEffectEvents()
	if len(batch.Spawns) != 2 {
		hub.mu.Unlock()
		t.Fatalf("expected second spawn after cooldown reset, have %d", len(batch.Spawns))
	}
	second := batch.Spawns[1]
	hub.mu.Unlock()

	if second.Instance.ID == firstSpawn.Instance.ID {
		t.Fatalf("expected unique effect IDs, both were %q", second.Instance.ID)
	}
	if second.Tick < firstTick {
		t.Fatalf("expected second spawn tick (%d) to be >= first (%d)", second.Tick, firstTick)
	}
}

func TestMeleeAttackDealsDamage(t *testing.T) {
	hub := newHubWithFullWorld()
	now := time.Now()
	attackerID := "attacker"
	targetID := "target"

	attackerState := newTestPlayerState(attackerID)
	attackerState.X = 200
	attackerState.Y = 200
	attackerState.Facing = FacingRight
	attackerState.LastHeartbeat = now
	attackerState.Cooldowns = make(map[string]time.Time)
	hub.world.players[attackerID] = attackerState

	targetState := newTestPlayerState(targetID)
	targetState.X = 200 + playerHalf + meleeAttackReach/2
	targetState.Y = 200
	targetState.Facing = FacingLeft
	targetState.LastHeartbeat = now
	hub.world.players[targetID] = targetState

	if _, ok, _ := hub.HandleAction(attackerID, effectTypeAttack); !ok {
		t.Fatalf("expected melee attack to execute")
	}

	runAdvance(hub, 1.0/float64(tickRate))

	hub.mu.Lock()
	target := hub.world.players[targetID]
	if target == nil {
		hub.mu.Unlock()
		t.Fatalf("expected target to remain in hub")
	}
	expected := baselinePlayerMaxHealth - meleeAttackDamage
	if math.Abs(target.Health-expected) > 1e-6 {
		hub.mu.Unlock()
		t.Fatalf("expected target health %.1f, got %.1f", expected, target.Health)
	}
	hub.mu.Unlock()
}

func TestMeleeAttackCanDefeatGoblin(t *testing.T) {
	hub := newHubWithFullWorld()

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
	attackerState.Cooldowns = make(map[string]time.Time)
	hub.world.players[attackerID] = attackerState

	const maxSwings = 10
	for i := 0; i < maxSwings; i++ {
		hub.mu.Lock()
		attacker := hub.world.players[attackerID]
		attacker.X = goblin.X - playerHalf - meleeAttackReach/2
		attacker.Y = goblin.Y
		attacker.Facing = FacingRight
		hub.mu.Unlock()

		if _, ok, _ := hub.HandleAction(attackerID, effectTypeAttack); !ok {
			t.Fatalf("expected melee attack to trigger")
		}
		runAdvance(hub, 1.0/float64(tickRate))
		hub.mu.Lock()
		if _, alive := hub.world.npcs[goblin.ID]; !alive {
			hub.mu.Unlock()
			break
		}
		if i < maxSwings-1 {
			hub.world.players[attackerID].Cooldowns[effectTypeAttack] = time.Now().Add(-meleeAttackCooldown)
		}
		hub.mu.Unlock()
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()

	if npc, alive := hub.world.npcs[goblin.ID]; alive {
		t.Fatalf("expected goblin %q to be removed after defeat (health=%.2f)", goblin.ID, npc.Health)
	}

	if _, ok := hub.world.npcs[goblin.ID]; ok {
		t.Fatalf("expected goblin %q to be removed after defeat", goblin.ID)
	}
}

func TestContractMeleeHitBroadcastsBloodEffect(t *testing.T) {
	if raceEnabled {
		t.Skip("contract melee broadcast relies on background hub goroutines not race-safe yet")
	}
	hub := newHubWithFullWorld()

	join := hub.Join()
	playerID := join.ID
	if playerID == "" {
		t.Fatal("expected joined player id")
	}

	now := time.Unix(0, 0)
	dt := 1.0 / float64(tickRate)

	const (
		playerX  = 360.0
		playerY  = 360.0
		goblinID = "npc-contract-target"
	)

	hub.mu.Lock()
	hub.world.obstacles = nil
	hub.world.npcs = make(map[string]*npcState)
	hub.world.SetPosition(playerID, playerX, playerY)
	hub.world.SetFacing(playerID, FacingRight)

	goblinStats := stats.DefaultComponent(stats.ArchetypeGoblin)
	goblinStats.Resolve(0)
	maxHealth := goblinStats.GetDerived(stats.DerivedMaxHealth)
	if maxHealth <= 0 {
		hub.mu.Unlock()
		t.Fatalf("expected goblin max health to be positive, got %.2f", maxHealth)
	}

	goblin := &npcState{
		ActorState: actorState{Actor: Actor{
			ID:        goblinID,
			X:         playerX + playerHalf + meleeAttackReach/2,
			Y:         playerY,
			Facing:    FacingLeft,
			Health:    maxHealth,
			MaxHealth: maxHealth,
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
		}},
		Stats:     goblinStats,
		Type:      NPCTypeGoblin,
		Cooldowns: make(map[string]time.Time),
	}
	hub.world.npcs[goblin.ID] = goblin
	hub.mu.Unlock()

	hub.world.DrainPatches()
	_ = hub.world.DrainEffectEvents()

	if _, ok, _ := hub.HandleAction(playerID, effectTypeAttack); !ok {
		t.Fatalf("expected melee attack command to be accepted")
	}

	players, npcs, triggers, groundItems, _ := hub.advance(now, dt)

	step := time.Second / time.Duration(tickRate)
	nextNow := now.Add(step)
	players, npcs, triggers, groundItems, _ = hub.advance(nextNow, dt)

	data, _, err := marshalStateLegacy(hub, players, npcs, triggers, groundItems, true, false)
	if err != nil {
		t.Fatalf("failed to marshal state message: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	if len(msg.EffectSpawns) == 0 {
		t.Fatalf("expected contract effect spawns, got none")
	}

	foundBlood := false
	for _, spawn := range msg.EffectSpawns {
		if spawn.Instance.DefinitionID != effectTypeBloodSplatter {
			continue
		}
		foundBlood = true
		if !slicesEqual(spawn.Instance.Colors, []string{"#7a0e12", "#4a090b"}) {
			t.Fatalf("expected spawn colors %v, got %v", []string{"#7a0e12", "#4a090b"}, spawn.Instance.Colors)
		}
	}
	if !foundBlood {
		t.Fatalf("expected blood splatter contract effect spawn, got %+v", msg.EffectSpawns)
	}
}

func TestMeleeAttackAgainstGoldOreAwardsCoin(t *testing.T) {
	hub := newHubWithFullWorld()
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
	minerState.LastHeartbeat = time.Now()
	minerState.Cooldowns = make(map[string]time.Time)
	hub.world.players[minerID] = minerState

	if _, ok, _ := hub.HandleAction(minerID, effectTypeAttack); !ok {
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
	hub := newHubWithFullWorld()
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
	if state.LastHeartbeat != received {
		t.Fatalf("lastHeartbeat not updated: want %v, got %v", received, state.LastHeartbeat)
	}
	if got := state.LastRTT; got != rtt {
		t.Fatalf("state lastRTT mismatch: want %s, got %s", rtt, got)
	}
}

func TestDiagnosticsSnapshotIncludesHeartbeatData(t *testing.T) {
	hub := newHubWithFullWorld()
	playerID := "diag"
	now := time.Now()
	diagState := newTestPlayerState(playerID)
	diagState.X = 120
	diagState.Y = 140
	diagState.LastHeartbeat = now
	diagState.LastRTT = 30 * time.Millisecond
	hub.world.players[playerID] = diagState

	sub := newSubscriber(nil, nil)
	sub.lastAck.Store(47)
	hub.mu.Lock()
	hub.subscribers[playerID] = sub
	hub.mu.Unlock()

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
	if entry.LastAck != 47 {
		t.Fatalf("expected LastAck 47, got %d", entry.LastAck)
	}
}

func TestRecordAckTracksMonotonicProgress(t *testing.T) {
	hub := newHubWithFullWorld()
	playerID := "monotonic"

	sub := newSubscriber(nil, nil)
	hub.mu.Lock()
	hub.subscribers[playerID] = sub
	hub.mu.Unlock()

	hub.RecordAck(playerID, 10)
	if got := sub.lastAck.Load(); got != 10 {
		t.Fatalf("expected lastAck to be 10, got %d", got)
	}

	hub.RecordAck(playerID, 8)
	if got := sub.lastAck.Load(); got != 10 {
		t.Fatalf("expected ack regression to be ignored, got %d", got)
	}

	hub.RecordAck(playerID, 25)
	if got := sub.lastAck.Load(); got != 25 {
		t.Fatalf("expected lastAck to advance to 25, got %d", got)
	}
}

func TestPlayerStopsAtObstacle(t *testing.T) {
	hub := newHubWithFullWorld()
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
	blockState.LastHeartbeat = now
	hub.world.AddPlayer(blockState)
	hub.world.SetIntent(playerID, 1, 0)

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

func TestLavaAppliesBurningStatusEffect(t *testing.T) {
	hub := newHubWithFullWorld()
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
	walkerState.LastHeartbeat = now
	hub.world.players[playerID] = walkerState

	players, _, _, _, _ := hub.advance(now, 1.0)

	damaged := findPlayer(players, playerID)
	if damaged == nil {
		t.Fatalf("expected player snapshot")
	}

	expected := baselinePlayerMaxHealth - lavaDamagePerSecond*burningTickInterval.Seconds()
	if math.Abs(damaged.Health-expected) > 1e-6 {
		t.Fatalf("expected burning to deal %.2f damage, got health %.2f", lavaDamagePerSecond*burningTickInterval.Seconds(), damaged.Health)
	}

	state := hub.world.players[playerID]
	if state == nil {
		t.Fatalf("expected player state tracked")
	}
	if len(state.StatusEffects) == 0 {
		t.Fatalf("expected burning status effect to be applied")
	}

	if !hasFollowEffect(hub.world.effects, effectTypeBurningVisual, playerID) {
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
	hub := newHubWithFullWorld()
	hub.world.obstacles = nil
	now := time.Now()

	firstID := "first"
	secondID := "second"

	firstState := newTestPlayerState(firstID)
	firstState.X = 300
	firstState.Y = 200
	firstState.LastHeartbeat = now
	hub.world.AddPlayer(firstState)
	hub.world.SetIntent(firstID, 1, 0)

	secondState := newTestPlayerState(secondID)
	secondState.X = 300 + playerHalf/2
	secondState.Y = 200
	secondState.LastHeartbeat = now
	hub.world.AddPlayer(secondState)
	hub.world.SetIntent(secondID, -1, 0)

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
	hub := newHubWithFullWorld()
	hub.world.obstacles = nil
	shooterID := "shooter"
	now := time.Now()

	shooterState := newTestPlayerState(shooterID)
	shooterState.X = 200
	shooterState.Y = 200
	shooterState.Facing = FacingRight
	shooterState.LastHeartbeat = now
	shooterState.Cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = shooterState

	if _, ok, _ := hub.HandleAction(shooterID, effectTypeFireball); !ok {
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

func TestContractMeleeDefinitionsApplyDamage(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}
	world.obstacles = nil

	attacker := newTestPlayerState("contract-attacker")
	attacker.X = 200
	attacker.Y = 200
	attacker.Facing = FacingRight
	attacker.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(attacker)

	target := newTestPlayerState("contract-target")
	target.X = 200 + playerHalf + meleeAttackReach/2
	target.Y = 200
	world.AddPlayer(target)

	commands := []Command{{ActorID: attacker.ID, Type: CommandAction, Action: &ActionCommand{Name: effectTypeAttack}}}
	now := time.Unix(0, 0)

	world.Step(1, now, 1.0/float64(tickRate), commands, nil)

	victim := world.players[target.ID]
	if victim == nil {
		t.Fatalf("expected victim to remain in world")
	}
	expected := baselinePlayerMaxHealth - meleeAttackDamage
	if math.Abs(victim.Health-expected) > 1e-6 {
		t.Fatalf("expected victim health %.1f, got %.1f", expected, victim.Health)
	}
}

func TestContractMeleeSpawnPopulatesMotionCenter(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}
	world.obstacles = nil

	attacker := newTestPlayerState("contract-motion-attacker")
	attacker.X = 320
	attacker.Y = 280
	attacker.Facing = FacingRight
	attacker.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(attacker)

	commands := []Command{{ActorID: attacker.ID, Type: CommandAction, Action: &ActionCommand{Name: effectTypeAttack}}}
	now := time.Unix(0, 0)

	world.Step(1, now, 1.0/float64(tickRate), commands, nil)

	batch := world.SnapshotEffectEvents()
	if len(batch.Spawns) != 1 {
		t.Fatalf("expected exactly one effect spawn, got %d", len(batch.Spawns))
	}
	spawn := batch.Spawns[0]
	motion := spawn.Instance.DeliveryState.Motion
	if motion.PositionX == 0 || motion.PositionY == 0 {
		t.Fatalf("expected motion center to be populated, got position=(%d,%d)", motion.PositionX, motion.PositionY)
	}
	geom := spawn.Instance.DeliveryState.Geometry
	expectedCenterX := quantizeWorldCoord(attacker.X + internaleffects.DequantizeWorldCoord(geom.OffsetX, tileSize))
	expectedCenterY := quantizeWorldCoord(attacker.Y + internaleffects.DequantizeWorldCoord(geom.OffsetY, tileSize))
	if motion.PositionX != expectedCenterX {
		t.Fatalf("expected motion positionX %d, got %d", expectedCenterX, motion.PositionX)
	}
	if motion.PositionY != expectedCenterY {
		t.Fatalf("expected motion positionY %d, got %d", expectedCenterY, motion.PositionY)
	}
}

func TestContractProjectileDefinitionsApplyDamage(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}
	world.obstacles = nil

	caster := newTestPlayerState("contract-caster")
	caster.X = 200
	caster.Y = 200
	caster.Facing = FacingRight
	caster.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(caster)

	target := newTestPlayerState("contract-projectile-target")
	travel := fireballSpeed / float64(tickRate)
	spawnOffset := playerHalf + fireballSpawnGap + fireballSize/2
	target.X = caster.X + spawnOffset + travel*5
	target.Y = caster.Y
	world.AddPlayer(target)

	commands := []Command{{ActorID: caster.ID, Type: CommandAction, Action: &ActionCommand{Name: effectTypeFireball}}}
	now := time.Unix(0, 0)
	dt := 1.0 / float64(tickRate)

	world.Step(1, now, dt, commands, nil)

	var managed *effectState
	for _, eff := range world.effects {
		if eff == nil {
			continue
		}
		if eff.Type == effectTypeFireball {
			managed = eff
			break
		}
	}
	if managed == nil {
		t.Fatalf("expected contract fireball effect to spawn")
	}
	if !managed.ContractManaged {
		t.Fatalf("expected contract-managed projectile, got legacy effect")
	}

	current := now.Add(time.Second / time.Duration(tickRate))
	expected := baselinePlayerMaxHealth - fireballDamage - lavaDamagePerSecond*burningTickInterval.Seconds()
	var victim *playerState
	for tick := uint64(2); tick <= 12; tick++ {
		world.Step(tick, current, dt, nil, nil)
		current = current.Add(time.Second / time.Duration(tickRate))
		victim = world.players[target.ID]
		if victim != nil && victim.Health <= expected {
			break
		}
	}

	if victim == nil {
		t.Fatalf("expected target to remain in world")
	}

	if math.Abs(victim.Health-expected) > 1e-6 {
		t.Fatalf("expected victim health %.1f, got %.1f", expected, victim.Health)
	}
	if victim.StatusEffects == nil || victim.StatusEffects[StatusEffectBurning] == nil {
		t.Fatalf("expected fireball to apply burning status")
	}
}

func TestContractProjectileSpawnPopulatesMotionCenter(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}
	world.obstacles = nil

	caster := newTestPlayerState("contract-motion-caster")
	caster.X = 360
	caster.Y = 300
	caster.Facing = FacingRight
	caster.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(caster)

	commands := []Command{{ActorID: caster.ID, Type: CommandAction, Action: &ActionCommand{Name: effectTypeFireball}}}
	now := time.Unix(0, 0)

	world.Step(1, now, 1.0/float64(tickRate), commands, nil)

	batch := world.SnapshotEffectEvents()
	if len(batch.Spawns) == 0 {
		t.Fatal("expected projectile spawn event")
	}
	var spawn effectcontract.EffectSpawnEvent
	for _, evt := range batch.Spawns {
		if evt.Instance.DefinitionID == effectTypeFireball {
			spawn = evt
			break
		}
	}
	if spawn.Instance.ID == "" {
		t.Fatal("expected projectile spawn to include instance")
	}
	motion := spawn.Instance.DeliveryState.Motion
	if motion.PositionX == 0 && motion.PositionY == 0 {
		t.Fatalf("expected projectile motion center to be populated, got position=(%d,%d)", motion.PositionX, motion.PositionY)
	}
	geom := spawn.Instance.DeliveryState.Geometry
	expectedCenterX := quantizeWorldCoord(caster.X + internaleffects.DequantizeWorldCoord(geom.OffsetX, tileSize))
	expectedCenterY := quantizeWorldCoord(caster.Y + internaleffects.DequantizeWorldCoord(geom.OffsetY, tileSize))
	if motion.PositionX != expectedCenterX {
		t.Fatalf("expected projectile positionX %d, got %d", expectedCenterX, motion.PositionX)
	}
	if motion.PositionY != expectedCenterY {
		t.Fatalf("expected projectile positionY %d, got %d", expectedCenterY, motion.PositionY)
	}
}

func TestContractBurningDefinitionsApplyDamage(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}

	target := newTestPlayerState("contract-burning-target")
	target.X = 240
	target.Y = 160
	world.AddPlayer(target)

	actor := &world.players[target.ID].ActorState
	now := time.Unix(0, 0)

	if applied := world.applyStatusEffect(actor, StatusEffectBurning, "lava-1", now); !applied {
		t.Fatalf("expected burning status effect to apply")
	}

	if world.effectManager.PendingIntentCount() == 0 {
		t.Fatalf("expected burning application to enqueue contract intents")
	}

	world.effectManager.RunTick(effectcontract.Tick(1), now, nil)

	burned := world.players[target.ID]
	expected := baselinePlayerMaxHealth - lavaDamagePerSecond*burningTickInterval.Seconds()
	if math.Abs(burned.Health-expected) > 1e-6 {
		t.Fatalf("expected target health %.2f after first tick, got %.2f", expected, burned.Health)
	}

	inst := burned.StatusEffects[StatusEffectBurning]
	if inst == nil {
		t.Fatalf("expected burning status effect instance to persist")
	}
	effect, ok := inst.AttachedEffect().(*effectState)
	if !ok || effect == nil {
		t.Fatalf("expected burning visual to attach to status effect")
	}
	if !effect.ContractManaged {
		t.Fatalf("expected burning visual to be contract-managed")
	}
	if effect.FollowActorID != target.ID {
		t.Fatalf("expected burning visual to follow %s", target.ID)
	}

	next := now.Add(burningTickInterval)
	world.advanceStatusEffects(next)
	if world.effectManager.PendingIntentCount() == 0 {
		t.Fatalf("expected burning tick to enqueue contract damage intent")
	}
	world.effectManager.RunTick(effectcontract.Tick(2), next, nil)

	burned = world.players[target.ID]
	expected = baselinePlayerMaxHealth - 2*lavaDamagePerSecond*burningTickInterval.Seconds()
	if math.Abs(burned.Health-expected) > 1e-6 {
		t.Fatalf("expected target health %.2f after second tick, got %.2f", expected, burned.Health)
	}
}

func TestContractBloodDecalDefinitionsSpawn(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}

	attacker := newTestPlayerState("player-blood-attacker")
	attacker.X = 260
	attacker.Y = 300
	world.AddPlayer(attacker)

	npc := &npcState{
		ActorState: actorState{Actor: Actor{
			ID:        "npc-target",
			X:         280,
			Y:         300,
			Health:    25,
			MaxHealth: 25,
			Inventory: NewInventory(),
		}},
		Stats: stats.DefaultComponent(stats.ArchetypeGoblin),
		Type:  NPCTypeGoblin,
	}
	world.npcs[npc.ID] = npc

	eff := &effectState{
		Type:   effectTypeAttack,
		Owner:  attacker.ID,
		X:      npc.X - playerHalf,
		Y:      npc.Y - playerHalf,
		Width:  playerHalf * 2,
		Height: playerHalf * 2,
	}

	now := time.Unix(0, 0)
	world.invokeNPCHitCallback(eff, npc, now)

	if world.effectManager.PendingIntentCount() == 0 {
		t.Fatalf("expected blood splatter to enqueue contract intent")
	}
	if triggers := world.flushEffectTriggersLocked(); len(triggers) != 0 {
		t.Fatalf("expected legacy triggers to be suppressed when contract blood decals enabled")
	}

	world.effectManager.RunTick(effectcontract.Tick(1), now, nil)

	if world.effectManager.PendingIntentCount() != 0 {
		t.Fatalf("expected intent queue to drain after tick, got %d", world.effectManager.PendingIntentCount())
	}

	if len(world.effectManager.Instances()) != 1 {
		t.Fatalf("expected one contract blood decal instance, got %d", len(world.effectManager.Instances()))
	}

	var instance *effectcontract.EffectInstance
	for _, inst := range world.effectManager.Instances() {
		instance = inst
		break
	}
	if instance == nil {
		t.Fatal("expected contract instance to exist")
	}
	if instance.DefinitionID != effectTypeBloodSplatter {
		t.Fatalf("expected DefinitionID %q, got %q", effectTypeBloodSplatter, instance.DefinitionID)
	}

	effect := world.effectManager.WorldEffect(instance.ID)
	if effect == nil {
		t.Fatal("expected contract blood decal to spawn in world state")
	}
	if effect.Type != effectTypeBloodSplatter {
		t.Fatalf("expected world effect type %q, got %q", effectTypeBloodSplatter, effect.Type)
	}
	if !effect.ContractManaged {
		t.Fatalf("expected blood decal to be contract-managed")
	}

	expectedColors := []string{"#7a0e12", "#4a090b"}
	if !slicesEqual(effect.Colors, expectedColors) {
		t.Fatalf("expected effect colors %v, got %v", expectedColors, effect.Colors)
	}
	for key, expected := range map[string]float64{
		"drag":           0.92,
		"dropletRadius":  3,
		"maxBursts":      0,
		"maxDroplets":    33,
		"maxStainRadius": 6,
		"maxStains":      140,
		"minDroplets":    4,
		"minStainRadius": 4,
		"spawnInterval":  1.1,
		"speed":          3,
	} {
		if value, ok := effect.Params[key]; !ok || math.Abs(value-expected) > 1e-6 {
			t.Fatalf("expected effect param %s=%.2f, got %.2f (present=%t)", key, expected, value, ok)
		}
	}

	centerXQuant := quantizeWorldCoord(npc.X)
	centerYQuant := quantizeWorldCoord(npc.Y)
	if instance.BehaviorState.Extra["centerX"] != centerXQuant {
		t.Fatalf("expected instance centerX %d, got %d", centerXQuant, instance.BehaviorState.Extra["centerX"])
	}
	if instance.BehaviorState.Extra["centerY"] != centerYQuant {
		t.Fatalf("expected instance centerY %d, got %d", centerYQuant, instance.BehaviorState.Extra["centerY"])
	}

	width := internaleffects.DequantizeWorldCoord(instance.DeliveryState.Geometry.Width, tileSize)
	if width <= 0 {
		width = playerHalf * 2
	}
	height := internaleffects.DequantizeWorldCoord(instance.DeliveryState.Geometry.Height, tileSize)
	if height <= 0 {
		height = playerHalf * 2
	}
	centerX := internaleffects.DequantizeWorldCoord(centerXQuant, tileSize)
	centerY := internaleffects.DequantizeWorldCoord(centerYQuant, tileSize)
	expectedX := centerX - width/2
	expectedY := centerY - height/2
	if math.Abs(effect.X-expectedX) > 1e-6 {
		t.Fatalf("expected effect X %.2f, got %.2f", expectedX, effect.X)
	}
	if math.Abs(effect.Y-expectedY) > 1e-6 {
		t.Fatalf("expected effect Y %.2f, got %.2f", expectedY, effect.Y)
	}

	if !slicesEqual(instance.Colors, expectedColors) {
		t.Fatalf("expected instance colors %v, got %v", expectedColors, instance.Colors)
	}
}

func TestEffectManagerSkeletonQueuesIntents(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	world.obstacles = nil
	attacker := newTestPlayerState("attacker")
	attacker.X = 200
	attacker.Y = 200
	attacker.Facing = FacingRight
	attacker.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(attacker)

	now := time.Now()
	commands := []Command{
		{ActorID: attacker.ID, Type: CommandAction, Action: &ActionCommand{Name: effectTypeAttack}},
		{ActorID: attacker.ID, Type: CommandAction, Action: &ActionCommand{Name: effectTypeFireball}},
	}

	var events struct {
		spawns  []effectcontract.EffectSpawnEvent
		updates []effectcontract.EffectUpdateEvent
		ends    []effectcontract.EffectEndEvent
	}
	collector := func(evt effectcontract.EffectLifecycleEvent) {
		switch e := evt.(type) {
		case effectcontract.EffectSpawnEvent:
			events.spawns = append(events.spawns, e)
		case effectcontract.EffectUpdateEvent:
			events.updates = append(events.updates, e)
		case effectcontract.EffectEndEvent:
			events.ends = append(events.ends, e)
		default:
			t.Fatalf("unexpected effect lifecycle event type %T", e)
		}
	}

	world.Step(1, now, 1.0/float64(tickRate), commands, collector)

	if world.effectManager == nil {
		t.Fatalf("expected effect manager to be instantiated when feature flag enabled")
	}
	if world.effectManager.TotalEnqueued() != len(commands) {
		t.Fatalf("expected %d intents to be enqueued, got %d", len(commands), world.effectManager.TotalEnqueued())
	}
	if world.effectManager.TotalDrained() != world.effectManager.TotalEnqueued() {
		t.Fatalf("expected total drained %d to match enqueued %d", world.effectManager.TotalDrained(), world.effectManager.TotalEnqueued())
	}
	if world.effectManager.PendingIntentCount() != 0 {
		t.Fatalf("expected intent queue to be drained after RunTick, found %d remaining", world.effectManager.PendingIntentCount())
	}
	expectedInstances := len(commands) - 1 // melee ends instantly
	if len(world.effectManager.Instances()) != expectedInstances {
		t.Fatalf("expected %d instances to remain active, found %d", expectedInstances, len(world.effectManager.Instances()))
	}
	if world.effectManager.LastTickProcessed() != effectcontract.Tick(1) {
		t.Fatalf("expected manager to record tick 1, got %d", world.effectManager.LastTickProcessed())
	}

	if len(events.spawns) != len(commands) {
		t.Fatalf("expected %d spawn events, got %d", len(commands), len(events.spawns))
	}
	if len(events.updates) != len(commands) {
		t.Fatalf("expected %d update events, got %d", len(commands), len(events.updates))
	}
	if len(events.ends) != 1 {
		t.Fatalf("expected exactly 1 end event on initial tick, got %d", len(events.ends))
	}

	updatesByID := make(map[string]effectcontract.EffectUpdateEvent)
	for _, update := range events.updates {
		if update.ID == "" {
			t.Fatalf("expected update to include instance id")
		}
		if _, exists := updatesByID[update.ID]; exists {
			t.Fatalf("duplicate update recorded for instance %q", update.ID)
		}
		updatesByID[update.ID] = update
	}

	endsByID := make(map[string]effectcontract.EffectEndEvent)
	for _, end := range events.ends {
		if end.ID == "" {
			t.Fatalf("expected end to include instance id")
		}
		if _, exists := endsByID[end.ID]; exists {
			t.Fatalf("duplicate end recorded for instance %q", end.ID)
		}
		endsByID[end.ID] = end
	}

	lastSeqByID := make(map[string]effectcontract.Seq)
	for i, spawn := range events.spawns {
		var expectedType string
		switch i {
		case 0:
			expectedType = effectTypeAttack
		case 1:
			expectedType = effectTypeFireball
		}
		if spawn.Instance.DefinitionID != expectedType {
			t.Fatalf("spawn %d expected definition %q, got %q", i, expectedType, spawn.Instance.DefinitionID)
		}
		if spawn.Instance.Definition == nil {
			t.Fatalf("expected spawn %d to include definition pointer", i)
		}
		if spawn.Instance.DefinitionID == effectTypeFireball && spawn.Instance.Definition.Delivery != effectcontract.DeliveryKindArea {
			t.Fatalf("expected fireball definition delivery to be %q", effectcontract.DeliveryKindArea)
		}
		update, ok := updatesByID[spawn.Instance.ID]
		if !ok {
			t.Fatalf("expected update event for instance %q", spawn.Instance.ID)
		}
		if update.ID != spawn.Instance.ID {
			t.Fatalf("expected update id %q to match spawn id", update.ID)
		}
		if def := world.effectManager.Definitions()[spawn.Instance.DefinitionID]; def != nil {
			if spawn.Instance.Replication.SendSpawn != def.Client.SendSpawn ||
				spawn.Instance.Replication.SendUpdates != def.Client.SendUpdates ||
				spawn.Instance.Replication.SendEnd != def.Client.SendEnd {
				t.Fatalf("spawn %q replication spec mismatch", spawn.Instance.ID)
			}
		}
		if update.Seq <= spawn.Seq {
			t.Fatalf("expected update seq > spawn seq for instance %q, got spawn=%d update=%d", spawn.Instance.ID, spawn.Seq, update.Seq)
		}
		lastSeqByID[spawn.Instance.ID] = update.Seq
		if end, ok := endsByID[spawn.Instance.ID]; ok {
			if end.Seq <= update.Seq {
				t.Fatalf("expected end seq > update seq for instance %q, got update=%d end=%d", spawn.Instance.ID, update.Seq, end.Seq)
			}
			if end.ID != spawn.Instance.ID {
				t.Fatalf("expected end id %q to match spawn id", end.ID)
			}
			if spawn.Instance.DefinitionID == effectTypeAttack && end.Reason != effectcontract.EndReasonExpired {
				t.Fatalf("expected melee end reason %q, got %q", effectcontract.EndReasonExpired, end.Reason)
			}
			lastSeqByID[spawn.Instance.ID] = end.Seq
		}
	}

	for id, update := range updatesByID {
		if _, hasEnd := endsByID[id]; hasEnd {
			continue
		}
		lastSeq, ok := lastSeqByID[id]
		if !ok {
			continue
		}
		if update.Seq != lastSeq {
			t.Fatalf("expected recorded seq for %q to match update seq, got %d vs %d", id, lastSeq, update.Seq)
		}
	}

	for id, end := range endsByID {
		if end.Seq != lastSeqByID[id] {
			t.Fatalf("expected recorded seq for end %q to match end seq", id)
		}
	}

	for _, eff := range world.effects {
		if eff == nil {
			continue
		}
		if eff.Type == effectTypeAttack && !eff.ContractManaged {
			t.Fatalf("unexpected legacy melee effect when contract pipeline enabled")
		}
	}
}

type effectEventCollector struct {
	spawns  []effectcontract.EffectSpawnEvent
	updates []effectcontract.EffectUpdateEvent
	ends    []effectcontract.EffectEndEvent
}

func (c *effectEventCollector) collect(evt effectcontract.EffectLifecycleEvent) {
	if c == nil {
		return
	}
	switch e := evt.(type) {
	case effectcontract.EffectSpawnEvent:
		c.spawns = append(c.spawns, e)
	case effectcontract.EffectUpdateEvent:
		c.updates = append(c.updates, e)
	case effectcontract.EffectEndEvent:
		c.ends = append(c.ends, e)
	default:
		panic(fmt.Sprintf("unexpected effect lifecycle event type %T", e))
	}
}

func TestContractMeleeEndsInstantly(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	attacker := newTestPlayerState("melee-owner")
	attacker.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(attacker)

	collector := &effectEventCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Now()
	commands := []Command{{
		ActorID: attacker.ID,
		Type:    CommandAction,
		Action:  &ActionCommand{Name: effectTypeAttack},
	}}

	world.Step(1, now, dt, commands, collector.collect)

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 melee spawn, got %d", len(collector.spawns))
	}
	if len(collector.updates) != 1 {
		t.Fatalf("expected 1 melee update, got %d", len(collector.updates))
	}
	if len(collector.ends) != 1 {
		t.Fatalf("expected 1 melee end, got %d", len(collector.ends))
	}

	spawn := collector.spawns[0]
	update := collector.updates[0]
	end := collector.ends[0]

	if spawn.Instance.DefinitionID != effectTypeAttack {
		t.Fatalf("expected melee spawn definition %q, got %q", effectTypeAttack, spawn.Instance.DefinitionID)
	}
	if update.ID != spawn.Instance.ID {
		t.Fatalf("expected update id %q to match spawn id", update.ID)
	}
	if end.ID != spawn.Instance.ID {
		t.Fatalf("expected end id %q to match spawn id", end.ID)
	}
	if !(spawn.Seq < update.Seq && update.Seq < end.Seq) {
		t.Fatalf("expected spawn < update < end seq, got %d %d %d", spawn.Seq, update.Seq, end.Seq)
	}
	if end.Reason != effectcontract.EndReasonExpired {
		t.Fatalf("expected melee end reason %q, got %q", effectcontract.EndReasonExpired, end.Reason)
	}
	if _, exists := world.effectManager.Instances()[spawn.Instance.ID]; exists {
		t.Fatalf("expected melee instance %q to be removed after instant end", spawn.Instance.ID)
	}
}

func TestContractProjectileEndsByDuration(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	world.obstacles = nil
	world.npcs = make(map[string]*npcState)
	attacker := newTestPlayerState("projectile-owner")
	attacker.Cooldowns = make(map[string]time.Time)
	attacker.X = worldWidth / 2
	attacker.Y = worldHeight / 2
	world.AddPlayer(attacker)

	if def := world.effectManager.Definitions()[effectTypeFireball]; def != nil {
		def.LifetimeTicks = 3
		def.End = effectcontract.EndPolicy{Kind: effectcontract.EndDuration}
	}

	collector := &effectEventCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Now()

	for tick := uint64(1); tick <= 4; tick++ {
		cmds := []Command{}
		if tick == 1 {
			cmds = append(cmds, Command{
				ActorID: attacker.ID,
				Type:    CommandAction,
				Action:  &ActionCommand{Name: effectTypeFireball},
			})
		}
		world.Step(tick, now.Add(time.Duration(tick-1)*time.Millisecond), dt, cmds, collector.collect)
	}

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 projectile spawn, got %d", len(collector.spawns))
	}
	if len(collector.ends) != 1 {
		t.Fatalf("expected projectile to end once, got %d", len(collector.ends))
	}
	if len(collector.updates) != 3 {
		t.Fatalf("expected 3 projectile updates before end, got %d", len(collector.updates))
	}

	spawn := collector.spawns[0]
	if spawn.Instance.DefinitionID != effectTypeFireball {
		t.Fatalf("expected projectile definition %q, got %q", effectTypeFireball, spawn.Instance.DefinitionID)
	}

	seqs := []effectcontract.Seq{spawn.Seq}
	for _, update := range collector.updates {
		if update.ID != spawn.Instance.ID {
			t.Fatalf("expected projectile update to reference %q, got %q", spawn.Instance.ID, update.ID)
		}
		seqs = append(seqs, update.Seq)
	}
	end := collector.ends[0]
	if end.ID != spawn.Instance.ID {
		t.Fatalf("expected projectile end to reference %q, got %q", spawn.Instance.ID, end.ID)
	}
	if end.Reason != effectcontract.EndReasonExpired {
		t.Fatalf("expected projectile end reason %q, got %q", effectcontract.EndReasonExpired, end.Reason)
	}
	seqs = append(seqs, end.Seq)

	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Fatalf("expected strictly increasing seq, got %v", seqs)
		}
	}
	if _, exists := world.effectManager.Instances()[spawn.Instance.ID]; exists {
		t.Fatalf("expected projectile instance %q to be removed after duration end", spawn.Instance.ID)
	}
}

func TestContractOwnerLostConditionEndsEffect(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	owner := newTestPlayerState("anchor-owner")
	owner.Cooldowns = make(map[string]time.Time)
	world.AddPlayer(owner)

	const anchorType = "anchor-effect"
	world.effectManager.Definitions()[anchorType] = &effectcontract.EffectDefinition{
		TypeID:        anchorType,
		Delivery:      effectcontract.DeliveryKindTarget,
		Shape:         effectcontract.GeometryShapeCircle,
		Motion:        effectcontract.MotionKindNone,
		Impact:        effectcontract.ImpactPolicyFirstHit,
		LifetimeTicks: 10,
		Client: effectcontract.ReplicationSpec{
			SendSpawn:   true,
			SendUpdates: true,
			SendEnd:     true,
		},
		End: effectcontract.EndPolicy{Kind: effectcontract.EndCondition, Conditions: effectcontract.EndConditions{OnOwnerLost: true}},
	}

	collector := &effectEventCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Now()

	world.effectManager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:       anchorType,
		TypeID:        anchorType,
		Delivery:      effectcontract.DeliveryKindTarget,
		SourceActorID: owner.ID,
		TargetActorID: owner.ID,
		Geometry:      effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
	})

	world.Step(1, now, dt, nil, collector.collect)

	if len(collector.spawns) != 1 {
		t.Fatalf("expected anchor spawn, got %d", len(collector.spawns))
	}
	spawn := collector.spawns[0]
	anchorID := spawn.Instance.ID

	world.RemovePlayer(owner.ID)

	world.Step(2, now.Add(time.Millisecond), dt, nil, collector.collect)

	if len(collector.ends) == 0 {
		t.Fatalf("expected anchor end event when owner lost")
	}

	end := collector.ends[len(collector.ends)-1]
	if end.ID != anchorID {
		t.Fatalf("expected anchor end id %q, got %q", anchorID, end.ID)
	}
	if end.Reason != effectcontract.EndReasonOwnerLost {
		t.Fatalf("expected owner-lost reason %q, got %q", effectcontract.EndReasonOwnerLost, end.Reason)
	}
	if _, exists := world.effectManager.Instances()[anchorID]; exists {
		t.Fatalf("expected anchor instance %q to be removed after owner lost", anchorID)
	}
}

func TestContractReplicationOffSkipsUpdates(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	const spawnOnlyType = "spawn-only"
	world.effectManager.Definitions()[spawnOnlyType] = &effectcontract.EffectDefinition{
		TypeID:        spawnOnlyType,
		Delivery:      effectcontract.DeliveryKindArea,
		Shape:         effectcontract.GeometryShapeCircle,
		Motion:        effectcontract.MotionKindNone,
		Impact:        effectcontract.ImpactPolicyFirstHit,
		LifetimeTicks: 2,
		Client: effectcontract.ReplicationSpec{
			SendSpawn:   true,
			SendUpdates: false,
			SendEnd:     true,
		},
		End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
	}

	collector := &effectEventCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Now()

	world.effectManager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:  spawnOnlyType,
		TypeID:   spawnOnlyType,
		Delivery: effectcontract.DeliveryKindArea,
		Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
	})

	for tick := uint64(1); tick <= 3; tick++ {
		world.Step(tick, now.Add(time.Duration(tick-1)*time.Millisecond), dt, nil, collector.collect)
	}

	if len(collector.spawns) != 1 {
		t.Fatalf("expected spawn-only definition to emit a single spawn, got %d", len(collector.spawns))
	}
	if len(collector.updates) != 0 {
		t.Fatalf("expected no updates when replication disabled, got %d", len(collector.updates))
	}
	if len(collector.ends) != 1 {
		t.Fatalf("expected spawn-only definition to emit exactly one end, got %d", len(collector.ends))
	}
}

func TestContractSeqMonotonicAcrossTicks(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	const seqType = "seq-effect"
	world.effectManager.Definitions()[seqType] = &effectcontract.EffectDefinition{
		TypeID:        seqType,
		Delivery:      effectcontract.DeliveryKindArea,
		Shape:         effectcontract.GeometryShapeCircle,
		Motion:        effectcontract.MotionKindNone,
		Impact:        effectcontract.ImpactPolicyFirstHit,
		LifetimeTicks: 4,
		Client: effectcontract.ReplicationSpec{
			SendSpawn:   true,
			SendUpdates: true,
			SendEnd:     true,
		},
		End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
	}

	collector := &effectEventCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Now()

	world.effectManager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:  seqType,
		TypeID:   seqType,
		Delivery: effectcontract.DeliveryKindArea,
		Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
	})

	for tick := uint64(1); tick <= 5; tick++ {
		world.Step(tick, now.Add(time.Duration(tick-1)*time.Millisecond), dt, nil, collector.collect)
	}

	if len(collector.spawns) != 1 {
		t.Fatalf("expected sequence test to emit one spawn, got %d", len(collector.spawns))
	}
	if len(collector.ends) != 1 {
		t.Fatalf("expected sequence test to emit one end, got %d", len(collector.ends))
	}
	if len(collector.updates) != 4 {
		t.Fatalf("expected sequence test to emit 4 updates, got %d", len(collector.updates))
	}

	spawn := collector.spawns[0]
	seqValues := []effectcontract.Seq{spawn.Seq}
	for _, update := range collector.updates {
		if update.ID != spawn.Instance.ID {
			t.Fatalf("expected update to reference %q, got %q", spawn.Instance.ID, update.ID)
		}
		seqValues = append(seqValues, update.Seq)
	}
	end := collector.ends[0]
	if end.ID != spawn.Instance.ID {
		t.Fatalf("expected end to reference %q, got %q", spawn.Instance.ID, end.ID)
	}
	seqValues = append(seqValues, end.Seq)

	for i := 1; i < len(seqValues); i++ {
		if seqValues[i] <= seqValues[i-1] {
			t.Fatalf("expected strict monotonic sequence, got %v", seqValues)
		}
	}
}

func TestFireballDealsDamageOnHit(t *testing.T) {
	hub := newHubWithFullWorld()
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
	casterState.LastHeartbeat = now
	casterState.Cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = casterState

	victimState := newTestPlayerState(targetID)
	victimState.X = shooterX + spawnOffset + travel/2
	victimState.Y = shooterY
	victimState.Facing = FacingLeft
	victimState.LastHeartbeat = now
	hub.world.players[targetID] = victimState

	if _, ok, _ := hub.HandleAction(shooterID, effectTypeFireball); !ok {
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
	expected := baselinePlayerMaxHealth - fireballDamage - lavaDamagePerSecond*burningTickInterval.Seconds()
	if math.Abs(target.Health-expected) > 1e-6 {
		hub.mu.Unlock()
		t.Fatalf("expected target health %.1f, got %.1f", expected, target.Health)
	}
	if target.StatusEffects == nil || target.StatusEffects[StatusEffectBurning] == nil {
		hub.mu.Unlock()
		t.Fatalf("expected fireball hit to apply burning status effect")
	}
	hub.mu.Unlock()
}

func TestHealthDeltaHealingClampsToMax(t *testing.T) {
	hub := newHubWithFullWorld()
	playerID := "patient"
	state := newTestPlayerState(playerID)
	state.X = 160
	state.Y = 160
	state.Health = baselinePlayerMaxHealth - 30
	state.LastHeartbeat = time.Now()
	hub.world.players[playerID] = state

	heal := &effectState{Type: effectTypeAttack, Owner: "healer", Params: map[string]float64{"healthDelta": 50}}

	hub.world.invokePlayerHitCallback(heal, state, time.Now())

	if math.Abs(state.Health-baselinePlayerMaxHealth) > 1e-6 {
		t.Fatalf("expected healing to clamp to max %.1f, got %.1f", baselinePlayerMaxHealth, state.Health)
	}
}

func TestHealthDamageClampsToZero(t *testing.T) {
	hub := newHubWithFullWorld()
	playerID := "fragile"
	state := newTestPlayerState(playerID)
	state.X = 180
	state.Y = 180
	state.Health = 5
	state.LastHeartbeat = time.Now()
	hub.world.players[playerID] = state

	blast := &effectState{Type: effectTypeAttack, Owner: "boom", Params: map[string]float64{"healthDelta": -50}}

	hub.world.invokePlayerHitCallback(blast, state, time.Now())

	if state.Health != 0 {
		t.Fatalf("expected damage to clamp to zero health, got %.1f", state.Health)
	}
}

func TestFireballExpiresOnObstacleCollision(t *testing.T) {
	hub := newHubWithFullWorld()
	now := time.Now()

	shooterID := "caster"
	casterState := newTestPlayerState(shooterID)
	casterState.X = 200
	casterState.Y = 200
	casterState.Facing = FacingRight
	casterState.LastHeartbeat = now
	casterState.Cooldowns = make(map[string]time.Time)
	hub.world.players[shooterID] = casterState

	hub.world.obstacles = []Obstacle{{
		ID:     "wall",
		X:      260,
		Y:      180,
		Width:  40,
		Height: 40,
	}}

	if _, ok, _ := hub.HandleAction(shooterID, effectTypeFireball); !ok {
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

func TestConsoleDropAndPickupSelf(t *testing.T) {
	hub := newHubWithFullWorld()
	playerID := "player-drop-self"
	player := newTestPlayerState(playerID)
	hub.world.AddPlayer(player)
	if err := hub.world.MutateInventory(playerID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 20})
		return err
	}); err != nil {
		t.Fatalf("failed to seed gold: %v", err)
	}

	ack, ok := hub.HandleConsoleCommand(playerID, "drop_gold", 10)
	if !ok {
		t.Fatalf("expected drop command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("expected drop ack to be ok, got %+v", ack)
	}
	if ack.Qty != 10 {
		t.Fatalf("expected drop ack qty 10, got %d", ack.Qty)
	}
	if remaining := player.Inventory.QuantityOf(ItemTypeGold); remaining != 10 {
		t.Fatalf("expected 10 gold remaining, got %d", remaining)
	}
	items := hub.world.GroundItemsSnapshot()
	if len(items) != 1 {
		t.Fatalf("expected exactly one ground item, got %d", len(items))
	}
	if ItemType(items[0].Type) != ItemTypeGold {
		t.Fatalf("expected ground item type gold, got %s", items[0].Type)
	}
	if items[0].Qty != 10 {
		t.Fatalf("expected ground stack of 10, got %d", items[0].Qty)
	}

	ack, ok = hub.HandleConsoleCommand(playerID, "pickup_gold", 0)
	if !ok {
		t.Fatalf("expected pickup command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("expected pickup ack to be ok, got %+v", ack)
	}
	if ack.Qty != 10 {
		t.Fatalf("expected pickup ack qty 10, got %d", ack.Qty)
	}
	if total := player.Inventory.QuantityOf(ItemTypeGold); total != 20 {
		t.Fatalf("expected inventory to restore 20 gold, got %d", total)
	}
	if remaining := hub.world.GroundItemsSnapshot(); len(remaining) != 0 {
		t.Fatalf("expected no ground items after pickup, got %d", len(remaining))
	}
}

func TestConsolePickupTransfersBetweenPlayers(t *testing.T) {
	hub := newHubWithFullWorld()
	dropperID := "player-dropper"
	pickerID := "player-picker"
	dropper := newTestPlayerState(dropperID)
	picker := newTestPlayerState(pickerID)
	hub.world.AddPlayer(dropper)
	hub.world.AddPlayer(picker)
	if err := hub.world.MutateInventory(dropperID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 15})
		return err
	}); err != nil {
		t.Fatalf("failed to seed dropper gold: %v", err)
	}

	ack, ok := hub.HandleConsoleCommand(dropperID, "drop_gold", 15)
	if !ok || ack.Status != "ok" {
		t.Fatalf("expected drop to succeed, ack=%+v", ack)
	}
	ack, ok = hub.HandleConsoleCommand(pickerID, "pickup_gold", 0)
	if !ok || ack.Status != "ok" {
		t.Fatalf("expected pickup to succeed, ack=%+v", ack)
	}
	if qty := picker.Inventory.QuantityOf(ItemTypeGold); qty != 15 {
		t.Fatalf("expected picker to have 15 gold, got %d", qty)
	}
	if qty := dropper.Inventory.QuantityOf(ItemTypeGold); qty != 0 {
		t.Fatalf("expected dropper to have 0 gold, got %d", qty)
	}
	if items := hub.world.GroundItemsSnapshot(); len(items) != 0 {
		t.Fatalf("expected ground to be empty, got %d items", len(items))
	}
}

func TestConsolePickupRaceHonoursFirstCollector(t *testing.T) {
	hub := newHubWithFullWorld()
	dropper := newTestPlayerState("player-race-a")
	contender := newTestPlayerState("player-race-b")
	hub.world.AddPlayer(dropper)
	hub.world.AddPlayer(contender)
	if err := hub.world.MutateInventory(dropper.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 8})
		return err
	}); err != nil {
		t.Fatalf("failed to seed dropper gold: %v", err)
	}

	dropAck, _ := hub.HandleConsoleCommand(dropper.ID, "drop_gold", 8)
	if dropAck.Status != "ok" {
		t.Fatalf("expected drop success, got %+v", dropAck)
	}

	firstAck, _ := hub.HandleConsoleCommand(contender.ID, "pickup_gold", 0)
	if firstAck.Status != "ok" {
		t.Fatalf("expected first pickup to succeed, got %+v", firstAck)
	}
	secondAck, _ := hub.HandleConsoleCommand(dropper.ID, "pickup_gold", 0)
	if secondAck.Status != "error" || secondAck.Reason != "not_found" {
		t.Fatalf("expected second pickup to fail with not_found, got %+v", secondAck)
	}
}

func TestConsoleDropGoldBroadcastsGroundItemsFromSimEngine(t *testing.T) {
	hub := newHubWithFullWorld()
	player := newTestPlayerState("player-console-engine")
	hub.world.AddPlayer(player)
	if err := hub.world.MutateInventory(player.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 12})
		return err
	}); err != nil {
		t.Fatalf("failed to seed gold: %v", err)
	}

	snapshot := sim.Snapshot{GroundItems: []sim.GroundItem{{
		ID:             "engine-ground",
		Type:           string(ItemTypeGold),
		FungibilityKey: "engine-fungibility",
		X:              42,
		Y:              24,
		Qty:            3,
	}}}
	engine := newRecordingSimEngine(snapshot)
	t.Cleanup(engine.AllowFurtherSnapshots)
	hub.engine = engine

	ack, ok := hub.HandleConsoleCommand(player.ID, "drop_gold", 4)
	if !ok {
		t.Fatalf("expected drop command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("expected drop ack to be ok, got %+v", ack)
	}
	if ack.StackID == snapshot.GroundItems[0].ID {
		t.Fatalf("expected stack id to come from world, got engine sentinel %q", ack.StackID)
	}

	if calls := engine.Calls(); calls != 1 {
		t.Fatalf("expected engine snapshot to be consulted exactly once, got %d", calls)
	}

	engine.AllowFurtherSnapshots()
}

func TestConsolePickupGoldBroadcastsGroundItemsFromSimEngine(t *testing.T) {
	hub := newHubWithFullWorld()
	player := newTestPlayerState("player-console-pickup-engine")
	hub.world.AddPlayer(player)
	stack := hub.world.upsertGroundItem(&player.ActorState, ItemStack{Type: ItemTypeGold, Quantity: 6}, "manual")
	if stack == nil {
		t.Fatalf("expected ground item to seed for pickup test")
	}
	originalQty := stack.Qty

	snapshot := sim.Snapshot{GroundItems: []sim.GroundItem{{
		ID:             "engine-ground",
		Type:           string(ItemTypeGold),
		FungibilityKey: "engine-fungibility",
		X:              11,
		Y:              7,
		Qty:            2,
	}}}
	engine := newRecordingSimEngine(snapshot)
	t.Cleanup(engine.AllowFurtherSnapshots)
	hub.engine = engine

	ack, ok := hub.HandleConsoleCommand(player.ID, "pickup_gold", 0)
	if !ok {
		t.Fatalf("expected pickup command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("expected pickup ack to be ok, got %+v", ack)
	}
	if ack.Qty != originalQty {
		t.Fatalf("expected pickup ack qty %d, got %d", originalQty, ack.Qty)
	}
	if ack.StackID != stack.ID {
		t.Fatalf("expected ack stack id %q, got %q", stack.ID, ack.StackID)
	}
	if total := player.Inventory.QuantityOf(ItemTypeGold); total != originalQty {
		t.Fatalf("expected inventory to contain %d gold, got %d", originalQty, total)
	}

	if calls := engine.Calls(); calls != 1 {
		t.Fatalf("expected engine snapshot to be consulted exactly once, got %d", calls)
	}

	engine.AllowFurtherSnapshots()
}

func TestRunSimulationBroadcastsGroundItemsFromSimEngine(t *testing.T) {
	hub := newHubWithFullWorld()

	player := newTestPlayerState("player-sim-broadcast")
	hub.world.AddPlayer(player)
	if stack := hub.world.upsertGroundItem(&player.ActorState, ItemStack{Type: ItemTypeGold, Quantity: 9}, "world"); stack == nil {
		t.Fatalf("expected world ground item to seed test scenario")
	}

	snapshot := sim.Snapshot{GroundItems: []sim.GroundItem{{
		ID:             "engine-ground",
		Type:           string(ItemTypeGold),
		FungibilityKey: "engine-fungibility",
		X:              12,
		Y:              34,
		Qty:            5,
	}}}
	engine := newRecordingSimEngine(snapshot)
	hub.engine = engine
	t.Cleanup(engine.AllowFurtherSnapshots)

	players, npcs, triggers, groundItems, toClose := hub.advance(time.Now(), 1.0/float64(tickRate))
	if len(toClose) != 0 {
		t.Fatalf("expected no subscribers to close during advance, got %d", len(toClose))
	}

	includeSnapshot := hub.shouldIncludeSnapshot()
	engine.AllowFurtherSnapshots()
	data, _, err := marshalStateLegacy(hub, players, npcs, triggers, groundItems, true, includeSnapshot)
	if err != nil {
		t.Fatalf("failed to marshal state payload: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode broadcast payload: %v", err)
	}

	if len(msg.GroundItems) != len(snapshot.GroundItems) {
		t.Fatalf("expected %d ground items from engine snapshot, got %d", len(snapshot.GroundItems), len(msg.GroundItems))
	}
	if msg.GroundItems[0].ID != snapshot.GroundItems[0].ID {
		t.Fatalf("expected broadcast ground item id %q, got %q", snapshot.GroundItems[0].ID, msg.GroundItems[0].ID)
	}

	if calls := engine.Calls(); calls < 1 {
		t.Fatalf("expected engine snapshot to be consulted at least once, got %d", calls)
	}
}

func TestDeathDropsPlayerInventory(t *testing.T) {
	hub := newHubWithFullWorld()
	attacker := newTestPlayerState("player-attacker")
	victim := newTestPlayerState("player-victim")
	hub.world.AddPlayer(attacker)
	hub.world.AddPlayer(victim)
	if err := hub.world.MutateInventory(victim.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 37})
		return err
	}); err != nil {
		t.Fatalf("failed to seed victim gold: %v", err)
	}
	if err := hub.world.MutateInventory(victim.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 2})
		return err
	}); err != nil {
		t.Fatalf("failed to seed victim potions: %v", err)
	}

	now := time.Now()
	damage := victim.Health + 5
	eff := &effectState{Type: effectTypeAttack, Owner: attacker.ID, Params: map[string]float64{"healthDelta": -damage}}
	hub.world.invokePlayerHitCallback(eff, victim, now)

	if victim.Health > 0 {
		t.Fatalf("expected victim health to reach zero, got %.2f", victim.Health)
	}
	if qty := victim.Inventory.QuantityOf(ItemTypeGold); qty != 0 {
		t.Fatalf("expected victim gold to be zero after death, got %d", qty)
	}
	if qty := victim.Inventory.QuantityOf(ItemTypeHealthPotion); qty != 0 {
		t.Fatalf("expected victim potions to be zero after death, got %d", qty)
	}
	items := hub.world.GroundItemsSnapshot()
	if len(items) != 2 {
		t.Fatalf("expected two ground drops after death, got %d", len(items))
	}
	totals := map[ItemType]int{}
	for _, item := range items {
		totals[ItemType(item.Type)] += item.Qty
	}
	if totals[ItemTypeGold] != 37 {
		t.Fatalf("expected ground gold to equal 37, got %d", totals[ItemTypeGold])
	}
	if totals[ItemTypeHealthPotion] != 2 {
		t.Fatalf("expected ground potion quantity 2, got %d", totals[ItemTypeHealthPotion])
	}
}

func TestEquipConsoleCommandUpdatesStats(t *testing.T) {
	hub := newHubWithFullWorld()

	playerID := "player-equip"
	player := newTestPlayerState(playerID)
	hub.world.AddPlayer(player)

	if err := hub.world.MutateInventory(playerID, func(inv *Inventory) error {
		inv.Slots = nil
		_, err := inv.AddStack(ItemStack{Type: ItemTypeIronDagger, Quantity: 1})
		return err
	}); err != nil {
		t.Fatalf("failed to seed dagger: %v", err)
	}

	baseMight := player.Stats.GetTotal(stats.StatMight)
	baseHealth := player.Stats.GetDerived(stats.DerivedMaxHealth)

	ack, handled := hub.HandleConsoleCommand(playerID, "equip_slot", 0)
	if !handled {
		t.Fatalf("expected equip command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("expected equip success, got %+v", ack)
	}
	if ack.Slot != string(EquipSlotMainHand) {
		t.Fatalf("expected equip slot %s, got %s", EquipSlotMainHand, ack.Slot)
	}

	if player.Stats.GetTotal(stats.StatMight) <= baseMight {
		t.Fatalf("expected might to increase after equip")
	}
	equippedHealth := player.Stats.GetDerived(stats.DerivedMaxHealth)
	if equippedHealth <= baseHealth {
		t.Fatalf("expected max health to increase, got %.2f <= %.2f", equippedHealth, baseHealth)
	}
	if _, ok := player.Equipment.Get(EquipSlotMainHand); !ok {
		t.Fatalf("expected main hand to be occupied after equip")
	}
	if qty := player.Inventory.QuantityOf(ItemTypeIronDagger); qty != 0 {
		t.Fatalf("expected inventory to consume dagger, remaining %d", qty)
	}

	unequipAck, handled := hub.HandleConsoleCommand(playerID, "unequip_slot", 0)
	if !handled {
		t.Fatalf("expected unequip command to be handled")
	}
	if unequipAck.Status != "ok" {
		t.Fatalf("expected unequip success, got %+v", unequipAck)
	}
	if qty := player.Inventory.QuantityOf(ItemTypeIronDagger); qty != 1 {
		t.Fatalf("expected dagger returned to inventory, got %d", qty)
	}
	player.Stats.Resolve(hub.world.currentTick)
	if player.Stats.GetTotal(stats.StatMight) != baseMight {
		t.Fatalf("expected might to return to baseline %.2f, got %.2f", baseMight, player.Stats.GetTotal(stats.StatMight))
	}
	restoredHealth := player.Stats.GetDerived(stats.DerivedMaxHealth)
	if math.Abs(restoredHealth-baseHealth) > 1e-6 {
		t.Fatalf("expected health to return to baseline %.2f, got %.2f", baseHealth, restoredHealth)
	}
}

func TestDeathDropsNPCInventory(t *testing.T) {
	hub := newHubWithFullWorld()
	attacker := newTestPlayerState("player-vs-npc")
	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-target", X: 300, Y: 300, Health: 25, MaxHealth: 25, Inventory: NewInventory()}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin), Type: NPCTypeGoblin}
	if _, err := npc.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 12}); err != nil {
		t.Fatalf("failed to seed npc gold: %v", err)
	}
	if _, err := npc.Inventory.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 1}); err != nil {
		t.Fatalf("failed to seed npc potion: %v", err)
	}
	hub.world.AddPlayer(attacker)
	hub.world.npcs[npc.ID] = npc

	now := time.Now()
	eff := &effectState{Type: effectTypeAttack, Owner: attacker.ID, Params: map[string]float64{"healthDelta": -(npc.Health + 5)}}
	hub.world.invokeNPCHitCallback(eff, npc, now)

	if _, exists := hub.world.npcs[npc.ID]; exists {
		t.Fatalf("expected npc to be removed after death")
	}
	items := hub.world.GroundItemsSnapshot()
	if len(items) != 2 {
		t.Fatalf("expected two ground drops for npc death, got %d", len(items))
	}
	totals := map[ItemType]int{}
	for _, item := range items {
		totals[ItemType(item.Type)] += item.Qty
	}
	if totals[ItemTypeGold] != 12 {
		t.Fatalf("expected npc drop of 12 gold, got %d", totals[ItemTypeGold])
	}
	if totals[ItemTypeHealthPotion] != 1 {
		t.Fatalf("expected npc drop of 1 potion, got %d", totals[ItemTypeHealthPotion])
	}
}

func TestRatDropsTailOnDeath(t *testing.T) {
	hub := newHubWithFullWorld()
	attacker := newTestPlayerState("player-rat-hunter")
	hub.world.AddPlayer(attacker)
	hub.world.spawnRatAt(220, 220)

	var rat *npcState
	for _, npc := range hub.world.npcs {
		if npc.Type == NPCTypeRat {
			rat = npc
			break
		}
	}
	if rat == nil {
		t.Fatalf("expected rat to spawn")
	}
	if qty := rat.Inventory.QuantityOf(ItemTypeRatTail); qty != 1 {
		t.Fatalf("expected rat to start with 1 tail, got %d", qty)
	}

	now := time.Now()
	eff := &effectState{Type: effectTypeAttack, Owner: attacker.ID, Params: map[string]float64{"healthDelta": -(rat.Health + 5)}}
	hub.world.invokeNPCHitCallback(eff, rat, now)

	if _, exists := hub.world.npcs[rat.ID]; exists {
		t.Fatalf("expected rat to be removed after death")
	}

	items := hub.world.GroundItemsSnapshot()
	if len(items) != 1 {
		t.Fatalf("expected one ground drop for rat tail, got %d", len(items))
	}
	drop := items[0]
	if ItemType(drop.Type) != ItemTypeRatTail {
		t.Fatalf("expected rat tail drop, got %s", drop.Type)
	}
	if drop.Qty != 1 {
		t.Fatalf("expected rat tail quantity 1, got %d", drop.Qty)
	}
}

func TestGroundGoldMergesOnSameTile(t *testing.T) {
	hub := newHubWithFullWorld()
	player := newTestPlayerState("player-merge")
	hub.world.AddPlayer(player)
	if err := hub.world.MutateInventory(player.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 50})
		return err
	}); err != nil {
		t.Fatalf("failed to seed gold: %v", err)
	}

	if ack, _ := hub.HandleConsoleCommand(player.ID, "drop_gold", 5); ack.Status != "ok" {
		t.Fatalf("expected first drop to succeed")
	}
	if ack, _ := hub.HandleConsoleCommand(player.ID, "drop_gold", 7); ack.Status != "ok" {
		t.Fatalf("expected second drop to succeed")
	}
	items := hub.world.GroundItemsSnapshot()
	if len(items) != 1 {
		t.Fatalf("expected merged stack, got %d items", len(items))
	}
	if ItemType(items[0].Type) != ItemTypeGold {
		t.Fatalf("expected merged stack type gold, got %s", items[0].Type)
	}
	if items[0].Qty != 12 {
		t.Fatalf("expected merged quantity 12, got %d", items[0].Qty)
	}
}

func TestConsoleDropValidation(t *testing.T) {
	hub := newHubWithFullWorld()
	player := newTestPlayerState("player-invalid")
	hub.world.AddPlayer(player)
	if err := hub.world.MutateInventory(player.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 5})
		return err
	}); err != nil {
		t.Fatalf("failed to seed gold: %v", err)
	}

	ack, _ := hub.HandleConsoleCommand(player.ID, "drop_gold", 0)
	if ack.Status != "error" || ack.Reason != "invalid_quantity" {
		t.Fatalf("expected invalid quantity error, got %+v", ack)
	}
	ack, _ = hub.HandleConsoleCommand(player.ID, "drop_gold", 10)
	if ack.Status != "error" || ack.Reason != "insufficient_gold" {
		t.Fatalf("expected insufficient_gold error, got %+v", ack)
	}
}

func TestConsolePickupOutOfRange(t *testing.T) {
	hub := newHubWithFullWorld()
	dropper := newTestPlayerState("player-out-range-a")
	picker := newTestPlayerState("player-out-range-b")
	hub.world.AddPlayer(dropper)
	hub.world.AddPlayer(picker)
	if err := hub.world.MutateInventory(dropper.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 6})
		return err
	}); err != nil {
		t.Fatalf("failed to seed dropper gold: %v", err)
	}

	if ack, _ := hub.HandleConsoleCommand(dropper.ID, "drop_gold", 6); ack.Status != "ok" {
		t.Fatalf("expected drop to succeed, got %+v", ack)
	}

	hub.mu.Lock()
	picker.X = dropper.X + groundPickupRadius*2
	picker.Y = dropper.Y + groundPickupRadius*2
	hub.mu.Unlock()

	ack, _ := hub.HandleConsoleCommand(picker.ID, "pickup_gold", 0)
	if ack.Status != "error" || ack.Reason != "out_of_range" {
		t.Fatalf("expected out_of_range error, got %+v", ack)
	}
}

func TestMarshalStateCapturesResubscribeBaselinesFromSnapshot(t *testing.T) {
	hub := newHub()
	player := newTestPlayerState("resubscribe-baseline")
	player.X = 12
	player.Y = -7
	player.IntentX = 0.5
	player.IntentY = -0.25
	hub.world.AddPlayer(player)

	if _, _, err := hub.marshalState(nil, nil, nil, nil, false, true); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	baselines := hub.resubscribeBaselines
	if baselines == nil {
		t.Fatalf("expected resubscribe baselines to be captured")
	}

	view, ok := baselines[player.ID]
	if !ok {
		t.Fatalf("expected baseline entry for %q", player.ID)
	}

	if view.Player.Actor.ID != player.ID {
		t.Fatalf("expected baseline actor id %q, got %q", player.ID, view.Player.Actor.ID)
	}
	if math.Abs(view.Player.Actor.X-player.X) > 1e-6 || math.Abs(view.Player.Actor.Y-player.Y) > 1e-6 {
		t.Fatalf("expected baseline coordinates (%.1f, %.1f), got (%.1f, %.1f)", player.X, player.Y, view.Player.Actor.X, view.Player.Actor.Y)
	}
	if math.Abs(view.IntentDX-player.IntentX) > 1e-6 || math.Abs(view.IntentDY-player.IntentY) > 1e-6 {
		t.Fatalf("expected baseline intents (%.2f, %.2f), got (%.2f, %.2f)", player.IntentX, player.IntentY, view.IntentDX, view.IntentDY)
	}
}

func TestPlayerViewsFromSimSnapshotCopiesIntent(t *testing.T) {
	snapshot := sim.Snapshot{
		Players: []sim.Player{{
			Actor: sim.Actor{
				ID: "player-1",
				Inventory: itemspkg.InventoryFromSimSlots([]sim.InventorySlot{{
					Slot: 1,
					Item: sim.ItemStack{Type: sim.ItemType("gold"), Quantity: 3},
				}}),
			},
			IntentDX: 0.5,
			IntentDY: -0.25,
		}},
	}

	views := playerViewsFromSimSnapshot(snapshot)
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	view, ok := views["player-1"]
	if !ok {
		t.Fatalf("expected view for player-1")
	}
	if math.Abs(view.IntentDX-0.5) > 1e-6 || math.Abs(view.IntentDY-(-0.25)) > 1e-6 {
		t.Fatalf("expected intents (0.5,-0.25), got (%.2f, %.2f)", view.IntentDX, view.IntentDY)
	}
	if view.Player.Actor.ID != "player-1" {
		t.Fatalf("expected actor id player-1, got %q", view.Player.Actor.ID)
	}
	if got := view.Player.Inventory.Slots[0].Item.Quantity; got != 3 {
		t.Fatalf("expected inventory quantity 3, got %d", got)
	}

	snapshot.Players[0].IntentDX = 1
	snapshot.Players[0].Inventory.Slots[0].Item.Quantity = 99
	if math.Abs(view.IntentDX-0.5) > 1e-6 {
		t.Fatalf("expected snapshot mutation to not affect captured view, got %.2f", view.IntentDX)
	}
	if got := view.Player.Inventory.Slots[0].Item.Quantity; got != 3 {
		t.Fatalf("expected inventory quantity to remain 3, got %d", got)
	}
}

func TestMarshalStateAppliesPlayerPatchesToResubscribeBaselines(t *testing.T) {
	hub := newHub()
	player := newTestPlayerState("resubscribe-patch")
	hub.world.AddPlayer(player)

	if _, _, err := hub.marshalState(nil, nil, nil, nil, false, true); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	hub.world.SetPosition(player.ID, 40, -15)
	hub.world.SetIntent(player.ID, -0.75, 0.25)

	if _, _, err := hub.marshalState(nil, nil, nil, nil, true, false); err != nil {
		t.Fatalf("marshalState returned error during delta: %v", err)
	}

	view, ok := hub.resubscribeBaselines[player.ID]
	if !ok {
		t.Fatalf("expected baseline entry for %q", player.ID)
	}
	if math.Abs(view.Player.Actor.X-40) > 1e-6 || math.Abs(view.Player.Actor.Y-(-15)) > 1e-6 {
		t.Fatalf("expected baseline position (40,-15), got (%.1f, %.1f)", view.Player.Actor.X, view.Player.Actor.Y)
	}
	if math.Abs(view.IntentDX-(-0.75)) > 1e-6 || math.Abs(view.IntentDY-0.25) > 1e-6 {
		t.Fatalf("expected baseline intent (-0.75,0.25), got (%.2f, %.2f)", view.IntentDX, view.IntentDY)
	}
}

func TestMarshalStateRefreshesResubscribeBaselinesFromSnapshotOnPatchFailure(t *testing.T) {
	hub := newHub()
	playerID := "resubscribe-fallback"
	player := newTestPlayerState(playerID)
	hub.world.AddPlayer(player)

	snapshot := sim.Snapshot{
		Players: []sim.Player{{
			Actor:    sim.Actor{ID: playerID, X: 42, Y: -17},
			IntentDX: 0.5,
			IntentDY: -0.25,
		}},
	}
	engine := newRecordingSimEngine(snapshot)
	engine.SetSnapshotPatches([]sim.Patch{{
		Kind:     sim.PatchPlayerIntent,
		EntityID: "missing-entity",
		Payload:  sim.PlayerIntentPayload{DX: 0.75, DY: 0.25},
	}})
	hub.engine = engine

	if _, _, err := hub.marshalState(nil, nil, nil, nil, false, true); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	engine.AllowFurtherSnapshots()

	hub.world.SetPosition(playerID, -99, 33)
	hub.world.SetIntent(playerID, 1.0, -1.0)

	if _, _, err := hub.marshalState(nil, nil, nil, nil, false, false); err != nil {
		t.Fatalf("marshalState returned error during delta: %v", err)
	}

	view, ok := hub.resubscribeBaselines[playerID]
	if !ok {
		t.Fatalf("expected baseline entry for %q", playerID)
	}

	if math.Abs(view.Player.Actor.X-42) > 1e-6 || math.Abs(view.Player.Actor.Y-(-17)) > 1e-6 {
		t.Fatalf("expected fallback baseline coordinates (42,-17), got (%.1f, %.1f)", view.Player.Actor.X, view.Player.Actor.Y)
	}
	if math.Abs(view.IntentDX-0.5) > 1e-6 || math.Abs(view.IntentDY-(-0.25)) > 1e-6 {
		t.Fatalf("expected fallback intents (0.5,-0.25), got (%.2f, %.2f)", view.IntentDX, view.IntentDY)
	}
}

func TestMarshalStateRemovesResubscribeBaselinesOnRemoval(t *testing.T) {
	hub := newHub()
	player := newTestPlayerState("resubscribe-remove")
	hub.world.AddPlayer(player)

	if _, _, err := hub.marshalState(nil, nil, nil, nil, false, true); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	if removed := hub.world.RemovePlayer(player.ID); !removed {
		t.Fatalf("expected player removal to succeed")
	}

	if _, _, err := hub.marshalState(nil, nil, nil, nil, true, false); err != nil {
		t.Fatalf("marshalState returned error during removal delta: %v", err)
	}

	if _, ok := hub.resubscribeBaselines[player.ID]; ok {
		t.Fatalf("expected baseline entry for %q to be removed", player.ID)
	}
}

func TestMarshalStateUsesResubscribeBaselinesForReplayPackaging(t *testing.T) {
	hub := newHub()
	player := newTestPlayerState("resubscribe-packaging")
	hub.world.AddPlayer(player)

	stale := []Player{{Actor: Actor{ID: player.ID, X: player.X, Y: player.Y}}}

	updatedX := player.X + 8
	updatedY := player.Y - 5
	hub.world.SetPosition(player.ID, updatedX, updatedY)

	data, _, err := marshalStateLegacy(hub, stale, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	if len(msg.Players) != 1 {
		t.Fatalf("expected exactly one player in packaged snapshot, got %d", len(msg.Players))
	}

	packaged := msg.Players[0]
	if math.Abs(packaged.X-updatedX) > 1e-6 || math.Abs(packaged.Y-updatedY) > 1e-6 {
		t.Fatalf("expected packaged coordinates (%.1f, %.1f), got (%.1f, %.1f)", updatedX, updatedY, packaged.X, packaged.Y)
	}
}

func TestMarshalStateUsesSimEngineSnapshotPatchesForReplay(t *testing.T) {
	hub := newHub()
	playerID := "resubscribe-engine-player"
	snapshot := sim.Snapshot{
		Players: []sim.Player{{Actor: sim.Actor{ID: playerID}}},
	}
	engine := newRecordingSimEngine(snapshot)
	engine.SetSnapshotPatches([]sim.Patch{{
		Kind:     sim.PatchPlayerIntent,
		EntityID: playerID,
		Payload:  sim.PlayerIntentPayload{DX: 0.5, DY: -0.25},
	}})
	hub.engine = engine

	data, _, err := hub.marshalState(nil, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	if len(msg.Patches) != 1 {
		t.Fatalf("expected exactly one patch in packaged snapshot, got %d", len(msg.Patches))
	}

	patch := msg.Patches[0]
	if patch.Kind != sim.PatchPlayerIntent {
		t.Fatalf("expected patch kind %q, got %q", sim.PatchPlayerIntent, patch.Kind)
	}
	if patch.EntityID != playerID {
		t.Fatalf("expected patch entity %q, got %q", playerID, patch.EntityID)
	}

	payload, ok := patch.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected player intent payload to decode as map, got %T", patch.Payload)
	}
	dx, dxOK := payload["dx"].(float64)
	dy, dyOK := payload["dy"].(float64)
	if !dxOK || !dyOK {
		t.Fatalf("expected payload to contain dx/dy floats, got %+v", payload)
	}
	if math.Abs(dx-0.5) > 1e-6 || math.Abs(dy-(-0.25)) > 1e-6 {
		t.Fatalf("expected payload (0.5,-0.25), got (%.2f, %.2f)", dx, dy)
	}
}

func TestClonePlayerViewsDeepCopiesData(t *testing.T) {
	base := map[string]simpaches.PlayerView{
		"player-clone": {
			Player: sim.Player{
				Actor: sim.Actor{
					ID: "player-clone",
					Inventory: itemspkg.InventoryFromSimSlots([]sim.InventorySlot{{
						Slot: 0,
						Item: sim.ItemStack{Type: sim.ItemType("gold"), Quantity: 5},
					}}),
					Equipment: itemspkg.EquipmentFromSimSlots([]sim.EquippedItem{{
						Slot: sim.EquipSlotMainHand,
						Item: sim.ItemStack{Type: sim.ItemType("sword"), Quantity: 1},
					}}),
				},
			},
			IntentDX: 0.25,
			IntentDY: -0.5,
		},
	}

	cloned := clonePlayerViewMap(base)
	if len(cloned) != len(base) {
		t.Fatalf("expected clone length %d, got %d", len(base), len(cloned))
	}

	view := cloned["player-clone"]
	view.Player.Inventory.Slots[0].Item.Quantity = 99
	view.Player.Equipment.Slots[0].Item.Type = sim.ItemType("axe")
	view.IntentDX = 1.0
	cloned["player-clone"] = view

	original := base["player-clone"]
	if original.Player.Inventory.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected original inventory quantity 5, got %d", original.Player.Inventory.Slots[0].Item.Quantity)
	}
	if original.Player.Equipment.Slots[0].Item.Type != sim.ItemType("sword") {
		t.Fatalf("expected original equipment type sword, got %s", original.Player.Equipment.Slots[0].Item.Type)
	}
	if math.Abs(original.IntentDX-0.25) > 1e-6 {
		t.Fatalf("expected original intent dx 0.25, got %.2f", original.IntentDX)
	}
}

func TestJoinIncludesGroundItems(t *testing.T) {
	hub := newHubWithFullWorld()
	first := hub.Join()
	if ack, _ := hub.HandleConsoleCommand(first.ID, "drop_gold", 10); ack.Status != "ok" {
		t.Fatalf("expected drop to succeed for seeded player")
	}
	second := hub.Join()
	if len(second.GroundItems) == 0 {
		t.Fatalf("expected join response to include ground items")
	}
	if second.GroundItems[0].Qty != 10 {
		t.Fatalf("expected join ground stack qty 10, got %d", second.GroundItems[0].Qty)
	}
}
