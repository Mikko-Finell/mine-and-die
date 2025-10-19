package main

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/sim"
)

type legacyEngineAdapter struct {
	world *World

	pendingTick     uint64
	pendingNow      time.Time
	pendingDT       float64
	pendingCommands []Command
	emitEffect      func(effectcontract.EffectLifecycleEvent)

	lastRemoved []string
}

func newLegacyEngineAdapter(world *World) *legacyEngineAdapter {
	return &legacyEngineAdapter{world: world}
}

func (a *legacyEngineAdapter) SetWorld(world *World) {
	a.world = world
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
	return sim.Snapshot{
		Players:      simPlayersFromLegacy(players),
		NPCs:         simNPCsFromLegacy(npcs),
		GroundItems:  simGroundItemsFromLegacy(groundItems),
		EffectEvents: simEffectTriggersFromLegacy(triggers),
	}
}

func (a *legacyEngineAdapter) DrainPatches() []sim.Patch {
	if a == nil || a.world == nil {
		return nil
	}
	patches := a.world.drainPatchesLocked()
	return simPatchesFromLegacy(patches)
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

func legacyPlayersFromSim(players []sim.Player) []Player {
	if len(players) == 0 {
		return nil
	}
	converted := make([]Player, len(players))
	for i, player := range players {
		converted[i] = Player{Actor: legacyActorFromSim(player.Actor)}
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
			Type:           toSimItemType(item.Type),
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
			Type:           legacyItemTypeFromSim(item.Type),
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
			Params:   cloneFloatMap(trigger.Params),
			Colors:   cloneStringSlice(trigger.Colors),
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
			Params:   cloneFloatMap(trigger.Params),
			Colors:   cloneStringSlice(trigger.Colors),
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

func convertPatchPayloadToSim(payload any) any {
	switch value := payload.(type) {
	case PositionPayload:
		return sim.PositionPayload{X: value.X, Y: value.Y}
	case FacingPayload:
		return sim.FacingPayload{Facing: toSimFacing(value.Facing)}
	case PlayerIntentPayload:
		return sim.PlayerIntentPayload{DX: value.DX, DY: value.DY}
	case HealthPayload:
		return sim.HealthPayload{Health: value.Health, MaxHealth: value.MaxHealth}
	case InventoryPayload:
		return sim.InventoryPayload{Slots: simInventorySlotsFromLegacy(value.Slots)}
	case EquipmentPayload:
		return sim.EquipmentPayload{Slots: simEquippedItemsFromLegacy(value.Slots)}
	case EffectParamsPayload:
		return sim.EffectParamsPayload{Params: cloneFloatMap(value.Params)}
	case GroundItemQtyPayload:
		return sim.GroundItemQtyPayload{Qty: value.Qty}
	default:
		return value
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
		return EffectParamsPayload{Params: cloneFloatMap(value.Params)}
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

func cloneFloatMap(values map[string]float64) map[string]float64 {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]float64, len(values))
	for k, v := range values {
		cloned[k] = v
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

// Ensure legacyEngineAdapter implements sim.Engine.
var _ sim.Engine = (*legacyEngineAdapter)(nil)
