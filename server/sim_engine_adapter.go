package server

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
	"mine-and-die/server/internal/simutil"
)

type legacyEngineAdapter struct {
	world *World
	deps  sim.Deps

	pendingTick     uint64
	pendingNow      time.Time
	pendingDT       float64
	pendingCommands []Command
	emitEffect      func(effectcontract.EffectLifecycleEvent)

	lastRemoved []string
}

func newLegacyEngineAdapter(world *World, deps sim.Deps) *legacyEngineAdapter {
	return &legacyEngineAdapter{world: world, deps: deps}
}

func (a *legacyEngineAdapter) Deps() sim.Deps {
	if a == nil {
		return sim.Deps{}
	}
	return a.deps
}

func (a *legacyEngineAdapter) SetWorld(world *World) {
	a.world = world
	if world != nil {
		a.deps.RNG = world.rng
	} else {
		a.deps.RNG = nil
	}
}

func (a *legacyEngineAdapter) PrepareStep(tick uint64, now time.Time, dt float64, emit func(effectcontract.EffectLifecycleEvent)) {
	a.pendingTick = tick
	a.pendingNow = now
	a.pendingDT = dt
	a.emitEffect = emit
}

func (a *legacyEngineAdapter) Apply(cmds []sim.Command) error {
	if a == nil {
		return nil
	}
	a.pendingCommands = legacyCommandsFromSim(cmds)
	return nil
}

func (a *legacyEngineAdapter) Step() {
	if a == nil || a.world == nil {
		return
	}
	removed := a.world.Step(a.pendingTick, a.pendingNow, a.pendingDT, a.pendingCommands, a.emitEffect)
	if len(removed) > 0 {
		copied := make([]string, len(removed))
		copy(copied, removed)
		a.lastRemoved = copied
	} else {
		a.lastRemoved = nil
	}
	a.pendingCommands = nil
}

func (a *legacyEngineAdapter) Snapshot() sim.Snapshot {
	if a == nil || a.world == nil {
		return sim.Snapshot{}
	}
	players, npcs := a.world.Snapshot(a.pendingNow)
	groundItems := a.world.GroundItemsSnapshot()
	triggers := a.world.flushEffectTriggersLocked()
	obstacles := a.world.obstacles
	aliveEffectIDs := simAliveEffectIDsFromLegacy(a.world.effects)
	simPlayers := simPlayersFromLegacy(players)
	if len(simPlayers) > 0 {
		for i := range simPlayers {
			id := simPlayers[i].Actor.ID
			if id == "" {
				continue
			}
			if state, ok := a.world.players[id]; ok && state != nil {
				simPlayers[i].IntentDX = state.intentX
				simPlayers[i].IntentDY = state.intentY
			}
		}
	}
	return sim.Snapshot{
		Players:        simPlayers,
		NPCs:           simNPCsFromLegacy(npcs),
		GroundItems:    simGroundItemsFromLegacy(groundItems),
		EffectEvents:   simEffectTriggersFromLegacy(triggers),
		Obstacles:      simObstaclesFromLegacy(obstacles),
		AliveEffectIDs: aliveEffectIDs,
	}
}

func (a *legacyEngineAdapter) DrainPatches() []sim.Patch {
	if a == nil || a.world == nil {
		return nil
	}
	patches := a.world.drainPatchesLocked()
	return simPatchesFromLegacy(patches)
}

func (a *legacyEngineAdapter) SnapshotPatches() []sim.Patch {
	if a == nil || a.world == nil {
		return nil
	}
	patches := a.world.snapshotPatchesLocked()
	return simPatchesFromLegacy(patches)
}

func (a *legacyEngineAdapter) RestorePatches(patches []sim.Patch) {
	if a == nil || a.world == nil {
		return
	}
	legacy := legacyPatchesFromSim(patches)
	a.world.journal.RestorePatches(legacy)
}

