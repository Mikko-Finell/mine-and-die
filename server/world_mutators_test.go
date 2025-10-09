package main

import (
	"math"
	"testing"
	"time"

	"mine-and-die/server/logging"
)

func TestSetPositionNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-1", X: 10, Y: 20}}}
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-2", X: 5, Y: 6}}}
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-3", Facing: FacingRight}}}
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-4", Facing: FacingUp}}}
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

func TestSetHealthNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-5", Health: 75, MaxHealth: 100}}}
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-6", Health: playerMaxHealth, MaxHealth: playerMaxHealth}}}
	w.AddPlayer(player)

	w.SetHealth("player-6", playerMaxHealth-25)

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
	if math.Abs(payload.Health-(playerMaxHealth-25)) > 1e-6 {
		t.Fatalf("expected payload health %.2f, got %.2f", playerMaxHealth-25, payload.Health)
	}
	if math.Abs(payload.MaxHealth-playerMaxHealth) > 1e-6 {
		t.Fatalf("expected payload max health %.2f, got %.2f", playerMaxHealth, payload.MaxHealth)
	}

	w.SetHealth("player-6", playerMaxHealth)
	if player.version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.version)
	}
}

func TestApplyEffectHitPlayerEmitsHealthPatch(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-7", Health: playerMaxHealth, MaxHealth: playerMaxHealth}}}
	w.AddPlayer(player)

	eff := &effectState{
		Effect: Effect{
			Type:   effectTypeAttack,
			Owner:  "attacker-1",
			Params: map[string]float64{"healthDelta": -15},
		},
	}

	w.applyEffectHitPlayer(eff, player, time.Now())

	expected := playerMaxHealth - 15
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-8", Inventory: NewInventory()}}}
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
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-9", Inventory: NewInventory()}}}
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

func TestMutateInventoryErrorRestoresState(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-10", Inventory: NewInventory()}}}
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
