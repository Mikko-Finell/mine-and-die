package main

import "math"

const positionEpsilon = 1e-6
const healthEpsilon = 1e-6
const intentEpsilon = 1e-6

func (w *World) appendPatch(kind PatchKind, entityID string, payload any) {
	if w == nil || entityID == "" {
		return
	}
	w.journal.AppendPatch(Patch{Kind: kind, EntityID: entityID, Payload: payload})
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
	*version++

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
	*version++

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
	*version++

	w.appendPatch(PatchPlayerIntent, entityID, PlayerIntentPayload{DX: dx, DY: dy})
}

func (w *World) setActorHealth(actor *actorState, version *uint64, entityID string, kind PatchKind, fallbackMax float64, health float64) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	if math.IsNaN(health) || math.IsInf(health, 0) {
		return
	}

	max := actor.MaxHealth
	if max <= 0 {
		max = fallbackMax
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

	if math.Abs(actor.Health-health) < healthEpsilon {
		return
	}

	actor.Health = health
	*version++

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

	*version++
	w.appendPatch(kind, entityID, InventoryPayload{Slots: cloneInventorySlots(actor.Inventory.Slots)})
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

	w.setActorHealth(&player.actorState, &player.version, playerID, PatchPlayerHealth, playerMaxHealth, health)
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

	w.setActorHealth(&npc.actorState, &npc.version, npcID, PatchNPCHealth, playerMaxHealth, health)
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

	eff.X = x
	eff.Y = y
	eff.version++

	w.appendPatch(PatchEffectPos, eff.ID, EffectPosPayload{X: x, Y: y})
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
