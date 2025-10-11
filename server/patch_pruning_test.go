package main

import (
	"encoding/json"
	"testing"
	"time"

	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestNPCRemovalPurgesPatches(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	npc := &npcState{actorState: actorState{Actor: Actor{ID: "npc-test", Health: 50, MaxHealth: 50}}, stats: stats.DefaultComponent(stats.ArchetypeGoblin), Type: NPCTypeGoblin}
	w.npcs[npc.ID] = npc

	tile := tileForPosition(npc.X, npc.Y)
	def, _ := ItemDefinitionFor(ItemTypeGold)
	existing := &groundItemState{GroundItem: GroundItem{ID: "ground-existing", Type: ItemTypeGold, FungibilityKey: def.FungibilityKey, Qty: 1}, tile: tile}
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	now := time.Now()
	expired := &effectState{Effect: Effect{ID: "effect-test", Type: effectTypeFireball}, expiresAt: now.Add(-time.Second)}
	alive := &effectState{Effect: Effect{ID: "effect-alive", Type: effectTypeFireball}, expiresAt: now.Add(time.Second)}
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

	data, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	var playerPatchKinds []PatchKind
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
	wantKinds := []PatchKind{PatchPlayerPos, PatchPlayerFacing}
	if len(playerPatchKinds) != len(wantKinds) {
		t.Fatalf("expected %d player patches after filtering, got %d", len(wantKinds), len(playerPatchKinds))
	}
	for i, kind := range playerPatchKinds {
		if kind != wantKinds[i] {
			t.Fatalf("expected player patch order %v, got %v", wantKinds, playerPatchKinds)
		}
	}
}
