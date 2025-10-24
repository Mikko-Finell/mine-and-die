package items

import "testing"

type (
	assemblerInventorySlot struct {
		Slot int `json:"slot"`
	}

	assemblerInventory struct {
		Slots []assemblerInventorySlot `json:"slots"`
	}

	assemblerEquipmentSlot struct {
		Slot string `json:"slot"`
	}

	assemblerEquipment struct {
		Slots []assemblerEquipmentSlot `json:"slots,omitempty"`
	}

	assemblerPayload struct {
		Slots any `json:"slots"`
	}

	typedInventoryPayload struct {
		Slots []assemblerInventorySlot `json:"slots"`
	}

	typedEquipmentPayload struct {
		Slots []assemblerEquipmentSlot `json:"slots"`
	}
)

func TestInventoryValueFromSlots(t *testing.T) {
	slots := []assemblerInventorySlot{{Slot: 3}}

	inv := InventoryValueFromSlots[assemblerInventorySlot, assemblerInventory](slots)

	if len(inv.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(inv.Slots))
	}
	if inv.Slots[0].Slot != 3 {
		t.Fatalf("unexpected slot payload: %+v", inv.Slots[0])
	}
	if &inv.Slots[0] != &slots[0] {
		t.Fatalf("expected slots slice to be reused")
	}
}

func TestInventoryValueFromSlotsEmpty(t *testing.T) {
	inv := InventoryValueFromSlots[assemblerInventorySlot, assemblerInventory](nil)

	if inv.Slots != nil {
		t.Fatalf("expected nil slots, got %+v", inv.Slots)
	}
}

func TestEquipmentValueFromSlots(t *testing.T) {
	slots := []assemblerEquipmentSlot{{Slot: "Head"}}

	eq := EquipmentValueFromSlots[assemblerEquipmentSlot, assemblerEquipment](slots)

	if len(eq.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(eq.Slots))
	}
	if eq.Slots[0].Slot != "Head" {
		t.Fatalf("unexpected slot payload: %+v", eq.Slots[0])
	}
	if &eq.Slots[0] != &slots[0] {
		t.Fatalf("expected slots slice to be reused")
	}
}

func TestEquipmentValueFromSlotsEmpty(t *testing.T) {
	eq := EquipmentValueFromSlots[assemblerEquipmentSlot, assemblerEquipment](nil)

	if eq.Slots != nil {
		t.Fatalf("expected nil slots, got %+v", eq.Slots)
	}
}

func TestInventoryPayloadFromSlots(t *testing.T) {
	slots := []assemblerInventorySlot{{Slot: 9}}

	payload := InventoryPayloadFromSlots[assemblerInventorySlot, assemblerPayload](slots)

	converted, ok := payload.Slots.([]assemblerInventorySlot)
	if !ok {
		t.Fatalf("expected slots slice, got %T", payload.Slots)
	}
	if len(converted) != 1 || converted[0].Slot != 9 {
		t.Fatalf("unexpected payload slots: %+v", converted)
	}
	if &converted[0] != &slots[0] {
		t.Fatalf("expected slots slice to be reused")
	}
}

func TestEquipmentPayloadFromSlots(t *testing.T) {
	slots := []assemblerEquipmentSlot{{Slot: "OffHand"}}

	payload := EquipmentPayloadFromSlots[assemblerEquipmentSlot, assemblerPayload](slots)

	converted, ok := payload.Slots.([]assemblerEquipmentSlot)
	if !ok {
		t.Fatalf("expected slots slice, got %T", payload.Slots)
	}
	if len(converted) != 1 || converted[0].Slot != "OffHand" {
		t.Fatalf("unexpected payload slots: %+v", converted)
	}
	if &converted[0] != &slots[0] {
		t.Fatalf("expected slots slice to be reused")
	}
}

func TestSimInventoryPayloadFromSlots(t *testing.T) {
	slots := []assemblerInventorySlot{{Slot: 11}}

	payload := SimInventoryPayloadFromSlots[assemblerInventorySlot, typedInventoryPayload](slots)

	if len(payload.Slots) != 1 || payload.Slots[0].Slot != 11 {
		t.Fatalf("unexpected typed inventory slots: %+v", payload.Slots)
	}
	if &payload.Slots[0] != &slots[0] {
		t.Fatalf("expected slots slice to be reused")
	}
}

func TestSimEquipmentPayloadFromSlots(t *testing.T) {
	slots := []assemblerEquipmentSlot{{Slot: "Accessory"}}

	payload := SimEquipmentPayloadFromSlots[assemblerEquipmentSlot, typedEquipmentPayload](slots)

	if len(payload.Slots) != 1 || payload.Slots[0].Slot != "Accessory" {
		t.Fatalf("unexpected typed equipment slots: %+v", payload.Slots)
	}
	if &payload.Slots[0] != &slots[0] {
		t.Fatalf("expected slots slice to be reused")
	}
}
