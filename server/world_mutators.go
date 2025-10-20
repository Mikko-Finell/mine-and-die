package server

import (
	"math"

	stats "mine-and-die/server/stats"
)

const positionEpsilon = 1e-6
const healthEpsilon = 1e-6
const intentEpsilon = 1e-6

func (w *World) appendPatch(kind PatchKind, entityID string, payload any) {
	if w == nil || entityID == "" {
		return
	}
	w.journal.AppendPatch(Patch{Kind: kind, EntityID: entityID, Payload: payload})
}

func incrementVersion(version *uint64) {
	if version == nil {
		return
	}
	*version = *version + 1
}

func (w *World) purgeEntityPatches(entityID string) {
	if w == nil || entityID == "" {
		return
	}
	w.journal.PurgeEntity(entityID)
}

func (w *World) setActorPosition(actor *actorState, version *uint64, entityID string, kind PatchKind, x, y float64) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	if positionsEqual(actor.X, actor.Y, x, y) {
		return
	}

	actor.X = x
	actor.Y = y
	incrementVersion(version)

	w.appendPatch(kind, entityID, PositionPayload{X: x, Y: y})
}

func (w *World) setActorFacing(actor *actorState, version *uint64, entityID string, kind PatchKind, facing FacingDirection) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	if facing == "" {
		facing = defaultFacing
	}

	if actor.Facing == facing {
		return
	}

	actor.Facing = facing
	incrementVersion(version)

	w.appendPatch(kind, entityID, FacingPayload{Facing: facing})
}

func (w *World) setActorIntent(actor *actorState, version *uint64, entityID string, dx, dy float64) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	if math.Abs(actor.intentX-dx) < intentEpsilon && math.Abs(actor.intentY-dy) < intentEpsilon {
		return
	}

	actor.intentX = dx
	actor.intentY = dy
	incrementVersion(version)

	w.appendPatch(PatchPlayerIntent, entityID, PlayerIntentPayload{DX: dx, DY: dy})
}

func (w *World) setActorHealth(actor *actorState, version *uint64, entityID string, kind PatchKind, computedMax float64, health float64) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	if math.IsNaN(health) || math.IsInf(health, 0) {
		return
	}

	max := computedMax
	if max <= 0 {
		max = actor.MaxHealth
	}
	if max <= 0 {
		max = health
	}

	if health < 0 {
		health = 0
	}
	if max > 0 && health > max {
		health = max
	}

	maxDiff := math.Abs(actor.MaxHealth - max)
	healthDiff := math.Abs(actor.Health - health)
	if maxDiff < healthEpsilon && healthDiff < healthEpsilon {
		return
	}

	actor.Health = health
	actor.MaxHealth = max
	incrementVersion(version)

	w.appendPatch(kind, entityID, HealthPayload{Health: health, MaxHealth: max})
}

func (w *World) mutateActorInventory(actor *actorState, version *uint64, entityID string, kind PatchKind, mutate func(inv *Inventory) error) error {
	if w == nil || actor == nil || version == nil || entityID == "" || mutate == nil {
		return nil
	}

	before := actor.Inventory.Clone()
	if err := mutate(&actor.Inventory); err != nil {
		actor.Inventory = before
		return err
	}

	if inventoriesEqual(before, actor.Inventory) {
		return nil
	}

	incrementVersion(version)
	w.appendPatch(kind, entityID, InventoryPayload{Slots: cloneInventorySlots(actor.Inventory.Slots)})
	return nil
}

func (w *World) mutateActorEquipment(actor *actorState, version *uint64, entityID string, kind PatchKind, mutate func(eq *Equipment) error) error {
	if w == nil || actor == nil || version == nil || entityID == "" || mutate == nil {
		return nil
	}

	before := actor.Equipment.Clone()
	if err := mutate(&actor.Equipment); err != nil {
		actor.Equipment = before
		return err
	}

	if equipmentsEqual(before, actor.Equipment) {
		return nil
	}

	incrementVersion(version)
	w.appendPatch(kind, entityID, EquipmentPayload{Slots: cloneEquipmentSlots(actor.Equipment.Slots)})
	return nil
}

