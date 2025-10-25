package server

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func requireInventorySlots(t *testing.T, slots any) []sim.InventorySlot {
	t.Helper()
	converted := itemspkg.SimInventorySlotsFromAny(slots)
	if converted == nil {
		if slots == nil {
			return nil
		}
		value := reflect.ValueOf(slots)
		if value.IsValid() && value.Kind() == reflect.Slice && value.Len() == 0 {
			return nil
		}
		t.Fatalf("expected inventory slots to be convertible, got %T", slots)
	}
	return converted
}

func requireEquipmentSlots(t *testing.T, slots any) []sim.EquippedItem {
	t.Helper()
	converted := itemspkg.SimEquippedItemsFromAny(slots)
	if converted == nil {
		if slots == nil {
			return nil
		}
		value := reflect.ValueOf(slots)
		if value.IsValid() && value.Kind() == reflect.Slice && value.Len() == 0 {
			return nil
		}
		t.Fatalf("expected equipment slots to be convertible, got %T", slots)
	}
	return converted
}

func TestMutateEquipmentRecordsPlayerPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-record", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: NewEquipment()}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	stack := ItemStack{Type: ItemTypeIronDagger, Quantity: 1}

	if err := w.MutateEquipment("player-equipment-record", func(eq *Equipment) error {
		eq.Set(EquipSlotMainHand, stack)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating equipment: %v", err)
	}

	if player.Version != 1 {
		t.Fatalf("expected player version to increment, got %d", player.Version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerEquipment {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerEquipment, patch.Kind)
	}
	if patch.EntityID != "player-equipment-record" {
		t.Fatalf("expected patch entity player-equipment-record, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerEquipmentPayload, got %T", patch.Payload)
	}

	expected := itemspkg.SimEquipmentPayloadFromSlots[sim.EquippedItem, PlayerEquipmentPayload](
		itemspkg.SimEquippedItemsFromAny([]EquippedItem{{
			Slot: EquipSlotMainHand,
			Item: stack,
		}}),
	)

	expectedSlots := requireEquipmentSlots(t, expected.Slots)
	slots := requireEquipmentSlots(t, payload.Slots)
	if len(slots) != len(expectedSlots) {
		t.Fatalf("expected payload to contain %d slot(s), got %d", len(expectedSlots), len(slots))
	}

	got := slots[0]
	want := expectedSlots[0]
	if got.Slot != want.Slot {
		t.Fatalf("expected slot %q, got %q", want.Slot, got.Slot)
	}
	if got.Item.Type != want.Item.Type {
		t.Fatalf("expected slot item type %q, got %q", want.Item.Type, got.Item.Type)
	}
	if got.Item.Quantity != want.Item.Quantity {
		t.Fatalf("expected slot quantity %d, got %d", want.Item.Quantity, got.Item.Quantity)
	}
}

func TestMutateEquipmentEmitsSortedSlots(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-sorted", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: NewEquipment()}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	if err := w.MutateEquipment("player-equipment-sorted", func(eq *Equipment) error {
		eq.Set(EquipSlotAccessory, ItemStack{Type: ItemTypeTravelerCharm, Quantity: 1})
		eq.Set(EquipSlotMainHand, ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::sorted", Quantity: 1})
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating equipment: %v", err)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	payload, ok := patches[0].Payload.(PlayerEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerEquipmentPayload, got %T", patches[0].Payload)
	}

	slots := requireEquipmentSlots(t, payload.Slots)
	if len(slots) != 2 {
		t.Fatalf("expected payload to contain 2 slots, got %d", len(slots))
	}

	if slots[0].Slot != sim.EquipSlot(EquipSlotMainHand) {
		t.Fatalf("expected main hand to appear first, got %q", slots[0].Slot)
	}
	if slots[1].Slot != sim.EquipSlot(EquipSlotAccessory) {
		t.Fatalf("expected accessory to appear second, got %q", slots[1].Slot)
	}

	expected := itemspkg.SimEquipmentPayloadFromSlots[sim.EquippedItem, PlayerEquipmentPayload](
		itemspkg.SimEquippedItemsFromAny([]EquippedItem{{
			Slot: EquipSlotMainHand,
			Item: ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::sorted", Quantity: 1},
		}, {
			Slot: EquipSlotAccessory,
			Item: ItemStack{Type: ItemTypeTravelerCharm, Quantity: 1},
		}}),
	)

	expectedSlots := requireEquipmentSlots(t, expected.Slots)
	for idx := range slots {
		if slots[idx].Slot != expectedSlots[idx].Slot {
			t.Fatalf("expected slot order %q at index %d, got %q", expectedSlots[idx].Slot, idx, slots[idx].Slot)
		}
		if slots[idx].Item.Type != expectedSlots[idx].Item.Type {
			t.Fatalf("expected slot item type %q at index %d, got %q", expectedSlots[idx].Item.Type, idx, slots[idx].Item.Type)
		}
	}
}