func (a *legacyEngineAdapter) DrainEffectEvents() sim.EffectEventBatch {
	if a == nil || a.world == nil {
		return sim.EffectEventBatch{}
	}
	batch := a.world.journal.DrainEffectEvents()
	return simEffectEventBatchFromLegacy(batch)
}

func (a *legacyEngineAdapter) SnapshotEffectEvents() sim.EffectEventBatch {
	if a == nil || a.world == nil {
		return sim.EffectEventBatch{}
	}
	batch := a.world.journal.SnapshotEffectEvents()
	return simEffectEventBatchFromLegacy(batch)
}

func (a *legacyEngineAdapter) RestoreEffectEvents(batch sim.EffectEventBatch) {
	if a == nil || a.world == nil {
		return
	}
	legacy := legacyEffectEventBatchFromSim(batch)
	a.world.journal.RestoreEffectEvents(legacy)
}

func (a *legacyEngineAdapter) ConsumeEffectResyncHint() (sim.EffectResyncSignal, bool) {
	if a == nil || a.world == nil {
		return sim.EffectResyncSignal{}, false
	}
	signal, ok := a.world.journal.ConsumeResyncHint()
	if !ok {
		return sim.EffectResyncSignal{}, false
	}
	return simEffectResyncSignalFromLegacy(signal), true
}

func (a *legacyEngineAdapter) RecordKeyframe(frame sim.Keyframe) sim.KeyframeRecordResult {
	if a == nil || a.world == nil {
		return sim.KeyframeRecordResult{}
	}
	legacy := legacyKeyframeFromSim(frame)
	record := a.world.journal.RecordKeyframe(legacy)
	return simKeyframeRecordResultFromLegacy(record)
}

func (a *legacyEngineAdapter) KeyframeBySequence(sequence uint64) (sim.Keyframe, bool) {
	if a == nil || a.world == nil {
		return sim.Keyframe{}, false
	}
	frame, ok := a.world.journal.KeyframeBySequence(sequence)
	if !ok {
		return sim.Keyframe{}, false
	}
	return simKeyframeFromLegacy(frame), true
}

func (a *legacyEngineAdapter) KeyframeWindow() (int, uint64, uint64) {
	if a == nil || a.world == nil {
		return 0, 0, 0
	}
	return a.world.journal.KeyframeWindow()
}

func (a *legacyEngineAdapter) RemovedPlayers() []string {
	if a == nil || len(a.lastRemoved) == 0 {
		return nil
	}
	removed := make([]string, len(a.lastRemoved))
	copy(removed, a.lastRemoved)
	a.lastRemoved = nil
	return removed
}

func legacyCommandsFromSim(cmds []sim.Command) []Command {
	if len(cmds) == 0 {
		return nil
	}
	converted := make([]Command, len(cmds))
	for i, cmd := range cmds {
		converted[i] = Command{
			OriginTick: cmd.OriginTick,
			ActorID:    cmd.ActorID,
			Type:       legacyCommandTypeFromSim(cmd.Type),
			IssuedAt:   cmd.IssuedAt,
		}
		if cmd.Move != nil {
			converted[i].Move = &MoveCommand{
				DX:     cmd.Move.DX,
				DY:     cmd.Move.DY,
				Facing: legacyFacingFromSim(cmd.Move.Facing),
			}
		}
		if cmd.Action != nil {
			converted[i].Action = &ActionCommand{Name: cmd.Action.Name}
		}
		if cmd.Heartbeat != nil {
			converted[i].Heartbeat = &HeartbeatCommand{
				ReceivedAt: cmd.Heartbeat.ReceivedAt,
				ClientSent: cmd.Heartbeat.ClientSent,
				RTT:        cmd.Heartbeat.RTT,
			}
		}
		if cmd.Path != nil {
			converted[i].Path = &PathCommand{
				TargetX: cmd.Path.TargetX,
				TargetY: cmd.Path.TargetY,
			}
		}
	}
	return converted
}

