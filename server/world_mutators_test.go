package server

import (
	"math"
	"testing"
	"time"

	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestSetPositionNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-1", X: 10, Y: 20, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetPosition("player-1", 10, 20)

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetPositionRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-2", X: 5, Y: 6, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetPosition("player-2", 15, 25)

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerPos {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerPos, patch.Kind)
	}
	if patch.EntityID != "player-2" {
		t.Fatalf("expected entity id player-2, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerPosPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerPosPayload, got %T", patch.Payload)
	}
	if payload.X != 15 || payload.Y != 25 {
		t.Fatalf("expected payload coords (15,25), got (%.2f, %.2f)", payload.X, payload.Y)
	}

	w.SetPosition("player-2", 30, 35)
	if player.version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.version)
	}
}

func TestSetFacingNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-3", Facing: FacingRight, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetFacing("player-3", FacingRight)

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetFacingRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-4", Facing: FacingUp, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetFacing("player-4", FacingLeft)

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerFacing {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerFacing, patch.Kind)
	}
	if patch.EntityID != "player-4" {
		t.Fatalf("expected entity id player-4, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerFacingPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerFacingPayload, got %T", patch.Payload)
	}
	if payload.Facing != FacingLeft {
		t.Fatalf("expected payload facing %q, got %q", FacingLeft, payload.Facing)
	}

	w.SetFacing("player-4", FacingDown)
	if player.version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.version)
	}
}

func TestSetIntentNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-intent-noop", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	player.intentX = 0.25
	player.intentY = -0.5
	w.AddPlayer(player)

	w.SetIntent("player-intent-noop", 0.25, -0.5)

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetIntentRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-intent", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetIntent("player-intent", 1, 0)

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerIntent {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerIntent, patch.Kind)
	}
	if patch.EntityID != "player-intent" {
		t.Fatalf("expected entity id player-intent, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerIntentPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerIntentPayload, got %T", patch.Payload)
	}
	if math.Abs(payload.DX-1) > 1e-6 || math.Abs(payload.DY-0) > 1e-6 {
		t.Fatalf("expected payload vector (1,0), got (%.2f, %.2f)", payload.DX, payload.DY)
	}

	w.SetIntent("player-intent", 0, -1)
	if player.version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.version)
	}
}

func TestSetHealthNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-5", Health: 75, MaxHealth: 100}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetHealth("player-5", 75)

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetHealthRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-6", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetHealth("player-6", baselinePlayerMaxHealth-25)

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerHealth {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerHealth, patch.Kind)
	}
	if patch.EntityID != "player-6" {
		t.Fatalf("expected entity id player-6, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerHealthPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerHealthPayload, got %T", patch.Payload)
	}
	if math.Abs(payload.Health-(baselinePlayerMaxHealth-25)) > 1e-6 {
		t.Fatalf("expected payload health %.2f, got %.2f", baselinePlayerMaxHealth-25, payload.Health)
	}
	if math.Abs(payload.MaxHealth-baselinePlayerMaxHealth) > 1e-6 {
		t.Fatalf("expected payload max health %.2f, got %.2f", baselinePlayerMaxHealth, payload.MaxHealth)
	}

	w.SetHealth("player-6", baselinePlayerMaxHealth)
	if player.version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.version)
	}
}

func TestResolveStatsEmitsPatchWhenMaxHealthChanges(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-max-sync", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	// Clear any staged patches from player onboarding.
	w.drainPatchesLocked()

	delta := stats.NewStatDelta()
	delta.Add[stats.StatMight] = 4
	player.stats.Apply(stats.CommandStatChange{
		Layer:  stats.LayerPermanent,
		Source: stats.SourceKey{Kind: stats.SourceKindProgression, ID: "test"},
		Delta:  delta,
	})

	w.resolveStats(w.currentTick)

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch after max health change, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerHealth {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerHealth, patch.Kind)
	}
	if patch.EntityID != "player-max-sync" {
		t.Fatalf("expected entity id player-max-sync, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerHealthPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerHealthPayload, got %T", patch.Payload)
	}

	expectedMax := player.stats.GetDerived(stats.DerivedMaxHealth)
	if math.Abs(payload.Health-baselinePlayerMaxHealth) > 1e-6 {
		t.Fatalf("expected payload health %.2f, got %.2f", baselinePlayerMaxHealth, payload.Health)
	}
	if math.Abs(payload.MaxHealth-expectedMax) > 1e-6 {
		t.Fatalf("expected payload max health %.2f, got %.2f", expectedMax, payload.MaxHealth)
	}
}