func TestMutateEquipmentPatchCloneIndependent(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-clone", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: NewEquipment()}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	original := ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::clone", Quantity: 1}

	if err := w.MutateEquipment("player-equipment-clone", func(eq *Equipment) error {
		eq.Set(EquipSlotMainHand, original)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating equipment: %v", err)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	payload, ok := patches[0].Payload.(PlayerEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerEquipmentPayload, got %T", patches[0].Payload)
	}

	slots := requireEquipmentSlots(t, payload.Slots)
	if len(slots) != 1 {
		t.Fatalf("expected payload to contain 1 slot, got %d", len(slots))
	}

	captured := slots[0]

	player.Equipment.Set(EquipSlotMainHand, ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::mutated", Quantity: 1})

	if slots[0].Item.Type != captured.Item.Type {
		t.Fatalf("expected payload item type to remain %q, got %q", captured.Item.Type, slots[0].Item.Type)
	}
	if slots[0].Slot != captured.Slot {
		t.Fatalf("expected payload slot to remain %q, got %q", captured.Slot, slots[0].Slot)
	}
}

func TestMutateEquipmentUpdatesPlayerSlot(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	initial := NewEquipment()
	initial.Set(EquipSlotMainHand, ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::base", Quantity: 1})

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-update", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: initial}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	upgraded := ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::upgraded", Quantity: 1}

	if err := w.MutateEquipment("player-equipment-update", func(eq *Equipment) error {
		eq.Set(EquipSlotMainHand, upgraded)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating equipment: %v", err)
	}

	if player.Version != 1 {
		t.Fatalf("expected player version to increment, got %d", player.Version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerEquipment {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerEquipment, patch.Kind)
	}

	payload, ok := patch.Payload.(PlayerEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerEquipmentPayload, got %T", patch.Payload)
	}

	expected := itemspkg.SimEquipmentPayloadFromSlots[sim.EquippedItem, PlayerEquipmentPayload](
		itemspkg.SimEquippedItemsFromAny([]EquippedItem{{
			Slot: EquipSlotMainHand,
			Item: upgraded,
		}}),
	)

	expectedSlots := requireEquipmentSlots(t, expected.Slots)
	slots := requireEquipmentSlots(t, payload.Slots)
	if len(slots) != len(expectedSlots) {
		t.Fatalf("expected payload to contain %d slot(s), got %d", len(expectedSlots), len(slots))
	}

	got := slots[0]
	want := expectedSlots[0]
	if got.Slot != want.Slot {
		t.Fatalf("expected slot %q, got %q", want.Slot, got.Slot)
	}
	if got.Item.Type != want.Item.Type {
		t.Fatalf("expected slot item type %q, got %q", want.Item.Type, got.Item.Type)
	}
	if got.Item.Quantity != want.Item.Quantity {
		t.Fatalf("expected slot quantity %d, got %d", want.Item.Quantity, got.Item.Quantity)
	}
}

func TestMutateEquipmentRemovalClearsPlayerPayload(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	initial := NewEquipment()
	initial.Set(EquipSlotMainHand, ItemStack{Type: ItemTypeIronDagger, Quantity: 1})

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-remove", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: initial}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	if err := w.MutateEquipment("player-equipment-remove", func(eq *Equipment) error {
		if _, ok := eq.Remove(EquipSlotMainHand); !ok {
			t.Fatalf("expected equipment to contain main hand entry")
		}
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating equipment: %v", err)
	}

	if player.Version != 1 {
		t.Fatalf("expected player version to increment, got %d", player.Version)
	}

	if len(player.Equipment.Slots) != 0 {
		t.Fatalf("expected equipment to be empty after removal")
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerEquipment {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerEquipment, patch.Kind)
	}

	payload, ok := patch.Payload.(PlayerEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerEquipmentPayload, got %T", patch.Payload)
	}

	slots := requireEquipmentSlots(t, payload.Slots)
	if slots != nil {
		t.Fatalf("expected payload slots to be nil after removal, got %v", slots)
	}
}

func TestMutateEquipmentErrorRestoresPlayerState(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	initial := NewEquipment()
	original := ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::error", Quantity: 1}
	initial.Set(EquipSlotMainHand, original)

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-error", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: initial}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	mutateErr := errors.New("equip failure")

	if err := w.MutateEquipment("player-equipment-error", func(eq *Equipment) error {
		eq.Set(EquipSlotMainHand, ItemStack{Type: ItemTypeIronDagger, FungibilityKey: "dagger::error-updated", Quantity: 1})
		return mutateErr
	}); !errors.Is(err, mutateErr) {
		t.Fatalf("expected mutate to return sentinel error, got %v", err)
	}

	if player.Version != 0 {
		t.Fatalf("expected player version to remain 0 after failed mutate, got %d", player.Version)
	}

	stack, ok := player.Equipment.Get(EquipSlotMainHand)
	if !ok {
		t.Fatalf("expected equipment to retain main hand item")
	}
	if stack.Type != original.Type {
		t.Fatalf("expected equipment to keep item type %q, got %q", original.Type, stack.Type)
	}
	if stack.FungibilityKey != original.FungibilityKey {
		t.Fatalf("expected equipment to keep fungibility key %q, got %q", original.FungibilityKey, stack.FungibilityKey)
	}

	if len(w.snapshotPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after failed mutate")
	}
}

func TestMutateEquipmentNoopDoesNotEmitPlayerPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	initial := NewEquipment()
	kept := ItemStack{Type: ItemTypeIronDagger, Quantity: 1}
	initial.Set(EquipSlotMainHand, kept)

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-equipment-noop", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Equipment: initial}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches after adding player")
	}

	if err := w.MutateEquipment("player-equipment-noop", func(eq *Equipment) error {
		eq.Set(EquipSlotMainHand, kept)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating equipment: %v", err)
	}

	if player.Version != 0 {
		t.Fatalf("expected player version to remain 0 for noop mutate, got %d", player.Version)
	}

	if len(w.snapshotPatchesLocked()) != 0 {
		t.Fatalf("expected no patches for noop mutate")
	}
}

func TestMutateEquipmentRecordsNPCPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-equipment-record", Equipment: NewEquipment()}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-equipment-record": npc}

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches before mutating npc equipment")
	}

	stack := ItemStack{Type: ItemTypeIronDagger, Quantity: 1}

	if err := w.MutateEquipment("npc-equipment-record", func(eq *Equipment) error {
		eq.Set(EquipSlotOffHand, stack)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating npc equipment: %v", err)
	}

	if npc.Version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.Version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchNPCEquipment {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCEquipment, patch.Kind)
	}
	if patch.EntityID != "npc-equipment-record" {
		t.Fatalf("expected patch entity npc-equipment-record, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(NPCEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCEquipmentPayload, got %T", patch.Payload)
	}

	expected := itemspkg.SimEquipmentPayloadFromSlots[sim.EquippedItem, NPCEquipmentPayload](
		itemspkg.SimEquippedItemsFromAny([]EquippedItem{{
			Slot: EquipSlotOffHand,
			Item: stack,
		}}),
	)

	expectedSlots := requireEquipmentSlots(t, expected.Slots)
	slots := requireEquipmentSlots(t, payload.Slots)
	if len(slots) != len(expectedSlots) {
		t.Fatalf("expected payload to contain %d slot(s), got %d", len(expectedSlots), len(slots))
	}

	got := slots[0]
	want := expectedSlots[0]
	if got.Slot != want.Slot {
		t.Fatalf("expected slot %q, got %q", want.Slot, got.Slot)
	}
	if got.Item.Type != want.Item.Type {
		t.Fatalf("expected slot item type %q, got %q", want.Item.Type, got.Item.Type)
	}
	if got.Item.Quantity != want.Item.Quantity {
		t.Fatalf("expected slot quantity %d, got %d", want.Item.Quantity, got.Item.Quantity)
	}
}