func simCommandsFromLegacy(cmds []Command) []sim.Command {
	if len(cmds) == 0 {
		return nil
	}
	converted := make([]sim.Command, len(cmds))
	for i, cmd := range cmds {
		converted[i] = sim.Command{
			OriginTick: cmd.OriginTick,
			ActorID:    cmd.ActorID,
			Type:       toSimCommandType(cmd.Type),
			IssuedAt:   cmd.IssuedAt,
		}
		if cmd.Move != nil {
			converted[i].Move = &sim.MoveCommand{
				DX:     cmd.Move.DX,
				DY:     cmd.Move.DY,
				Facing: toSimFacing(cmd.Move.Facing),
			}
		}
		if cmd.Action != nil {
			converted[i].Action = &sim.ActionCommand{Name: cmd.Action.Name}
		}
		if cmd.Heartbeat != nil {
			converted[i].Heartbeat = &sim.HeartbeatCommand{
				ReceivedAt: cmd.Heartbeat.ReceivedAt,
				ClientSent: cmd.Heartbeat.ClientSent,
				RTT:        cmd.Heartbeat.RTT,
			}
		}
		if cmd.Path != nil {
			converted[i].Path = &sim.PathCommand{
				TargetX: cmd.Path.TargetX,
				TargetY: cmd.Path.TargetY,
			}
		}
	}
	return converted
}

func simPlayersFromLegacy(players []Player) []sim.Player {
	if len(players) == 0 {
		return nil
	}
	converted := make([]sim.Player, len(players))
	for i, player := range players {
		converted[i] = sim.Player{Actor: simActorFromLegacy(player.Actor)}
	}
	return converted
}

func legacyPlayerFromSim(player sim.Player) Player {
	return Player{Actor: legacyActorFromSim(player.Actor)}
}

func legacyPlayersFromSim(players []sim.Player) []Player {
	if len(players) == 0 {
		return nil
	}
	converted := make([]Player, len(players))
	for i, player := range players {
		converted[i] = legacyPlayerFromSim(player)
	}
	return converted
}

func simNPCsFromLegacy(npcs []NPC) []sim.NPC {
	if len(npcs) == 0 {
		return nil
	}
	converted := make([]sim.NPC, len(npcs))
	for i, npc := range npcs {
		converted[i] = sim.NPC{
			Actor:            simActorFromLegacy(npc.Actor),
			Type:             toSimNPCType(npc.Type),
			AIControlled:     npc.AIControlled,
			ExperienceReward: npc.ExperienceReward,
		}
	}
	return converted
}

func legacyNPCsFromSim(npcs []sim.NPC) []NPC {
	if len(npcs) == 0 {
		return nil
	}
	converted := make([]NPC, len(npcs))
	for i, npc := range npcs {
		converted[i] = NPC{
			Actor:            legacyActorFromSim(npc.Actor),
			Type:             legacyNPCTypeFromSim(npc.Type),
			AIControlled:     npc.AIControlled,
			ExperienceReward: npc.ExperienceReward,
		}
	}
	return converted
}

func legacyActorsFromSimSnapshot(snapshot sim.Snapshot) ([]Player, []NPC) {
	players := legacyPlayersFromSim(snapshot.Players)
	if players == nil {
		players = make([]Player, 0)
	}
	npcs := legacyNPCsFromSim(snapshot.NPCs)
	if npcs == nil {
		npcs = make([]NPC, 0)
	}
	return players, npcs
}

func simGroundItemsFromLegacy(items []GroundItem) []sim.GroundItem {
	if len(items) == 0 {
		return nil
	}
	converted := make([]sim.GroundItem, len(items))
	for i, item := range items {
		converted[i] = sim.GroundItem{
			ID:             item.ID,
			Type:           toSimItemType(ItemType(item.Type)),
			FungibilityKey: item.FungibilityKey,
			X:              item.X,
			Y:              item.Y,
			Qty:            item.Qty,
		}
	}
	return converted
}