func TestApplyEffectHitPlayerEmitsHealthPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-7", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	eff := &effectState{
		Type:   effectTypeAttack,
		Owner:  "attacker-1",
		Params: map[string]float64{"healthDelta": -15},
	}

	w.applyEffectHitPlayer(eff, player, time.Now())

	expected := baselinePlayerMaxHealth - 15
	if math.Abs(player.Health-expected) > 1e-6 {
		t.Fatalf("expected player health to drop to %.2f, got %.2f", expected, player.Health)
	}
	if player.version != 1 {
		t.Fatalf("expected version to increment to 1 after damage, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch after damage, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerHealth {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerHealth, patch.Kind)
	}
	payload, ok := patch.Payload.(PlayerHealthPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerHealthPayload, got %T", patch.Payload)
	}
	if math.Abs(payload.Health-expected) > 1e-6 {
		t.Fatalf("expected payload health %.2f, got %.2f", expected, payload.Health)
	}
}

func TestMutateInventoryNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-8", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: NewInventory()}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if err := w.MutateInventory("player-8", func(inv *Inventory) error { return nil }); err != nil {
		t.Fatalf("expected mutate to succeed, got %v", err)
	}

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestMutateInventoryRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-9", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: NewInventory()}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if err := w.MutateInventory("player-9", func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 3})
		return err
	}); err != nil {
		t.Fatalf("expected mutate to succeed, got %v", err)
	}

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerInventory {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerInventory, patch.Kind)
	}
	if patch.EntityID != "player-9" {
		t.Fatalf("expected patch entity player-9, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerInventoryPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerInventoryPayload, got %T", patch.Payload)
	}
	if len(payload.Slots) != 1 {
		t.Fatalf("expected payload to contain 1 slot, got %d", len(payload.Slots))
	}
	slot := payload.Slots[0]
	if slot.Slot != 0 {
		t.Fatalf("expected slot index 0, got %d", slot.Slot)
	}
	if slot.Item.Type != ItemTypeGold {
		t.Fatalf("expected slot item type %q, got %q", ItemTypeGold, slot.Item.Type)
	}
	if slot.Item.Quantity != 3 {
		t.Fatalf("expected slot quantity 3, got %d", slot.Item.Quantity)
	}
}

func TestMutateInventoryEmitsPatchWhenFungibilityChanges(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	daggerDef, ok := ItemDefinitionFor(ItemTypeIronDagger)
	if !ok {
		t.Fatalf("expected definition for %q", ItemTypeIronDagger)
	}

	initialInventory := Inventory{Slots: []InventorySlot{{
		Slot: 0,
		Item: ItemStack{Type: ItemTypeIronDagger, FungibilityKey: daggerDef.FungibilityKey, Quantity: 1},
	}}}

	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-9-fungibility", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: initialInventory}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches after adding player, got %d", len(patches))
	}

	newKey := daggerDef.FungibilityKey + "::unique"

	if err := w.MutateInventory("player-9-fungibility", func(inv *Inventory) error {
		if len(inv.Slots) != 1 {
			t.Fatalf("expected inventory to contain 1 slot, got %d", len(inv.Slots))
		}
		inv.Slots[0].Item.FungibilityKey = newKey
		return nil
	}); err != nil {
		t.Fatalf("expected mutate to succeed, got %v", err)
	}

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerInventory {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerInventory, patch.Kind)
	}
	if patch.EntityID != "player-9-fungibility" {
		t.Fatalf("expected patch entity player-9-fungibility, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerInventoryPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerInventoryPayload, got %T", patch.Payload)
	}
	if len(payload.Slots) != 1 {
		t.Fatalf("expected payload to contain 1 slot, got %d", len(payload.Slots))
	}
	slot := payload.Slots[0]
	if slot.Item.FungibilityKey != newKey {
		t.Fatalf("expected payload fungibility key %q, got %q", newKey, slot.Item.FungibilityKey)
	}
}

func TestMutateInventoryErrorRestoresState(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-10", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: NewInventory()}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if err := w.MutateInventory("player-10", func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 4})
		return err
	}); err != nil {
		t.Fatalf("expected initial mutate to succeed, got %v", err)
	}

	if err := w.MutateInventory("player-10", func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 0})
		return err
	}); err == nil {
		t.Fatalf("expected mutate to return error for invalid quantity")
	}

	if qty := player.Inventory.QuantityOf(ItemTypeGold); qty != 4 {
		t.Fatalf("expected inventory to retain 4 gold, got %d", qty)
	}

	if player.version != 1 {
		t.Fatalf("expected version to remain 1 after failed mutate, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected a single patch from the successful mutate, got %d", len(patches))
	}
}

func TestSetNPCPositionRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{actorState: actorState{Actor: Actor{ID: "npc-1", X: 1, Y: 2}}, stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-1": npc}

	w.SetNPCPosition("npc-1", 10, 20)

	if npc.version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchNPCPos {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCPos, patch.Kind)
	}
	payload, ok := patch.Payload.(NPCPosPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCPosPayload, got %T", patch.Payload)
	}
	if payload.X != 10 || payload.Y != 20 {
		t.Fatalf("expected payload coords (10,20), got (%.2f, %.2f)", payload.X, payload.Y)
	}
}

func TestSetNPCFacingRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{actorState: actorState{Actor: Actor{ID: "npc-2", Facing: FacingUp}}, stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-2": npc}

	w.SetNPCFacing("npc-2", FacingLeft)

	if npc.version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != PatchNPCFacing {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCFacing, patch.Kind)
	}
	payload, ok := patch.Payload.(NPCFacingPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCFacingPayload, got %T", patch.Payload)
	}
	if payload.Facing != FacingLeft {
		t.Fatalf("expected payload facing %q, got %q", FacingLeft, payload.Facing)
	}
}

func TestSetNPCHealthRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{actorState: actorState{Actor: Actor{ID: "npc-3", Health: 50, MaxHealth: 100}}, stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-3": npc}

	w.SetNPCHealth("npc-3", 20)

	if npc.version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.version)
	}
	if math.Abs(npc.Health-20) > 1e-6 {
		t.Fatalf("expected npc health to be 20, got %.2f", npc.Health)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != PatchNPCHealth {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCHealth, patch.Kind)
	}
	payload, ok := patch.Payload.(NPCHealthPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCHealthPayload, got %T", patch.Payload)
	}
	if math.Abs(payload.Health-20) > 1e-6 {
		t.Fatalf("expected payload health 20, got %.2f", payload.Health)
	}
}

func TestMutateNPCInventoryRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{actorState: actorState{Actor: Actor{ID: "npc-4", Inventory: NewInventory()}}, stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-4": npc}

	if err := w.MutateNPCInventory("npc-4", func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 2})
		return err
	}); err != nil {
		t.Fatalf("unexpected error mutating npc inventory: %v", err)
	}

	if npc.version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != PatchNPCInventory {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCInventory, patch.Kind)
	}
	payload, ok := patch.Payload.(NPCInventoryPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCInventoryPayload, got %T", patch.Payload)
	}
	if len(payload.Slots) != 1 {
		t.Fatalf("expected payload to contain 1 slot, got %d", len(payload.Slots))
	}
	if payload.Slots[0].Item.Quantity != 2 {
		t.Fatalf("expected slot quantity 2, got %d", payload.Slots[0].Item.Quantity)
	}
}

func TestSetEffectPositionRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	eff := &effectState{ID: "effect-1", X: 1, Y: 2}

	w.SetEffectPosition(eff, 5, 7)

	if eff.version != 1 {
		t.Fatalf("expected effect version to increment, got %d", eff.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != PatchEffectPos {
		t.Fatalf("expected patch kind %q, got %q", PatchEffectPos, patch.Kind)
	}
	payload, ok := patch.Payload.(EffectPosPayload)
	if !ok {
		t.Fatalf("expected payload to be EffectPosPayload, got %T", patch.Payload)
	}
	if payload.X != 5 || payload.Y != 7 {
		t.Fatalf("expected payload coords (5,7), got (%.2f, %.2f)", payload.X, payload.Y)
	}
}

func TestSetEffectParamRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	eff := &effectState{ID: "effect-2"}

	w.SetEffectParam(eff, "remainingRange", 3.5)

	if eff.version != 1 {
		t.Fatalf("expected effect version to increment, got %d", eff.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != PatchEffectParams {
		t.Fatalf("expected patch kind %q, got %q", PatchEffectParams, patch.Kind)
	}
	payload, ok := patch.Payload.(EffectParamsPayload)
	if !ok {
		t.Fatalf("expected payload to be EffectParamsPayload, got %T", patch.Payload)
	}
	if payload.Params["remainingRange"] != 3.5 {
		t.Fatalf("expected remainingRange 3.5, got %.2f", payload.Params["remainingRange"])
	}
}

func TestSetGroundItemQuantityRecordsPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	item := &groundItemState{GroundItem: GroundItem{ID: "ground-1", Qty: 1, X: 0, Y: 0}}

	w.SetGroundItemQuantity(item, 5)

	if item.version != 1 {
		t.Fatalf("expected item version to increment, got %d", item.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != PatchGroundItemQty {
		t.Fatalf("expected patch kind %q, got %q", PatchGroundItemQty, patch.Kind)
	}
	payload, ok := patch.Payload.(GroundItemQtyPayload)
	if !ok {
		t.Fatalf("expected payload to be GroundItemQtyPayload, got %T", patch.Payload)
	}
	if payload.Qty != 5 {
		t.Fatalf("expected quantity 5, got %d", payload.Qty)
	}
}
