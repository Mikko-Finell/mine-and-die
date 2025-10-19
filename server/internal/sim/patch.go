package sim

// PatchKind identifies the type of diff entry.
type PatchKind string

const (
	PatchPlayerPos       PatchKind = "player_pos"
	PatchPlayerFacing    PatchKind = "player_facing"
	PatchPlayerIntent    PatchKind = "player_intent"
	PatchPlayerHealth    PatchKind = "player_health"
	PatchPlayerInventory PatchKind = "player_inventory"
	PatchPlayerEquipment PatchKind = "player_equipment"
	PatchPlayerRemoved   PatchKind = "player_removed"

	PatchNPCPos       PatchKind = "npc_pos"
	PatchNPCFacing    PatchKind = "npc_facing"
	PatchNPCHealth    PatchKind = "npc_health"
	PatchNPCInventory PatchKind = "npc_inventory"
	PatchNPCEquipment PatchKind = "npc_equipment"

	PatchEffectPos    PatchKind = "effect_pos"
	PatchEffectParams PatchKind = "effect_params"

	PatchGroundItemPos PatchKind = "ground_item_pos"
	PatchGroundItemQty PatchKind = "ground_item_qty"
)

// Patch represents a diff entry that can be applied to the client state.
type Patch struct {
	Kind     PatchKind `json:"kind"`
	EntityID string    `json:"entityId"`
	Payload  any       `json:"payload,omitempty"`
}

// PositionPayload captures the coordinates for an entity position patch.
type PositionPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PlayerPosPayload captures the coordinates for a player position patch.
type PlayerPosPayload = PositionPayload

// NPCPosPayload captures the coordinates for an NPC position patch.
type NPCPosPayload = PositionPayload

// EffectPosPayload captures the coordinates for an effect position patch.
type EffectPosPayload = PositionPayload

// GroundItemPosPayload captures the coordinates for a ground item patch.
type GroundItemPosPayload = PositionPayload

// FacingPayload captures the facing for an entity patch.
type FacingPayload struct {
	Facing FacingDirection `json:"facing"`
}

// PlayerFacingPayload captures the facing for a player patch.
type PlayerFacingPayload = FacingPayload

// NPCFacingPayload captures the facing for an NPC patch.
type NPCFacingPayload = FacingPayload

// PlayerIntentPayload captures the movement intent vector for a player patch.
type PlayerIntentPayload struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
}

// HealthPayload captures the health for an entity patch.
type HealthPayload struct {
	Health    float64 `json:"health"`
	MaxHealth float64 `json:"maxHealth,omitempty"`
}

// PlayerHealthPayload captures the health for a player patch.
type PlayerHealthPayload = HealthPayload

// NPCHealthPayload captures the health for an NPC patch.
type NPCHealthPayload = HealthPayload

// InventoryPayload captures the inventory slots for an entity patch.
type InventoryPayload struct {
	Slots []InventorySlot `json:"slots"`
}

// PlayerInventoryPayload captures the inventory slots for a player patch.
type PlayerInventoryPayload = InventoryPayload

// NPCInventoryPayload captures the inventory slots for an NPC patch.
type NPCInventoryPayload = InventoryPayload

// EquipmentPayload captures the equipped items for an entity patch.
type EquipmentPayload struct {
	Slots []EquippedItem `json:"slots"`
}

// PlayerEquipmentPayload captures the equipped items for a player patch.
type PlayerEquipmentPayload = EquipmentPayload

// NPCEquipmentPayload captures the equipped items for an NPC patch.
type NPCEquipmentPayload = EquipmentPayload

// EffectParamsPayload captures the mutable parameters for an effect patch.
type EffectParamsPayload struct {
	Params map[string]float64 `json:"params"`
}

// GroundItemQtyPayload captures the quantity for a ground item patch.
type GroundItemQtyPayload struct {
	Qty int `json:"qty"`
}