func legacyGroundItemsFromSim(items []sim.GroundItem) []GroundItem {
	if len(items) == 0 {
		return nil
	}
	converted := make([]GroundItem, len(items))
	for i, item := range items {
		converted[i] = GroundItem{
			ID:             item.ID,
			Type:           string(legacyItemTypeFromSim(item.Type)),
			FungibilityKey: item.FungibilityKey,
			X:              item.X,
			Y:              item.Y,
			Qty:            item.Qty,
		}
	}
	return converted
}

func simEffectTriggersFromLegacy(triggers []EffectTrigger) []sim.EffectTrigger {
	if len(triggers) == 0 {
		return nil
	}
	converted := make([]sim.EffectTrigger, len(triggers))
	for i, trigger := range triggers {
		converted[i] = sim.EffectTrigger{
			ID:       trigger.ID,
			Type:     trigger.Type,
			Start:    trigger.Start,
			Duration: trigger.Duration,
			X:        trigger.X,
			Y:        trigger.Y,
			Width:    trigger.Width,
			Height:   trigger.Height,
			Params:   simutil.CloneFloatMap(trigger.Params),
			Colors:   simutil.CloneStringSlice(trigger.Colors),
		}
	}
	return converted
}

func legacyEffectTriggersFromSim(triggers []sim.EffectTrigger) []EffectTrigger {
	if len(triggers) == 0 {
		return nil
	}
	converted := make([]EffectTrigger, len(triggers))
	for i, trigger := range triggers {
		converted[i] = EffectTrigger{
			ID:       trigger.ID,
			Type:     trigger.Type,
			Start:    trigger.Start,
			Duration: trigger.Duration,
			X:        trigger.X,
			Y:        trigger.Y,
			Width:    trigger.Width,
			Height:   trigger.Height,
			Params:   simutil.CloneFloatMap(trigger.Params),
			Colors:   simutil.CloneStringSlice(trigger.Colors),
		}
	}
	return converted
}

func simPatchesFromLegacy(patches []Patch) []sim.Patch {
	if len(patches) == 0 {
		return nil
	}
	converted := make([]sim.Patch, len(patches))
	for i, patch := range patches {
		converted[i] = sim.Patch{
			Kind:     toSimPatchKind(patch.Kind),
			EntityID: patch.EntityID,
			Payload:  convertPatchPayloadToSim(patch.Payload),
		}
	}
	return converted
}

func simKeyframeFromLegacy(frame keyframe) sim.Keyframe {
	var (
		legacyPlayers     []Player
		legacyNPCs        []NPC
		legacyObstacles   []Obstacle
		legacyGroundItems []GroundItem
		legacyConfig      worldConfig
	)

	if typed, ok := frame.Players.([]Player); ok {
		legacyPlayers = typed
	}
	if typed, ok := frame.NPCs.([]NPC); ok {
		legacyNPCs = typed
	}
	if typed, ok := frame.Obstacles.([]Obstacle); ok {
		legacyObstacles = typed
	}
	if typed, ok := frame.GroundItems.([]GroundItem); ok {
		legacyGroundItems = typed
	}
	if typed, ok := frame.Config.(worldConfig); ok {
		legacyConfig = typed
	}

	return sim.Keyframe{
		Tick:        frame.Tick,
		Sequence:    frame.Sequence,
		Players:     simPlayersFromLegacy(legacyPlayers),
		NPCs:        simNPCsFromLegacy(legacyNPCs),
		Obstacles:   simObstaclesFromLegacy(legacyObstacles),
		GroundItems: simGroundItemsFromLegacy(legacyGroundItems),
		Config:      simWorldConfigFromLegacy(legacyConfig),
		RecordedAt:  frame.RecordedAt,
	}
}

