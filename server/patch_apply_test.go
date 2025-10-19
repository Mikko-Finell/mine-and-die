package main

import (
	"math"
	"testing"

	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

const floatEpsilon = 1e-6

func floatsEqual(a, b float64) bool {
	return math.Abs(a-b) <= floatEpsilon
}

func TestApplyPatchesReplaysLatestSnapshot(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	basePlayer := &playerState{actorState: actorState{Actor: Actor{
		ID:        "player-1",
		X:         10,
		Y:         20,
		Facing:    FacingUp,
		Health:    75,
		MaxHealth: 100,
		Inventory: Inventory{Slots: []InventorySlot{{
			Slot: 0,
			Item: ItemStack{Type: ItemTypeGold, Quantity: 3},
		}}},
	}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	basePlayer.intentX = 0.5
	basePlayer.intentY = -0.5
	w.AddPlayer(basePlayer)

	secondary := &playerState{actorState: actorState{Actor: Actor{
		ID:        "player-2",
		X:         0,
		Y:         0,
		Facing:    FacingDown,
		Health:    50,
		MaxHealth: 80,
	}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	secondary.intentX = 0
	secondary.intentY = 1
	w.AddPlayer(secondary)

	original := capturePlayerViews(w)

	w.SetPosition("player-1", 42, -7)
	w.SetFacing("player-1", FacingLeft)
	w.SetIntent("player-1", 1, 0)
	w.SetHealth("player-1", 40)
	if err := w.MutateInventory("player-1", func(inv *Inventory) error {
		inv.Slots = nil
		_, err := inv.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 2})
		return err
	}); err != nil {
		t.Fatalf("unexpected inventory mutation error: %v", err)
	}

	w.SetHealth("player-2", 30)
	w.SetIntent("player-2", 0, 0)

	patches := w.snapshotPatchesLocked()
	if len(patches) == 0 {
		t.Fatalf("expected patches to be recorded after mutations")
	}

	expected := capturePlayerViews(w)
	originalFrozen := capturePlayerViewsFromMap(original)

	replayed, err := ApplyPatches(original, patches)
	if err != nil {
		t.Fatalf("apply patches failed: %v", err)
	}

	if len(replayed) != len(expected) {
		t.Fatalf("expected %d players after replay, got %d", len(expected), len(replayed))
	}

	for id, want := range expected {
		got, ok := replayed[id]
		if !ok {
			t.Fatalf("expected player %s in replayed snapshot", id)
		}
		if !playerViewsEqual(got, want) {
			t.Fatalf("player %s view mismatch after replay", id)
		}
	}

	for id, originalView := range original {
		frozen := originalFrozen[id]
		if !playerViewsEqual(originalView, frozen) {
			t.Fatalf("original snapshot mutated for player %s", id)
		}
	}
}

func capturePlayerViews(w *World) map[string]PlayerPatchView {
	views := make(map[string]PlayerPatchView, len(w.players))
	for id, state := range w.players {
		views[id] = PlayerPatchView{
			Player:   state.snapshot(),
			IntentDX: state.intentX,
			IntentDY: state.intentY,
		}
	}
	return views
}

func capturePlayerViewsFromMap(src map[string]PlayerPatchView) map[string]PlayerPatchView {
	views := make(map[string]PlayerPatchView, len(src))
	for id, view := range src {
		views[id] = clonePlayerPatchView(view)
	}
	return views
}

func playerViewsEqual(a, b PlayerPatchView) bool {
	if a.ID != b.ID {
		return false
	}
	if !floatsEqual(a.X, b.X) || !floatsEqual(a.Y, b.Y) {
		return false
	}
	if a.Facing != b.Facing {
		return false
	}
	if !floatsEqual(a.IntentDX, b.IntentDX) || !floatsEqual(a.IntentDY, b.IntentDY) {
		return false
	}
	if !floatsEqual(a.Health, b.Health) || !floatsEqual(a.MaxHealth, b.MaxHealth) {
		return false
	}
	if !inventoriesSlotEqual(a.Inventory, b.Inventory) {
		return false
	}
	return true
}

func playerViewMapsEqual(a, b map[string]PlayerPatchView) bool {
	if len(a) != len(b) {
		return false
	}
	for id, viewA := range a {
		viewB, ok := b[id]
		if !ok {
			return false
		}
		if !playerViewsEqual(viewA, viewB) {
			return false
		}
	}
	return true
}

func inventoriesSlotEqual(a, b Inventory) bool {
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

func TestApplyPatchesNoop(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	player := &playerState{actorState: actorState{Actor: Actor{
		ID:        "player-1",
		X:         5,
		Y:         -3,
		Facing:    FacingRight,
		Health:    90,
		MaxHealth: 100,
		Inventory: Inventory{Slots: []InventorySlot{{
			Slot: 0,
			Item: ItemStack{Type: ItemTypeGold, Quantity: 1},
		}}},
	}}, stats: stats.DefaultComponent(stats.ArchetypePlayer)}
	player.intentX = 1
	player.intentY = 0
	w.AddPlayer(player)

	original := capturePlayerViews(w)

	replayed, err := ApplyPatches(original, nil)
	if err != nil {
		t.Fatalf("apply patches failed: %v", err)
	}

	if !playerViewMapsEqual(original, replayed) {
		t.Fatalf("expected replayed snapshot to equal original when applying nil patches")
	}

	// Ensure the original snapshot remains untouched after replay.
	frozen := capturePlayerViewsFromMap(original)
	if !playerViewMapsEqual(original, frozen) {
		t.Fatalf("original snapshot mutated during noop replay")
	}
}

func TestApplyPatchesRemovesPlayer(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	player := newTestPlayerState("player-remove")
	w.AddPlayer(player)

	base := capturePlayerViews(w)
	if _, ok := base[player.ID]; !ok {
		t.Fatalf("expected base snapshot to include %s", player.ID)
	}

	patches := []Patch{{Kind: PatchPlayerRemoved, EntityID: player.ID}}

	replayed, err := ApplyPatches(base, patches)
	if err != nil {
		t.Fatalf("apply patches failed: %v", err)
	}

	if _, ok := replayed[player.ID]; ok {
		t.Fatalf("expected player %s to be removed from replayed snapshot", player.ID)
	}

	if _, stillPresent := base[player.ID]; !stillPresent {
		t.Fatalf("expected original snapshot to retain player %s", player.ID)
	}
}

func TestApplyPatchesUnknownEntity(t *testing.T) {
	base := map[string]PlayerPatchView{
		"player-1": {
			Player: Player{
				Actor: Actor{
					ID:        "player-1",
					X:         1,
					Y:         2,
					Facing:    FacingUp,
					Health:    50,
					MaxHealth: 75,
				},
			},
			IntentDX: 0,
			IntentDY: 1,
		},
	}

	patches := []Patch{{
		Kind:     PatchPlayerPos,
		EntityID: "ghost",
		Payload:  PlayerPosPayload{X: 3, Y: 4},
	}}

	replayed, err := ApplyPatches(base, patches)
	if err == nil {
		t.Fatalf("expected error when applying patch for unknown entity")
	}

	if replayed != nil {
		t.Fatalf("expected no snapshot on error, got %#v", replayed)
	}

	if !playerViewMapsEqual(base, capturePlayerViewsFromMap(base)) {
		t.Fatalf("base snapshot mutated when applying unknown entity patch")
	}
}

func TestApplyPatchesDuplicatePatchesLastWriteWins(t *testing.T) {
	base := map[string]PlayerPatchView{
		"player-1": {
			Player: Player{
				Actor: Actor{
					ID:        "player-1",
					X:         0,
					Y:         0,
					Facing:    FacingDown,
					Health:    10,
					MaxHealth: 20,
					Inventory: Inventory{Slots: []InventorySlot{{
						Slot: 0,
						Item: ItemStack{Type: ItemTypeGold, Quantity: 1},
					}}}},
			},
			IntentDX: 0,
			IntentDY: 0,
		},
	}

	patches := []Patch{
		{
			Kind:     PatchPlayerPos,
			EntityID: "player-1",
			Payload:  PlayerPosPayload{X: 1, Y: 1},
		},
		{
			Kind:     PatchPlayerPos,
			EntityID: "player-1",
			Payload:  PlayerPosPayload{X: 5, Y: -2},
		},
		{
			Kind:     PatchPlayerIntent,
			EntityID: "player-1",
			Payload:  PlayerIntentPayload{DX: 1, DY: 0},
		},
		{
			Kind:     PatchPlayerIntent,
			EntityID: "player-1",
			Payload:  PlayerIntentPayload{DX: -1, DY: 1},
		},
		{
			Kind:     PatchPlayerHealth,
			EntityID: "player-1",
			Payload:  PlayerHealthPayload{Health: 15, MaxHealth: 30},
		},
		{
			Kind:     PatchPlayerHealth,
			EntityID: "player-1",
			Payload:  PlayerHealthPayload{Health: 8},
		},
		{
			Kind:     PatchPlayerInventory,
			EntityID: "player-1",
			Payload: PlayerInventoryPayload{
				Slots: []InventorySlot{
					{
						Slot: 0,
						Item: ItemStack{Type: ItemTypeGold, Quantity: 2},
					},
				},
			},
		},
		{
			Kind:     PatchPlayerInventory,
			EntityID: "player-1",
			Payload: PlayerInventoryPayload{
				Slots: []InventorySlot{
					{
						Slot: 0,
						Item: ItemStack{Type: ItemTypeHealthPotion, Quantity: 1},
					},
					{
						Slot: 1,
						Item: ItemStack{Type: ItemTypeGold, Quantity: 4},
					},
				},
			},
		},
		{
			Kind:     PatchPlayerFacing,
			EntityID: "player-1",
			Payload:  PlayerFacingPayload{Facing: FacingLeft},
		},
		{
			Kind:     PatchPlayerFacing,
			EntityID: "player-1",
			Payload:  PlayerFacingPayload{Facing: FacingUp},
		},
	}

	replayed, err := ApplyPatches(base, patches)
	if err != nil {
		t.Fatalf("apply patches failed: %v", err)
	}

	expected := PlayerPatchView{
		Player: Player{
			Actor: Actor{
				ID:        "player-1",
				X:         5,
				Y:         -2,
				Facing:    FacingUp,
				Health:    8,
				MaxHealth: 30,
				Inventory: Inventory{Slots: []InventorySlot{
					{
						Slot: 0,
						Item: ItemStack{Type: ItemTypeHealthPotion, Quantity: 1},
					},
					{
						Slot: 1,
						Item: ItemStack{Type: ItemTypeGold, Quantity: 4},
					},
				}},
			},
		},
		IntentDX: -1,
		IntentDY: 1,
	}

	replayedView, ok := replayed["player-1"]
	if !ok {
		t.Fatalf("expected player-1 in replayed snapshot")
	}

	if !playerViewsEqual(replayedView, expected) {
		t.Fatalf("expected last write wins for duplicate patches")
	}

	if !playerViewMapsEqual(base, capturePlayerViewsFromMap(base)) {
		t.Fatalf("base snapshot mutated during duplicate patch replay")
	}
}

func TestApplyPatchesUpdatesEquipment(t *testing.T) {
	base := map[string]PlayerPatchView{
		"player-1": {
			Player: Player{Actor: Actor{ID: "player-1", Equipment: Equipment{Slots: []EquippedItem{{
				Slot: EquipSlotBody,
				Item: ItemStack{Type: ItemTypeLeatherJerkin, Quantity: 1},
			}}}}},
		},
	}

	patches := []Patch{{
		Kind:     PatchPlayerEquipment,
		EntityID: "player-1",
		Payload: EquipmentPayload{Slots: []EquippedItem{
			{
				Slot: EquipSlotBody,
				Item: ItemStack{Type: ItemTypeLeatherJerkin, Quantity: 1},
			},
			{
				Slot: EquipSlotMainHand,
				Item: ItemStack{Type: ItemTypeIronDagger, Quantity: 1},
			},
		}},
	}}

	frozen := capturePlayerViewsFromMap(base)
	replayed, err := ApplyPatches(base, patches)
	if err != nil {
		t.Fatalf("apply patches failed: %v", err)
	}

	replayedView, ok := replayed["player-1"]
	if !ok {
		t.Fatalf("expected player-1 in replayed snapshot")
	}

	expected := Equipment{Slots: []EquippedItem{
		{
			Slot: EquipSlotBody,
			Item: ItemStack{Type: ItemTypeLeatherJerkin, Quantity: 1},
		},
		{
			Slot: EquipSlotMainHand,
			Item: ItemStack{Type: ItemTypeIronDagger, Quantity: 1},
		},
	}}

	if !equipmentsEqual(expected, replayedView.Equipment) {
		t.Fatalf("expected equipment update to replay, got %+v", replayedView.Equipment)
	}

	if !playerViewMapsEqual(base, frozen) {
		t.Fatalf("base snapshot mutated during equipment patch replay")
	}
}
