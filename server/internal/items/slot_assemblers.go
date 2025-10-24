package items

// inventoryValue is satisfied by types whose underlying structure contains a
// `Slots` field backed by an inventory slot slice. The JSON tag matches the
// legacy inventory schema so callers can reuse these helpers for both
// production and test types without bespoke wrappers.
type inventoryValue[Slot any] interface {
	~struct {
		Slots []Slot `json:"slots"`
	}
}

// equipmentValue is satisfied by types whose underlying structure contains a
// `Slots` field backed by an equipped item slice. The JSON tag matches the
// legacy equipment schema so callers can rely on deterministic marshal behavior
// when reusing these helpers.
type equipmentValue[Slot any] interface {
	~struct {
		Slots []Slot `json:"slots,omitempty"`
	}
}

// legacyPayloadValue is satisfied by payload structs that expose a JSON `slots`
// field typed as `any`. This matches the journal payload schema which accepts
// pre-converted slot slices.
type legacyPayloadValue interface {
	~struct {
		Slots any `json:"slots"`
	}
}

// typedPayloadValue is satisfied by payload structs that expose a JSON `slots`
// field whose element type matches the provided slot slice.
type typedPayloadValue[Slot any] interface {
	~struct {
		Slots []Slot `json:"slots"`
	}
}

// InventoryValueFromSlots assembles an inventory-like struct from the provided
// slots slice. Callers supply the concrete inventory type via the type
// parameter so shared conversion helpers can construct legacy values without
// redefining thin wrappers at each call site.
func InventoryValueFromSlots[Slot any, Inventory inventoryValue[Slot]](slots []Slot) Inventory {
	return Inventory{Slots: slots}
}

// EquipmentValueFromSlots assembles an equipment-like struct from the provided
// slots slice. Callers supply the concrete equipment type via the type
// parameter so shared conversion helpers can construct legacy values without
// bespoke wrappers.
func EquipmentValueFromSlots[Slot any, Equipment equipmentValue[Slot]](slots []Slot) Equipment {
	return Equipment{Slots: slots}
}

// InventoryPayloadFromSlots assembles an inventory payload from the provided
// slots slice. The resulting value preserves the caller-provided slice so
// payload conversions can hand journal payloads a prebuilt slot representation
// without additional cloning.
func InventoryPayloadFromSlots[Slot any, Payload legacyPayloadValue](slots []Slot) Payload {
	return Payload{Slots: slots}
}

// EquipmentPayloadFromSlots assembles an equipment payload from the provided
// slots slice. The helper mirrors InventoryPayloadFromSlots so conversion code
// can share the central constructor while targeting the appropriate payload
// type.
func EquipmentPayloadFromSlots[Slot any, Payload legacyPayloadValue](slots []Slot) Payload {
	return Payload{Slots: slots}
}

// SimInventoryPayloadFromSlots assembles a simulation inventory payload from the
// provided slots slice. Typed payloads keep their slice element type rather than
// collapsing to `any`.
func SimInventoryPayloadFromSlots[Slot any, Payload typedPayloadValue[Slot]](slots []Slot) Payload {
	return Payload{Slots: slots}
}

// SimEquipmentPayloadFromSlots assembles a simulation equipment payload from the
// provided slots slice, preserving the typed slot representation.
func SimEquipmentPayloadFromSlots[Slot any, Payload typedPayloadValue[Slot]](slots []Slot) Payload {
	return Payload{Slots: slots}
}