func TestMutateEquipmentRemovalClearsNPCPayload(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	initial := NewEquipment()
	initial.Set(EquipSlotOffHand, ItemStack{Type: ItemTypeIronDagger, Quantity: 1})

	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-equipment-remove", Equipment: initial}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-equipment-remove": npc}

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches before mutating npc equipment")
	}

	if err := w.MutateEquipment("npc-equipment-remove", func(eq *Equipment) error {
		if _, ok := eq.Remove(EquipSlotOffHand); !ok {
			t.Fatalf("expected equipment to contain off hand entry")
		}
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating npc equipment: %v", err)
	}

	if npc.Version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.Version)
	}

	if len(npc.Equipment.Slots) != 0 {
		t.Fatalf("expected npc equipment to be empty after removal")
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchNPCEquipment {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCEquipment, patch.Kind)
	}

	payload, ok := patch.Payload.(NPCEquipmentPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCEquipmentPayload, got %T", patch.Payload)
	}

	slots := requireEquipmentSlots(t, payload.Slots)
	if slots != nil {
		t.Fatalf("expected payload slots to be nil after removal, got %v", slots)
	}
}

func TestMutateEquipmentNoopDoesNotEmitNPCPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	initial := NewEquipment()
	kept := ItemStack{Type: ItemTypeIronDagger, Quantity: 1}
	initial.Set(EquipSlotOffHand, kept)

	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-equipment-noop", Equipment: initial}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-equipment-noop": npc}

	if len(w.drainPatchesLocked()) != 0 {
		t.Fatalf("expected no patches before mutating npc equipment")
	}

	if err := w.MutateEquipment("npc-equipment-noop", func(eq *Equipment) error {
		eq.Set(EquipSlotOffHand, kept)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error mutating npc equipment: %v", err)
	}

	if npc.Version != 0 {
		t.Fatalf("expected npc version to remain 0 for noop mutate, got %d", npc.Version)
	}

	if len(w.snapshotPatchesLocked()) != 0 {
		t.Fatalf("expected no patches for npc noop mutate")
	}
}

func TestSetPositionNoopDoesNotEmitPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-1", X: 10, Y: 20, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetPosition("player-1", 10, 20)

	if player.Version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.Version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetPositionRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-2", X: 5, Y: 6, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetPosition("player-2", 15, 25)

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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
	if player.Version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.Version)
	}
}

func TestSetFacingNoopDoesNotEmitPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-3", Facing: FacingRight, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetFacing("player-3", FacingRight)

	if player.Version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.Version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetFacingRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-4", Facing: FacingUp, Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetFacing("player-4", FacingLeft)

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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
	if payload.Facing != sim.FacingDirection(FacingLeft) {
		t.Fatalf("expected payload facing %q, got %q", FacingLeft, payload.Facing)
	}

	w.SetFacing("player-4", FacingDown)
	if player.Version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.Version)
	}
}

func TestSetIntentNoopDoesNotEmitPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-intent-noop", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	player.IntentX = 0.25
	player.IntentY = -0.5
	w.AddPlayer(player)

	w.SetIntent("player-intent-noop", 0.25, -0.5)

	if player.Version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.Version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetIntentRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-intent", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetIntent("player-intent", 1, 0)

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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
	if player.Version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.Version)
	}
}

func TestSetHealthNoopDoesNotEmitPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-5", Health: 75, MaxHealth: 100}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetHealth("player-5", 75)

	if player.Version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.Version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetHealthRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-6", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	w.SetHealth("player-6", baselinePlayerMaxHealth-25)

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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
	if player.Version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.Version)
	}
}

func TestResolveStatsEmitsPatchWhenMaxHealthChanges(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-max-sync", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	// Clear any staged patches from player onboarding.
	w.drainPatchesLocked()

	delta := stats.NewStatDelta()
	delta.Add[stats.StatMight] = 4
	player.Stats.Apply(stats.CommandStatChange{
		Layer:  stats.LayerPermanent,
		Source: stats.SourceKey{Kind: stats.SourceKindProgression, ID: "test"},
		Delta:  delta,
	})

	w.resolveStats(w.currentTick)

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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

	expectedMax := player.Stats.GetDerived(stats.DerivedMaxHealth)
	if math.Abs(payload.Health-baselinePlayerMaxHealth) > 1e-6 {
		t.Fatalf("expected payload health %.2f, got %.2f", baselinePlayerMaxHealth, payload.Health)
	}
	if math.Abs(payload.MaxHealth-expectedMax) > 1e-6 {
		t.Fatalf("expected payload max health %.2f, got %.2f", expectedMax, payload.MaxHealth)
	}
}

func TestPlayerHitCallbackEmitsHealthPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-7", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	eff := &effectState{
		Type:   effectTypeAttack,
		Owner:  "attacker-1",
		Params: map[string]float64{"healthDelta": -15},
	}

	w.invokePlayerHitCallback(eff, player, time.Now())

	expected := baselinePlayerMaxHealth - 15
	if math.Abs(player.Health-expected) > 1e-6 {
		t.Fatalf("expected player health to drop to %.2f, got %.2f", expected, player.Health)
	}
	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1 after damage, got %d", player.Version)
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
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-8", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: NewInventory()}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if err := w.MutateInventory("player-8", func(inv *Inventory) error { return nil }); err != nil {
		t.Fatalf("expected mutate to succeed, got %v", err)
	}

	if player.Version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.Version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestMutateInventoryRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-9", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: NewInventory()}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	w.AddPlayer(player)

	if err := w.MutateInventory("player-9", func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 3})
		return err
	}); err != nil {
		t.Fatalf("expected mutate to succeed, got %v", err)
	}

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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

	expected := itemspkg.SimInventoryPayloadFromSlots[sim.InventorySlot, PlayerInventoryPayload](
		itemspkg.SimInventorySlotsFromAny([]InventorySlot{{
			Slot: 0,
			Item: ItemStack{Type: ItemTypeGold, Quantity: 3},
		}}),
	)

	expectedSlots := requireInventorySlots(t, expected.Slots)
	slots := requireInventorySlots(t, payload.Slots)
	if len(slots) != len(expectedSlots) {
		t.Fatalf("expected payload to contain %d slot(s), got %d", len(expectedSlots), len(slots))
	}

	got := slots[0]
	want := expectedSlots[0]
	if got.Slot != want.Slot {
		t.Fatalf("expected slot index %d, got %d", want.Slot, got.Slot)
	}
	if got.Item.Type != want.Item.Type {
		t.Fatalf("expected slot item type %q, got %q", want.Item.Type, got.Item.Type)
	}
	if got.Item.Quantity != want.Item.Quantity {
		t.Fatalf("expected slot quantity %d, got %d", want.Item.Quantity, got.Item.Quantity)
	}
}

func TestMutateInventoryEmitsPatchWhenFungibilityChanges(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	daggerDef, ok := ItemDefinitionFor(ItemTypeIronDagger)
	if !ok {
		t.Fatalf("expected definition for %q", ItemTypeIronDagger)
	}

	initialInventory := itemspkg.InventoryValueFromSlots[InventorySlot, Inventory]([]InventorySlot{{
		Slot: 0,
		Item: ItemStack{Type: ItemTypeIronDagger, FungibilityKey: daggerDef.FungibilityKey, Quantity: 1},
	}})

	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-9-fungibility", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: initialInventory}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
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

	if player.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.Version)
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

	expected := itemspkg.SimInventoryPayloadFromSlots[sim.InventorySlot, PlayerInventoryPayload](
		itemspkg.SimInventorySlotsFromAny([]InventorySlot{{
			Slot: 0,
			Item: ItemStack{Type: ItemTypeIronDagger, FungibilityKey: newKey, Quantity: 1},
		}}),
	)

	expectedSlots := requireInventorySlots(t, expected.Slots)
	slots := requireInventorySlots(t, payload.Slots)
	if len(slots) != len(expectedSlots) {
		t.Fatalf("expected payload to contain %d slot(s), got %d", len(expectedSlots), len(slots))
	}

	got := slots[0]
	want := expectedSlots[0]
	if got.Slot != want.Slot {
		t.Fatalf("expected slot index %d, got %d", want.Slot, got.Slot)
	}
	if got.Item.Type != want.Item.Type {
		t.Fatalf("expected slot item type %q, got %q", want.Item.Type, got.Item.Type)
	}
	if got.Item.Quantity != want.Item.Quantity {
		t.Fatalf("expected slot quantity %d, got %d", want.Item.Quantity, got.Item.Quantity)
	}
	if got.Item.FungibilityKey != want.Item.FungibilityKey {
		t.Fatalf("expected payload fungibility key %q, got %q", want.Item.FungibilityKey, got.Item.FungibilityKey)
	}
}