// positionsEqual reports whether two coordinate pairs are effectively the same.
func positionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= positionEpsilon && math.Abs(ay-by) <= positionEpsilon
}

// SetPosition updates a player's position, bumps the version, and records a patch.
// All player position writes must flow through this helper so snapshot versions
// and patch journals stay authoritative.
func (w *World) SetPosition(playerID string, x, y float64) {
	if w == nil {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	w.setActorPosition(&player.actorState, &player.version, playerID, PatchPlayerPos, x, y)
}

// SetFacing updates a player's facing, bumps the version, and records a patch.
// All player facing writes must flow through this helper so snapshot versions
// and patch journals stay authoritative.
func (w *World) SetFacing(playerID string, facing FacingDirection) {
	if w == nil {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	w.setActorFacing(&player.actorState, &player.version, playerID, PatchPlayerFacing, facing)
}

// SetIntent updates a player's movement intent, bumps the version, and records
// a patch. All player intent writes must flow through this helper so snapshot
// versions and patch journals stay authoritative.
func (w *World) SetIntent(playerID string, dx, dy float64) {
	if w == nil {
		return
	}

	if math.IsNaN(dx) || math.IsNaN(dy) || math.IsInf(dx, 0) || math.IsInf(dy, 0) {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	w.setActorIntent(&player.actorState, &player.version, playerID, dx, dy)
}

// SetHealth updates a player's health, bumps the version, and records a patch.
// All player health writes must flow through this helper so snapshot versions
// and patch journals stay authoritative.
func (w *World) SetHealth(playerID string, health float64) {
	if w == nil {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	player.stats.Resolve(w.currentTick)
	max := player.stats.GetDerived(stats.DerivedMaxHealth)
	w.setActorHealth(&player.actorState, &player.version, playerID, PatchPlayerHealth, max, health)
}

// MutateInventory applies the provided mutation to a player's inventory while
// preserving snapshot versioning and patch emission. All inventory writes for
// players must flow through this helper.
func (w *World) MutateInventory(playerID string, mutate func(inv *Inventory) error) error {
	if w == nil || mutate == nil {
		return nil
	}

	player, ok := w.players[playerID]
	if !ok {
		return nil
	}

	return w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, mutate)
}

// MutateEquipment applies the provided mutation to an actor's equipment while preserving patches.
// The actor can be either a player or an NPC, and the correct patch kind is emitted automatically.
func (w *World) MutateEquipment(entityID string, mutate func(eq *Equipment) error) error {
	if w == nil || mutate == nil || entityID == "" {
		return nil
	}

	if player, ok := w.players[entityID]; ok {
		return w.mutateActorEquipment(&player.actorState, &player.version, entityID, PatchPlayerEquipment, mutate)
	}

	if npc, ok := w.npcs[entityID]; ok {
		return w.mutateActorEquipment(&npc.actorState, &npc.version, entityID, PatchNPCEquipment, mutate)
	}

	return nil
}

// SetNPCPosition updates an NPC's position, bumps the version, and records a patch.
func (w *World) SetNPCPosition(npcID string, x, y float64) {
	if w == nil {
		return
	}

	npc, ok := w.npcs[npcID]
	if !ok {
		return
	}

	w.setActorPosition(&npc.actorState, &npc.version, npcID, PatchNPCPos, x, y)
}

// SetNPCFacing updates an NPC's facing direction, bumps the version, and records a patch.
func (w *World) SetNPCFacing(npcID string, facing FacingDirection) {
	if w == nil {
		return
	}

	npc, ok := w.npcs[npcID]
	if !ok {
		return
	}

	w.setActorFacing(&npc.actorState, &npc.version, npcID, PatchNPCFacing, facing)
}

// SetNPCHealth updates an NPC's health, bumps the version, and records a patch.
func (w *World) SetNPCHealth(npcID string, health float64) {
	if w == nil {
		return
	}

	npc, ok := w.npcs[npcID]
	if !ok {
		return
	}

	npc.stats.Resolve(w.currentTick)
	max := npc.stats.GetDerived(stats.DerivedMaxHealth)
	w.setActorHealth(&npc.actorState, &npc.version, npcID, PatchNPCHealth, max, health)
}

// MutateNPCInventory applies a mutation to an NPC inventory with versioning and patches.
func (w *World) MutateNPCInventory(npcID string, mutate func(inv *Inventory) error) error {
	if w == nil || mutate == nil {
		return nil
	}

	npc, ok := w.npcs[npcID]
	if !ok {
		return nil
	}

	return w.mutateActorInventory(&npc.actorState, &npc.version, npcID, PatchNPCInventory, mutate)
}

// SetEffectPosition updates an effect's position, bumps the version, and records a patch.
func (w *World) SetEffectPosition(eff *effectState, x, y float64) {
	if w == nil || eff == nil {
		return
	}

	if positionsEqual(eff.X, eff.Y, x, y) {
		return
	}

	oldX := eff.X
	oldY := eff.Y
	eff.X = x
	eff.Y = y
	if w.effectsIndex != nil {
		if !w.effectsIndex.Upsert(eff) {
			eff.X = oldX
			eff.Y = oldY
			_ = w.effectsIndex.Upsert(eff)
			return
		}
	}
	eff.version++

	w.appendPatch(PatchEffectPos, eff.ID, EffectPosPayload{X: x, Y: y})
	w.recordEffectUpdate(eff, "position")
}

// SetEffectParam updates or inserts a parameter for an effect and records a patch when it changes.
func (w *World) SetEffectParam(eff *effectState, key string, value float64) {
	if w == nil || eff == nil || key == "" {
		return
	}

	if eff.Params == nil {
		eff.Params = make(map[string]float64)
	}
	current, exists := eff.Params[key]
	if exists && math.Abs(current-value) < intentEpsilon {
		return
	}

	eff.Params[key] = value
	eff.version++

	w.appendPatch(PatchEffectParams, eff.ID, EffectParamsPayload{Params: cloneEffectParams(eff.Params)})
	w.recordEffectUpdate(eff, "param")
}

// SetGroundItemPosition updates a ground item's position, bumps the version, and records a patch.
func (w *World) SetGroundItemPosition(item *groundItemState, x, y float64) {
	if w == nil || item == nil {
		return
	}

	if positionsEqual(item.X, item.Y, x, y) {
		return
	}

	item.X = x
	item.Y = y
	item.version++

	w.appendPatch(PatchGroundItemPos, item.ID, GroundItemPosPayload{X: x, Y: y})
}

// SetGroundItemQuantity updates a ground item's quantity, bumps the version, and records a patch.
func (w *World) SetGroundItemQuantity(item *groundItemState, qty int) {
	if w == nil || item == nil {
		return
	}

	if qty < 0 {
		qty = 0
	}

	if item.Qty == qty {
		return
	}

	item.Qty = qty
	item.version++

	w.appendPatch(PatchGroundItemQty, item.ID, GroundItemQtyPayload{Qty: qty})
}

func cloneEffectParams(params map[string]float64) map[string]float64 {
	if len(params) == 0 {
		return nil
	}
	cloned := make(map[string]float64, len(params))
	for k, v := range params {
		cloned[k] = v
	}
	return cloned
}

func inventoriesEqual(a, b Inventory) bool {
	if len(a.Slots) != len(b.Slots) {
		return false
	}
	for i := range a.Slots {
		as := a.Slots[i]
		bs := b.Slots[i]
		if as.Slot != bs.Slot {
			return false
		}
		if as.Item.Type != bs.Item.Type {
			return false
		}
		if as.Item.FungibilityKey != bs.Item.FungibilityKey {
			return false
		}
		if as.Item.Quantity != bs.Item.Quantity {
			return false
		}
	}
	return true
}

func cloneInventorySlots(slots []InventorySlot) []InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}