func legacyKeyframeFromSim(frame sim.Keyframe) keyframe {
	return keyframe{
		Tick:        frame.Tick,
		Sequence:    frame.Sequence,
		Players:     legacyPlayersFromSim(frame.Players),
		NPCs:        legacyNPCsFromSim(frame.NPCs),
		Obstacles:   legacyObstaclesFromSim(frame.Obstacles),
		GroundItems: legacyGroundItemsFromSim(frame.GroundItems),
		Config:      legacyWorldConfigFromSim(frame.Config),
		RecordedAt:  frame.RecordedAt,
	}
}

func simKeyframeRecordResultFromLegacy(result keyframeRecordResult) sim.KeyframeRecordResult {
	evicted := make([]sim.KeyframeEviction, len(result.Evicted))
	for i, eviction := range result.Evicted {
		evicted[i] = sim.KeyframeEviction{
			Sequence: eviction.Sequence,
			Tick:     eviction.Tick,
			Reason:   eviction.Reason,
		}
	}
	return sim.KeyframeRecordResult{
		Size:           result.Size,
		OldestSequence: result.OldestSequence,
		NewestSequence: result.NewestSequence,
		Evicted:        evicted,
	}
}

func legacyKeyframeRecordResultFromSim(result sim.KeyframeRecordResult) keyframeRecordResult {
	evicted := make([]journalEviction, len(result.Evicted))
	for i, eviction := range result.Evicted {
		evicted[i] = journalEviction{
			Sequence: eviction.Sequence,
			Tick:     eviction.Tick,
			Reason:   eviction.Reason,
		}
	}
	return keyframeRecordResult{
		Size:           result.Size,
		OldestSequence: result.OldestSequence,
		NewestSequence: result.NewestSequence,
		Evicted:        evicted,
	}
}

func simObstaclesFromLegacy(obstacles []Obstacle) []sim.Obstacle {
	if len(obstacles) == 0 {
		return nil
	}
	converted := make([]sim.Obstacle, len(obstacles))
	for i, obstacle := range obstacles {
		converted[i] = sim.Obstacle{
			ID:     obstacle.ID,
			Type:   obstacle.Type,
			X:      obstacle.X,
			Y:      obstacle.Y,
			Width:  obstacle.Width,
			Height: obstacle.Height,
		}
	}
	return converted
}

func legacyObstaclesFromSim(obstacles []sim.Obstacle) []Obstacle {
	if len(obstacles) == 0 {
		return nil
	}
	converted := make([]Obstacle, len(obstacles))
	for i, obstacle := range obstacles {
		converted[i] = Obstacle{
			ID:     obstacle.ID,
			Type:   obstacle.Type,
			X:      obstacle.X,
			Y:      obstacle.Y,
			Width:  obstacle.Width,
			Height: obstacle.Height,
		}
	}
	return converted
}