func TestMutateInventoryErrorRestoresState(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	player := &playerState{ActorState: actorState{Actor: Actor{ID: "player-10", Health: baselinePlayerMaxHealth, MaxHealth: baselinePlayerMaxHealth, Inventory: NewInventory()}}, Stats: stats.DefaultComponent(stats.ArchetypePlayer)}
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

	if player.Version != 1 {
		t.Fatalf("expected version to remain 1 after failed mutate, got %d", player.Version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected a single patch from the successful mutate, got %d", len(patches))
	}
}

func TestSetNPCPositionRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-1", X: 1, Y: 2}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-1": npc}

	w.SetNPCPosition("npc-1", 10, 20)

	if npc.Version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.Version)
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
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-2", Facing: FacingUp}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-2": npc}

	w.SetNPCFacing("npc-2", FacingLeft)

	if npc.Version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.Version)
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
	if payload.Facing != sim.FacingDirection(FacingLeft) {
		t.Fatalf("expected payload facing %q, got %q", FacingLeft, payload.Facing)
	}
}

func TestSetNPCHealthRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-3", Health: 50, MaxHealth: 100}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-3": npc}

	w.SetNPCHealth("npc-3", 20)

	if npc.Version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.Version)
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
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	npc := &npcState{ActorState: actorState{Actor: Actor{ID: "npc-4", Inventory: NewInventory()}}, Stats: stats.DefaultComponent(stats.ArchetypeGoblin)}
	w.npcs = map[string]*npcState{"npc-4": npc}

	if err := w.MutateNPCInventory("npc-4", func(inv *Inventory) error {
		_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 2})
		return err
	}); err != nil {
		t.Fatalf("unexpected error mutating npc inventory: %v", err)
	}

	if npc.Version != 1 {
		t.Fatalf("expected npc version to increment, got %d", npc.Version)
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

	expected := itemspkg.SimInventoryPayloadFromSlots[sim.InventorySlot, NPCInventoryPayload](
		itemspkg.SimInventorySlotsFromAny([]InventorySlot{{
			Slot: 0,
			Item: ItemStack{Type: ItemTypeGold, Quantity: 2},
		}}),
	)

	expectedSlots := requireInventorySlots(t, expected.Slots)
	slots := requireInventorySlots(t, payload.Slots)
	if len(slots) != len(expectedSlots) {
		t.Fatalf("expected payload to contain %d slot(s), got %d", len(expectedSlots), len(slots))
	}

	got := slots[0]
	want := expectedSlots[0]
	if got.Slot != want.Slot {
		t.Fatalf("expected slot index %d, got %d", want.Slot, got.Slot)
	}
	if got.Item.Type != want.Item.Type {
		t.Fatalf("expected slot item type %q, got %q", want.Item.Type, got.Item.Type)
	}
	if got.Item.Quantity != want.Item.Quantity {
		t.Fatalf("expected slot quantity %d, got %d", want.Item.Quantity, got.Item.Quantity)
	}
}

func TestSetEffectPositionRecordsPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	eff := &effectState{ID: "effect-1", X: 1, Y: 2}

	w.SetEffectPosition(eff, 5, 7)

	if eff.Version != 1 {
		t.Fatalf("expected effect version to increment, got %d", eff.Version)
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
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	eff := &effectState{ID: "effect-2"}

	w.SetEffectParam(eff, "remainingRange", 3.5)

	if eff.Version != 1 {
		t.Fatalf("expected effect version to increment, got %d", eff.Version)
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
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	item := &itemspkg.GroundItemState{GroundItem: itemspkg.GroundItem{ID: "ground-1", Qty: 1, X: 0, Y: 0}}

	setter := itemspkg.GroundItemQuantityJournalSetter(w.journal.AppendPatch)

	setter(item, 5)

	if item.Version != 1 {
		t.Fatalf("expected item version to increment, got %d", item.Version)
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
