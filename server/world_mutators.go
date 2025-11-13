package server

import (
	"math"

	journalpkg "mine-and-die/server/internal/journal"
	worldpkg "mine-and-die/server/internal/world"
	worldeffects "mine-and-die/server/internal/world/effects"
	stats "mine-and-die/server/stats"
)

const intentEpsilon = 1e-6

func (w *World) appendPatch(kind PatchKind, entityID string, payload any) {
	if w == nil || entityID == "" {
		return
	}
	w.AppendPatch(Patch{Kind: kind, EntityID: entityID, Payload: payload})
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
	w.PurgeEntity(entityID)
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

	w.appendPatch(kind, entityID, FacingPayload{Facing: toSimFacing(facing)})
}

func (w *World) setActorIntent(actor *actorState, version *uint64, entityID string, dx, dy float64) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	if math.Abs(actor.IntentX-dx) < intentEpsilon && math.Abs(actor.IntentY-dy) < intentEpsilon {
		return
	}

	actor.IntentX = dx
	actor.IntentY = dy
	incrementVersion(version)

	w.appendPatch(PatchPlayerIntent, entityID, PlayerIntentPayload{DX: dx, DY: dy})
}

func (w *World) setActorHealth(actor *actorState, version *uint64, entityID string, kind PatchKind, computedMax float64, health float64) {
	if w == nil || actor == nil || version == nil || entityID == "" {
		return
	}

	emit := func(pk journalpkg.PatchKind, id string, payload any) {
		w.appendPatch(PatchKind(pk), id, payload)
	}
	_ = worldpkg.ApplyActorHealth(actor, version, entityID, journalpkg.PatchKind(kind), computedMax, health, emit)
}

func (w *World) mutateActorInventory(actor *actorState, version *uint64, entityID string, kind PatchKind, mutate func(inv *Inventory) error) error {
	if w == nil || actor == nil || version == nil || entityID == "" || mutate == nil {
		return nil
	}

	emit := func(pk journalpkg.PatchKind, id string, payload any) {
		w.appendPatch(PatchKind(pk), id, payload)
	}
	return worldpkg.MutateActorInventory(actor, version, entityID, journalpkg.PatchKind(kind), mutate, emit)
}

func (w *World) mutateActorEquipment(actor *actorState, version *uint64, entityID string, kind PatchKind, mutate func(eq *Equipment) error) error {
	if w == nil || actor == nil || version == nil || entityID == "" || mutate == nil {
		return nil
	}

	emit := func(pk journalpkg.PatchKind, id string, payload any) {
		w.appendPatch(PatchKind(pk), id, payload)
	}
	return worldpkg.MutateActorEquipment(actor, version, entityID, journalpkg.PatchKind(kind), mutate, emit)
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

	w.setActorPosition(&player.ActorState, &player.Version, playerID, PatchPlayerPos, x, y)
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

	w.setActorFacing(&player.ActorState, &player.Version, playerID, PatchPlayerFacing, facing)
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

	w.setActorIntent(&player.ActorState, &player.Version, playerID, dx, dy)
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

	player.Stats.Resolve(w.currentTick)
	max := player.Stats.GetDerived(stats.DerivedMaxHealth)
	w.setActorHealth(&player.ActorState, &player.Version, playerID, PatchPlayerHealth, max, health)
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

	return w.mutateActorInventory(&player.ActorState, &player.Version, playerID, PatchPlayerInventory, mutate)
}

// MutateEquipment applies the provided mutation to an actor's equipment while preserving patches.
// The actor can be either a player or an NPC, and the correct patch kind is emitted automatically.
func (w *World) MutateEquipment(entityID string, mutate func(eq *Equipment) error) error {
	if w == nil || mutate == nil || entityID == "" {
		return nil
	}

	if player, ok := w.players[entityID]; ok {
		return w.mutateActorEquipment(&player.ActorState, &player.Version, entityID, PatchPlayerEquipment, mutate)
	}

	if npc, ok := w.npcs[entityID]; ok {
		return w.mutateActorEquipment(&npc.ActorState, &npc.Version, entityID, PatchNPCEquipment, mutate)
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

	w.setActorPosition(&npc.ActorState, &npc.Version, npcID, PatchNPCPos, x, y)
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

	w.setActorFacing(&npc.ActorState, &npc.Version, npcID, PatchNPCFacing, facing)
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

	npc.Stats.Resolve(w.currentTick)
	max := npc.Stats.GetDerived(stats.DerivedMaxHealth)
	w.setActorHealth(&npc.ActorState, &npc.Version, npcID, PatchNPCHealth, max, health)
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

	return w.mutateActorInventory(&npc.ActorState, &npc.Version, npcID, PatchNPCInventory, mutate)
}

// SetEffectPosition updates an effect's position, bumps the version, and records a patch.
func (w *World) SetEffectPosition(eff *effectState, x, y float64) {
	if w == nil || eff == nil {
		return
	}

	changed := worldeffects.SetPosition(
		eff,
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

	eff.Version++

	w.appendPatch(PatchEffectPos, eff.ID, EffectPosPayload{X: eff.X, Y: eff.Y})
	w.recordEffectUpdate(eff, "position")
}

// SetEffectParam updates or inserts a parameter for an effect and records a patch when it changes.
func (w *World) SetEffectParam(eff *effectState, key string, value float64) {
	if w == nil || eff == nil || key == "" {
		return
	}

	if !worldeffects.SetParam(eff, key, value) {
		return
	}

	eff.Version++

	w.appendPatch(PatchEffectParams, eff.ID, EffectParamsPayload{Params: cloneEffectParams(eff.Params)})
	w.recordEffectUpdate(eff, "param")
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
