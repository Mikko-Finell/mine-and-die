package server

import (
	"encoding/json"
	"testing"
	"time"

	"mine-and-die/server/internal/sim"
	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestNPCRemovalPurgesPatches(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{actorState: actorState{Actor: Actor{ID: "npc-test", Health: 50, MaxHealth: 50}}, stats: stats.DefaultComponent(stats.ArchetypeGoblin), Type: NPCTypeGoblin}
	w.npcs[npc.ID] = npc

	tile := tileForPosition(npc.X, npc.Y)
	def, _ := ItemDefinitionFor(ItemTypeGold)
	existing := &groundItemState{GroundItem: toWorldGroundItem(GroundItem{ID: "ground-existing", Type: ItemTypeGold, FungibilityKey: def.FungibilityKey, Qty: 1}), Tile: tile}
	w.groundItems[existing.ID] = existing
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[groundTileKey]map[string]*groundItemState)
	}
	w.groundItemsByTile[tile] = map[string]*groundItemState{def.FungibilityKey: existing}

	w.SetNPCHealth(npc.ID, 0)
	if err := w.MutateNPCInventory(npc.ID, func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
		return err
	}); err != nil {
		t.Fatalf("mutate npc inventory failed: %v", err)
	}

	w.handleNPCDefeat(npc)

	patches := w.snapshotPatchesLocked()
	foundGroundPatch := false
	for _, patch := range patches {
		if patch.EntityID == npc.ID {
			t.Fatalf("expected npc patches to be purged, found kind %q", patch.Kind)
		}
		if patch.Kind == PatchGroundItemQty || patch.Kind == PatchGroundItemPos {
			foundGroundPatch = true
		}
	}
	if !foundGroundPatch {
		t.Fatalf("expected ground item patches to remain after npc defeat")
	}
}

func TestEffectExpiryPurgesPatches(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	now := time.Now()
	expired := &effectState{ID: "effect-test", Type: effectTypeFireball, expiresAt: now.Add(-time.Second)}
	alive := &effectState{ID: "effect-alive", Type: effectTypeFireball, expiresAt: now.Add(time.Second)}
	w.effects = append(w.effects, expired, alive)

	w.SetEffectPosition(expired, 10, 15)
	w.SetEffectParam(expired, "radius", 2.5)
	w.SetEffectParam(alive, "radius", 1.5)

	w.pruneEffects(now)

	foundAlivePatch := false
	for _, patch := range w.snapshotPatchesLocked() {
		if patch.EntityID == expired.ID {
			t.Fatalf("expected effect patches to be purged, found kind %q", patch.Kind)
		}
		if patch.EntityID == alive.ID {
			foundAlivePatch = true
		}
	}
	if !foundAlivePatch {
		t.Fatalf("expected surviving effect patches to remain after pruning")
	}
}

func TestMarshalStateOmitsUnknownEntityPatches(t *testing.T) {
	hub := newHub()

	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-patch", Facing: FacingDown, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}

	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.world.appendPatch(PatchPlayerPos, player.ID, PlayerPosPayload{X: 3, Y: 4})
	hub.world.appendPatch(PatchNPCHealth, "npc-phantom", NPCHealthPayload{Health: 0, MaxHealth: baselinePlayerMaxHealth})
	hub.world.appendPatch(PatchPlayerFacing, player.ID, PlayerFacingPayload{Facing: FacingUp})
	hub.mu.Unlock()

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	var playerPatchKinds []sim.PatchKind
	for _, patch := range msg.Patches {
		if patch.EntityID == "npc-phantom" {
			t.Fatalf("expected broadcast to omit patches for unknown npc, saw kind %q", patch.Kind)
		}
		if patch.EntityID == player.ID {
			playerPatchKinds = append(playerPatchKinds, patch.Kind)
		}
	}

	if len(playerPatchKinds) == 0 {
		t.Fatalf("expected player patch to survive filtering")
	}
	wantKinds := []sim.PatchKind{sim.PatchPlayerPos, sim.PatchPlayerFacing}
	if len(playerPatchKinds) != len(wantKinds) {
		t.Fatalf("expected %d player patches after filtering, got %d", len(wantKinds), len(playerPatchKinds))
	}
	for i, kind := range playerPatchKinds {
		if kind != wantKinds[i] {
			t.Fatalf("expected player patch order %v, got %v", wantKinds, playerPatchKinds)
		}
	}
}

func TestMarshalStateRetainsEffectPatches(t *testing.T) {
	hub := newHub()

	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-anchor", Facing: FacingDown, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	now := time.Now()
	effect := &effectState{ID: "effect-patch", Type: effectTypeFireball, expiresAt: now.Add(time.Minute)}

	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.world.registerEffect(effect)
	hub.world.SetEffectPosition(effect, 5, 6)
	hub.world.SetEffectParam(effect, "radius", 1.5)
	hub.mu.Unlock()

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	var effectPatchKinds []sim.PatchKind
	for _, patch := range msg.Patches {
		if patch.EntityID == effect.ID {
			effectPatchKinds = append(effectPatchKinds, patch.Kind)
		}
	}

	if len(effectPatchKinds) == 0 {
		t.Fatalf("expected effect patches to survive filtering")
	}

	wantKinds := []sim.PatchKind{sim.PatchEffectPos, sim.PatchEffectParams}
	if len(effectPatchKinds) != len(wantKinds) {
		t.Fatalf("expected %d effect patches after filtering, got %d", len(wantKinds), len(effectPatchKinds))
	}
	for i, kind := range effectPatchKinds {
		if wantKinds[i] != kind {
			t.Fatalf("expected effect patch order %v, got %v", wantKinds, effectPatchKinds)
		}
	}
}

func TestMarshalStateUsesFacadeEffectIDsForFiltering(t *testing.T) {
	hub := newHub()

	effectID := "engine-effect"

	hub.mu.Lock()
	hub.world.appendPatch(PatchEffectParams, effectID, EffectParamsPayload{Params: map[string]float64{"radius": 1}})
	hub.world.effects = nil
	hub.mu.Unlock()

	snapshot := sim.Snapshot{AliveEffectIDs: []string{effectID}}
	engine := newRecordingSimEngine(snapshot)
	t.Cleanup(engine.AllowFurtherSnapshots)
	engine.SetSnapshotPatches([]sim.Patch{{
		Kind:     sim.PatchEffectParams,
		EntityID: effectID,
		Payload:  sim.EffectParamsPayload{Params: map[string]float64{"radius": 1}},
	}})
	hub.engine = engine

	data, _, err := hub.marshalState(nil, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	found := false
	for _, patch := range msg.Patches {
		if patch.EntityID == effectID && patch.Kind == sim.PatchEffectParams {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected effect patch for %q to survive filtering when provided by engine snapshot", effectID)
	}
}