func simAliveEffectIDsFromLegacy(effects []*effectState) []string {
	if len(effects) == 0 {
		return nil
	}
	ids := make([]string, 0, len(effects))
	for _, eff := range effects {
		if eff == nil || eff.ID == "" {
			continue
		}
		ids = append(ids, eff.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func simWorldConfigFromLegacy(cfg worldConfig) sim.WorldConfig {
	return sim.WorldConfig{
		Obstacles:      cfg.Obstacles,
		ObstaclesCount: cfg.ObstaclesCount,
		GoldMines:      cfg.GoldMines,
		GoldMineCount:  cfg.GoldMineCount,
		NPCs:           cfg.NPCs,
		GoblinCount:    cfg.GoblinCount,
		RatCount:       cfg.RatCount,
		NPCCount:       cfg.NPCCount,
		Lava:           cfg.Lava,
		LavaCount:      cfg.LavaCount,
		Seed:           cfg.Seed,
		Width:          cfg.Width,
		Height:         cfg.Height,
	}
}

func legacyWorldConfigFromSim(cfg sim.WorldConfig) worldConfig {
	return worldConfig{
		Obstacles:      cfg.Obstacles,
		ObstaclesCount: cfg.ObstaclesCount,
		GoldMines:      cfg.GoldMines,
		GoldMineCount:  cfg.GoldMineCount,
		NPCs:           cfg.NPCs,
		GoblinCount:    cfg.GoblinCount,
		RatCount:       cfg.RatCount,
		NPCCount:       cfg.NPCCount,
		Lava:           cfg.Lava,
		LavaCount:      cfg.LavaCount,
		Seed:           cfg.Seed,
		Width:          cfg.Width,
		Height:         cfg.Height,
	}
}

func convertPatchPayloadToSim(payload any) any {
	switch value := payload.(type) {
	case PositionPayload:
		return sim.PositionPayload{X: value.X, Y: value.Y}
	case *PositionPayload:
		if value == nil {
			return nil
		}
		return sim.PositionPayload{X: value.X, Y: value.Y}
	case FacingPayload:
		return sim.FacingPayload{Facing: toSimFacingFromAny(value.Facing)}
	case *FacingPayload:
		if value == nil {
			return nil
		}
		return sim.FacingPayload{Facing: toSimFacingFromAny(value.Facing)}
	case PlayerIntentPayload:
		return sim.PlayerIntentPayload{DX: value.DX, DY: value.DY}
	case *PlayerIntentPayload:
		if value == nil {
			return nil
		}
		return sim.PlayerIntentPayload{DX: value.DX, DY: value.DY}
	case HealthPayload:
		return sim.HealthPayload{Health: value.Health, MaxHealth: value.MaxHealth}
	case *HealthPayload:
		if value == nil {
			return nil
		}
		return sim.HealthPayload{Health: value.Health, MaxHealth: value.MaxHealth}
	case InventoryPayload:
		return sim.InventoryPayload{Slots: simInventorySlotsFromAny(value.Slots)}
	case *InventoryPayload:
		if value == nil {
			return nil
		}
		return sim.InventoryPayload{Slots: simInventorySlotsFromAny(value.Slots)}
	case EquipmentPayload:
		return sim.EquipmentPayload{Slots: simEquippedItemsFromAny(value.Slots)}
	case *EquipmentPayload:
		if value == nil {
			return nil
		}
		return sim.EquipmentPayload{Slots: simEquippedItemsFromAny(value.Slots)}
	case EffectParamsPayload:
		return sim.EffectParamsPayload{Params: simutil.CloneFloatMap(value.Params)}
	case *EffectParamsPayload:
		if value == nil {
			return nil
		}
		return sim.EffectParamsPayload{Params: simutil.CloneFloatMap(value.Params)}
	case GroundItemQtyPayload:
		return sim.GroundItemQtyPayload{Qty: value.Qty}
	case *GroundItemQtyPayload:
		if value == nil {
			return nil
		}
		return sim.GroundItemQtyPayload{Qty: value.Qty}
	default:
		return value
	}
}

func toSimFacingFromAny(value any) sim.FacingDirection {
	switch facing := value.(type) {
	case nil:
		return ""
	case sim.FacingDirection:
		return facing
	case FacingDirection:
		return toSimFacing(facing)
	case *FacingDirection:
		if facing == nil {
			return ""
		}
		return toSimFacing(*facing)
	case string:
		return toSimFacing(FacingDirection(facing))
	case *string:
		if facing == nil {
			return ""
		}
		return toSimFacing(FacingDirection(*facing))
	default:
		return ""
	}
}

func simInventorySlotsFromAny(value any) []sim.InventorySlot {
	switch slots := value.(type) {
	case nil:
		return nil
	case []sim.InventorySlot:
		return simutil.CloneInventorySlots(slots)
	case []InventorySlot:
		return simInventorySlotsFromLegacy(slots)
	case *[]sim.InventorySlot:
		if slots == nil {
			return nil
		}
		return simutil.CloneInventorySlots(*slots)
	case *[]InventorySlot:
		if slots == nil {
			return nil
		}
		return simInventorySlotsFromLegacy(*slots)
	default:
		return nil
	}
}

func simEquippedItemsFromAny(value any) []sim.EquippedItem {
	switch slots := value.(type) {
	case nil:
		return nil
	case []sim.EquippedItem:
		return simutil.CloneEquippedItems(slots)
	case []EquippedItem:
		return simEquippedItemsFromLegacy(slots)
	case *[]sim.EquippedItem:
		if slots == nil {
			return nil
		}
		return simutil.CloneEquippedItems(*slots)
	case *[]EquippedItem:
		if slots == nil {
			return nil
		}
		return simEquippedItemsFromLegacy(*slots)
	default:
		return nil
	}
}

func legacyPatchesFromSim(patches []sim.Patch) []Patch {
	if len(patches) == 0 {
		return nil
	}
	converted := make([]Patch, len(patches))
	for i, patch := range patches {
		converted[i] = Patch{
			Kind:     legacyPatchKindFromSim(patch.Kind),
			EntityID: patch.EntityID,
			Payload:  convertPatchPayloadFromSim(patch.Payload),
		}
	}
	return converted
}

func simEffectEventBatchFromLegacy(batch EffectEventBatch) sim.EffectEventBatch {
	return sim.EffectEventBatch{
		Spawns:      journal.CloneEffectSpawnEvents(batch.Spawns),
		Updates:     journal.CloneEffectUpdateEvents(batch.Updates),
		Ends:        journal.CloneEffectEndEvents(batch.Ends),
		LastSeqByID: journal.CopySeqMap(batch.LastSeqByID),
	}
}

func legacyEffectEventBatchFromSim(batch sim.EffectEventBatch) EffectEventBatch {
	return EffectEventBatch{
		Spawns:      journal.CloneEffectSpawnEvents(batch.Spawns),
		Updates:     journal.CloneEffectUpdateEvents(batch.Updates),
		Ends:        journal.CloneEffectEndEvents(batch.Ends),
		LastSeqByID: journal.CopySeqMap(batch.LastSeqByID),
	}
}

func simEffectResyncSignalFromLegacy(signal resyncSignal) sim.EffectResyncSignal {
	return sim.EffectResyncSignal{
		LostSpawns:  signal.LostSpawns,
		TotalEvents: signal.TotalEvents,
		Reasons:     simResyncReasonsFromLegacy(signal.Reasons),
	}
}

func legacyEffectResyncSignalFromSim(signal sim.EffectResyncSignal) resyncSignal {
	return resyncSignal{
		LostSpawns:  signal.LostSpawns,
		TotalEvents: signal.TotalEvents,
		Reasons:     legacyResyncReasonsFromSim(signal.Reasons),
	}
}

func simResyncReasonsFromLegacy(reasons []resyncReason) []sim.EffectResyncReason {
	if len(reasons) == 0 {
		return nil
	}
	converted := make([]sim.EffectResyncReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = sim.EffectResyncReason{
			Kind:     reason.Kind,
			EffectID: reason.EffectID,
		}
	}
	return converted
}

func legacyResyncReasonsFromSim(reasons []sim.EffectResyncReason) []resyncReason {
	if len(reasons) == 0 {
		return nil
	}
	converted := make([]resyncReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = resyncReason{
			Kind:     reason.Kind,
			EffectID: reason.EffectID,
		}
	}
	return converted
}

func convertPatchPayloadFromSim(payload any) any {
	switch value := payload.(type) {
	case sim.PositionPayload:
		return PositionPayload{X: value.X, Y: value.Y}
	case sim.FacingPayload:
		return FacingPayload{Facing: legacyFacingFromSim(value.Facing)}
	case sim.PlayerIntentPayload:
		return PlayerIntentPayload{DX: value.DX, DY: value.DY}
	case sim.HealthPayload:
		return HealthPayload{Health: value.Health, MaxHealth: value.MaxHealth}
	case sim.InventoryPayload:
		inv := legacyInventoryFromSim(sim.Inventory{Slots: value.Slots})
		return InventoryPayload{Slots: inv.Slots}
	case sim.EquipmentPayload:
		eq := legacyEquipmentFromSim(sim.Equipment{Slots: value.Slots})
		return EquipmentPayload{Slots: eq.Slots}
	case sim.EffectParamsPayload:
		return EffectParamsPayload{Params: simutil.CloneFloatMap(value.Params)}
	case sim.GroundItemQtyPayload:
		return GroundItemQtyPayload{Qty: value.Qty}
	default:
		return value
	}
}

func simInventorySlotsFromLegacy(slots []InventorySlot) []sim.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	converted := make([]sim.InventorySlot, len(slots))
	for i, slot := range slots {
		converted[i] = sim.InventorySlot{
			Slot: slot.Slot,
			Item: simItemStackFromLegacy(slot.Item),
		}
	}
	return converted
}

func simEquippedItemsFromLegacy(slots []EquippedItem) []sim.EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	converted := make([]sim.EquippedItem, len(slots))
	for i, slot := range slots {
		converted[i] = sim.EquippedItem{
			Slot: toSimEquipSlot(slot.Slot),
			Item: simItemStackFromLegacy(slot.Item),
		}
	}
	return converted
}

func simItemStackFromLegacy(stack ItemStack) sim.ItemStack {
	return sim.ItemStack{
		Type:           toSimItemType(stack.Type),
		FungibilityKey: stack.FungibilityKey,
		Quantity:       stack.Quantity,
	}
}

func simActorFromLegacy(actor Actor) sim.Actor {
	return sim.Actor{
		ID:        actor.ID,
		X:         actor.X,
		Y:         actor.Y,
		Facing:    toSimFacing(actor.Facing),
		Health:    actor.Health,
		MaxHealth: actor.MaxHealth,
		Inventory: simInventoryFromLegacy(actor.Inventory),
		Equipment: simEquipmentFromLegacy(actor.Equipment),
	}
}

func simInventoryFromLegacy(inv Inventory) sim.Inventory {
	return sim.Inventory{Slots: simInventorySlotsFromLegacy(inv.Slots)}
}

func simEquipmentFromLegacy(eq Equipment) sim.Equipment {
	return sim.Equipment{Slots: simEquippedItemsFromLegacy(eq.Slots)}
}

func legacyActorFromSim(actor sim.Actor) Actor {
	return Actor{
		ID:        actor.ID,
		X:         actor.X,
		Y:         actor.Y,
		Facing:    legacyFacingFromSim(actor.Facing),
		Health:    actor.Health,
		MaxHealth: actor.MaxHealth,
		Inventory: legacyInventoryFromSim(actor.Inventory),
		Equipment: legacyEquipmentFromSim(actor.Equipment),
	}
}

func legacyInventoryFromSim(inv sim.Inventory) Inventory {
	if len(inv.Slots) == 0 {
		return Inventory{}
	}
	converted := make([]InventorySlot, len(inv.Slots))
	for i, slot := range inv.Slots {
		converted[i] = InventorySlot{
			Slot: slot.Slot,
			Item: legacyItemStackFromSim(slot.Item),
		}
	}
	return Inventory{Slots: converted}
}

func legacyEquipmentFromSim(eq sim.Equipment) Equipment {
	if len(eq.Slots) == 0 {
		return Equipment{}
	}
	converted := make([]EquippedItem, len(eq.Slots))
	for i, slot := range eq.Slots {
		converted[i] = EquippedItem{
			Slot: legacyEquipSlotFromSim(slot.Slot),
			Item: legacyItemStackFromSim(slot.Item),
		}
	}
	return Equipment{Slots: converted}
}

func legacyItemStackFromSim(stack sim.ItemStack) ItemStack {
	return ItemStack{
		Type:           legacyItemTypeFromSim(stack.Type),
		FungibilityKey: stack.FungibilityKey,
		Quantity:       stack.Quantity,
	}
}

// Ensure legacyEngineAdapter implements sim.EngineCore.
var _ sim.EngineCore = (*legacyEngineAdapter)(nil)
