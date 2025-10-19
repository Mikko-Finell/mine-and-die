package patches

import (
	"testing"

	"mine-and-die/server/internal/sim"
)

func TestApplyPlayersReplaysLatestSnapshot(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {
			Player: sim.Player{Actor: sim.Actor{
				ID:        "player-1",
				X:         10,
				Y:         20,
				Facing:    sim.FacingUp,
				Health:    75,
				MaxHealth: 100,
				Inventory: sim.Inventory{Slots: []sim.InventorySlot{{
					Slot: 0,
					Item: sim.ItemStack{Type: "gold", Quantity: 3},
				}}},
			}},
			IntentDX: 0.5,
			IntentDY: -0.5,
		},
		"player-2": {
			Player: sim.Player{Actor: sim.Actor{
				ID:        "player-2",
				X:         0,
				Y:         0,
				Facing:    sim.FacingDown,
				Health:    50,
				MaxHealth: 80,
			}},
			IntentDX: 0,
			IntentDY: 1,
		},
	}

	patches := []sim.Patch{
		{
			Kind:     sim.PatchPlayerPos,
			EntityID: "player-1",
			Payload:  sim.PlayerPosPayload{X: 42, Y: -7},
		},
		{
			Kind:     sim.PatchPlayerFacing,
			EntityID: "player-1",
			Payload:  sim.PlayerFacingPayload{Facing: sim.FacingLeft},
		},
		{
			Kind:     sim.PatchPlayerIntent,
			EntityID: "player-1",
			Payload:  sim.PlayerIntentPayload{DX: 1, DY: 0},
		},
		{
			Kind:     sim.PatchPlayerHealth,
			EntityID: "player-1",
			Payload:  sim.PlayerHealthPayload{Health: 40},
		},
		{
			Kind:     sim.PatchPlayerInventory,
			EntityID: "player-1",
			Payload: sim.PlayerInventoryPayload{Slots: []sim.InventorySlot{{
				Slot: 1,
				Item: sim.ItemStack{Type: "potion", Quantity: 2},
			}}},
		},
		{
			Kind:     sim.PatchPlayerHealth,
			EntityID: "player-2",
			Payload:  sim.PlayerHealthPayload{Health: 30},
		},
		{
			Kind:     sim.PatchPlayerIntent,
			EntityID: "player-2",
			Payload:  sim.PlayerIntentPayload{DX: 0, DY: 0},
		},
	}

	expected := map[string]PlayerView{
		"player-1": {
			Player: sim.Player{Actor: sim.Actor{
				ID:        "player-1",
				X:         42,
				Y:         -7,
				Facing:    sim.FacingLeft,
				Health:    40,
				MaxHealth: 100,
				Inventory: sim.Inventory{Slots: []sim.InventorySlot{{
					Slot: 1,
					Item: sim.ItemStack{Type: "potion", Quantity: 2},
				}}},
			}},
			IntentDX: 1,
			IntentDY: 0,
		},
		"player-2": {
			Player: sim.Player{Actor: sim.Actor{
				ID:        "player-2",
				X:         0,
				Y:         0,
				Facing:    sim.FacingDown,
				Health:    30,
				MaxHealth: 80,
			}},
			IntentDX: 0,
			IntentDY: 0,
		},
	}

	original := clonePlayerViewMap(base)

	replayed, err := ApplyPlayers(base, patches)
	if err != nil {
		t.Fatalf("apply players failed: %v", err)
	}

	if !playerViewMapsEqual(replayed, expected) {
		t.Fatalf("replayed snapshot mismatch\nwant: %+v\n got: %+v", expected, replayed)
	}

	if !playerViewMapsEqual(base, original) {
		t.Fatalf("base snapshot mutated during replay")
	}
}

