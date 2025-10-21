package server

import (
	"math"

	worldpkg "mine-and-die/server/internal/world"
	stats "mine-and-die/server/stats"
)

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

	if worldpkg.PositionsEqual(actor.X, actor.Y, x, y) {
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

	state := worldpkg.HealthState{Health: actor.Health, MaxHealth: actor.MaxHealth}
	if !worldpkg.SetActorHealth(&state, computedMax, health) {
		return
	}

	actor.Health = state.Health
	actor.MaxHealth = state.MaxHealth
	incrementVersion(version)

	w.appendPatch(kind, entityID, HealthPayload{Health: state.Health, MaxHealth: state.MaxHealth})
}

func (w *World) mutateActorInventory(actor *actorState, version *uint64, entityID string, kind PatchKind, mutate func(inv *Inventory) error) error {
	if w == nil || actor == nil || version == nil || entityID == "" || mutate == nil {
		return nil
	}

	return worldpkg.MutateActorInventory(
		&actor.Inventory,
		version,
		mutate,
		func(inv Inventory) Inventory { return inv.Clone() },
		inventoriesEqual,
		func(inv Inventory) {
			w.appendPatch(kind, entityID, InventoryPayload{Slots: cloneInventorySlots(inv.Slots)})
		},
	)
}

func (w *World) mutateActorEquipment(actor *actorState, version *uint64, entityID string, kind PatchKind, mutate func(eq *Equipment) error) error {
	if w == nil || actor == nil || version == nil || entityID == "" || mutate == nil {
		return nil
	}

	return worldpkg.MutateActorEquipment(
		&actor.Equipment,
		version,
		mutate,
		func(eq Equipment) Equipment { return eq.Clone() },
		equipmentsEqual,
		func(eq Equipment) {
			w.appendPatch(kind, entityID, EquipmentPayload{Slots: cloneEquipmentSlots(eq.Slots)})
		},
	)
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

	changed := worldpkg.SetEffectPosition(
		&eff.X,
		&eff.Y,
		x,
		y,
		func(oldX, oldY float64) bool {
			if w.effectsIndex == nil {
				return true
			}
			if w.effectsIndex.Upsert(eff) {
				return true
			}
			eff.X = oldX
			eff.Y = oldY
			_ = w.effectsIndex.Upsert(eff)
			return false
		},
	)
	if !changed {
		return
	}

	eff.version++

	w.appendPatch(PatchEffectPos, eff.ID, EffectPosPayload{X: eff.X, Y: eff.Y})
	w.recordEffectUpdate(eff, "position")
}

// SetEffectParam updates or inserts a parameter for an effect and records a patch when it changes.
func (w *World) SetEffectParam(eff *effectState, key string, value float64) {
	if w == nil || eff == nil || key == "" {
		return
	}

	if !worldpkg.SetEffectParam(&eff.Params, key, value) {
		return
	}

	eff.version++

	w.appendPatch(PatchEffectParams, eff.ID, EffectParamsPayload{Params: cloneEffectParams(eff.Params)})
	w.recordEffectUpdate(eff, "param")
}

// SetGroundItemPosition updates a ground item's position, bumps the version, and records a patch.
func (w *World) SetGroundItemPosition(item *groundItemState, x, y float64) {
	if w == nil || item == nil {
		return
	}

	if !worldpkg.SetGroundItemPosition(&item.X, &item.Y, x, y) {
		return
	}

	item.Version++

	w.appendPatch(PatchGroundItemPos, item.ID, GroundItemPosPayload{X: item.X, Y: item.Y})
}

// SetGroundItemQuantity updates a ground item's quantity, bumps the version, and records a patch.
func (w *World) SetGroundItemQuantity(item *groundItemState, qty int) {
	if w == nil || item == nil {
		return
	}

	if !worldpkg.SetGroundItemQuantity(&item.Qty, qty) {
		return
	}

	item.Version++

	w.appendPatch(PatchGroundItemQty, item.ID, GroundItemQtyPayload{Qty: item.Qty})
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