func TestApplyPlayersNoop(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {
			Player: sim.Player{Actor: sim.Actor{
				ID:        "player-1",
				X:         5,
				Y:         -3,
				Facing:    sim.FacingRight,
				Health:    90,
				MaxHealth: 100,
				Inventory: sim.Inventory{Slots: []sim.InventorySlot{{
					Slot: 0,
					Item: sim.ItemStack{Type: "gold", Quantity: 1},
				}}},
			}},
			IntentDX: 1,
			IntentDY: 0,
		},
	}

	original := clonePlayerViewMap(base)

	replayed, err := ApplyPlayers(base, nil)
	if err != nil {
		t.Fatalf("apply players failed: %v", err)
	}

	if !playerViewMapsEqual(replayed, base) {
		t.Fatalf("expected replayed snapshot to equal base when applying nil patches")
	}

	if !playerViewMapsEqual(base, original) {
		t.Fatalf("base snapshot mutated during noop replay")
	}
}

func TestApplyPlayersRemovesPlayer(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {Player: sim.Player{Actor: sim.Actor{ID: "player-1"}}},
		"player-2": {Player: sim.Player{Actor: sim.Actor{ID: "player-2"}}},
	}

	patches := []sim.Patch{{Kind: sim.PatchPlayerRemoved, EntityID: "player-2"}}

	replayed, err := ApplyPlayers(base, patches)
	if err != nil {
		t.Fatalf("apply players failed: %v", err)
	}

	if len(replayed) != 1 {
		t.Fatalf("expected 1 player after removal, got %d", len(replayed))
	}

	if _, ok := replayed["player-1"]; !ok {
		t.Fatalf("expected player-1 to remain after removal")
	}
	if _, ok := replayed["player-2"]; ok {
		t.Fatalf("expected player-2 to be removed")
	}
}

func TestApplyPlayersUnknownEntity(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {Player: sim.Player{Actor: sim.Actor{ID: "player-1"}}},
	}
	original := clonePlayerViewMap(base)

	patches := []sim.Patch{{
		Kind:     sim.PatchPlayerHealth,
		EntityID: "player-2",
		Payload:  sim.PlayerHealthPayload{Health: 10},
	}}

	_, err := ApplyPlayers(base, patches)
	if err == nil {
		t.Fatalf("expected error when applying patch for unknown entity")
	}

	if !playerViewMapsEqual(base, original) {
		t.Fatalf("base snapshot mutated when applying unknown entity patch")
	}
}

func TestApplyPlayersDuplicatePatchesLastWriteWins(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {
			Player: sim.Player{Actor: sim.Actor{
				ID:        "player-1",
				X:         0,
				Y:         0,
				Facing:    sim.FacingUp,
				Health:    10,
				MaxHealth: 10,
			}},
		},
	}

	patches := []sim.Patch{
		{Kind: sim.PatchPlayerPos, EntityID: "player-1", Payload: sim.PlayerPosPayload{X: 1, Y: 2}},
		{Kind: sim.PatchPlayerPos, EntityID: "player-1", Payload: sim.PlayerPosPayload{X: 5, Y: 6}},
		{Kind: sim.PatchPlayerFacing, EntityID: "player-1", Payload: sim.PlayerFacingPayload{Facing: sim.FacingLeft}},
		{Kind: sim.PatchPlayerFacing, EntityID: "player-1", Payload: sim.PlayerFacingPayload{Facing: sim.FacingDown}},
		{Kind: sim.PatchPlayerHealth, EntityID: "player-1", Payload: sim.PlayerHealthPayload{Health: 7}},
		{Kind: sim.PatchPlayerHealth, EntityID: "player-1", Payload: sim.PlayerHealthPayload{Health: 9}},
		{Kind: sim.PatchPlayerIntent, EntityID: "player-1", Payload: sim.PlayerIntentPayload{DX: 1, DY: 0}},
		{Kind: sim.PatchPlayerIntent, EntityID: "player-1", Payload: sim.PlayerIntentPayload{DX: -1, DY: 1}},
	}

	replayed, err := ApplyPlayers(base, patches)
	if err != nil {
		t.Fatalf("apply players failed: %v", err)
	}

	view := replayed["player-1"]
	if view.Player.X != 5 || view.Player.Y != 6 {
		t.Fatalf("expected last position patch to win, got (%f,%f)", view.Player.X, view.Player.Y)
	}
	if view.Player.Facing != sim.FacingDown {
		t.Fatalf("expected last facing patch to win, got %v", view.Player.Facing)
	}
	if view.Player.Health != 9 {
		t.Fatalf("expected last health patch to win, got %f", view.Player.Health)
	}
	if view.IntentDX != -1 || view.IntentDY != 1 {
		t.Fatalf("expected last intent patch to win, got (%f,%f)", view.IntentDX, view.IntentDY)
	}
}

func clonePlayerViewMap(src map[string]PlayerView) map[string]PlayerView {
	if src == nil {
		return nil
	}
	cloned := make(map[string]PlayerView, len(src))
	for id, view := range src {
		cloned[id] = view.Clone()
	}
	return cloned
}

func playerViewMapsEqual(a, b map[string]PlayerView) bool {
	if len(a) != len(b) {
		return false
	}
	for id, va := range a {
		vb, ok := b[id]
		if !ok {
			return false
		}
		if !playerViewsEqual(va, vb) {
			return false
		}
	}
	return true
}

func playerViewsEqual(a, b PlayerView) bool {
	if a.Player.ID != b.Player.ID {
		return false
	}
	if a.Player.X != b.Player.X || a.Player.Y != b.Player.Y {
		return false
	}
	if a.Player.Facing != b.Player.Facing {
		return false
	}
	if a.Player.Health != b.Player.Health || a.Player.MaxHealth != b.Player.MaxHealth {
		return false
	}
	if a.IntentDX != b.IntentDX || a.IntentDY != b.IntentDY {
		return false
	}
	if !inventoriesEqual(a.Player.Inventory, b.Player.Inventory) {
		return false
	}
	if !equipmentEqual(a.Player.Equipment, b.Player.Equipment) {
		return false
	}
	return true
}

func inventoriesEqual(a, b sim.Inventory) bool {
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

func equipmentEqual(a, b sim.Equipment) bool {
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

func TestApplyPlayersUpdatesEquipment(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {
			Player: sim.Player{Actor: sim.Actor{ID: "player-1", Equipment: sim.Equipment{Slots: []sim.EquippedItem{{
				Slot: sim.EquipSlotMainHand,
				Item: sim.ItemStack{Type: "sword", Quantity: 1},
			}}}}},
		},
	}
	original := clonePlayerViewMap(base)

	patches := []sim.Patch{{
		Kind:     sim.PatchPlayerEquipment,
		EntityID: "player-1",
		Payload: sim.PlayerEquipmentPayload{Slots: []sim.EquippedItem{{
			Slot: sim.EquipSlotMainHand,
			Item: sim.ItemStack{Type: "bow", Quantity: 1},
		}}},
	}}

	replayed, err := ApplyPlayers(base, patches)
	if err != nil {
		t.Fatalf("apply players failed: %v", err)
	}

	view := replayed["player-1"]
	if len(view.Player.Equipment.Slots) != 1 {
		t.Fatalf("expected one equipment slot after replay")
	}
	if view.Player.Equipment.Slots[0].Item.Type != "bow" {
		t.Fatalf("expected equipment to be replaced, got %q", view.Player.Equipment.Slots[0].Item.Type)
	}

	if !playerViewMapsEqual(base, original) {
		t.Fatalf("base snapshot mutated when applying equipment patch")
	}
}

func TestApplyPlayersMissingEntityID(t *testing.T) {
	_, err := ApplyPlayers(nil, []sim.Patch{{Kind: sim.PatchPlayerPos}})
	if err == nil {
		t.Fatalf("expected error when entity id is missing")
	}
}

func TestApplyPlayersUnsupportedPatch(t *testing.T) {
	base := map[string]PlayerView{
		"player-1": {Player: sim.Player{Actor: sim.Actor{ID: "player-1"}}},
	}

	_, err := ApplyPlayers(base, []sim.Patch{{
		Kind:     sim.PatchNPCPos,
		EntityID: "player-1",
		Payload:  sim.NPCPosPayload{X: 1, Y: 2},
	}})
	if err == nil {
		t.Fatalf("expected error for unsupported patch kind")
	}
}
