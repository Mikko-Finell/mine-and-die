package state

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const defaultRecycleValue = 0.75

// ItemType represents a unique identifier for an item kind.
type ItemType string

// ItemClass enumerates the canonical classes used across gameplay systems.
type ItemClass string

const (
	ItemClassWeapon            ItemClass = "weapon"
	ItemClassShield            ItemClass = "shield"
	ItemClassArmor             ItemClass = "armor"
	ItemClassAccessory         ItemClass = "accessory"
	ItemClassCoating           ItemClass = "coating"
	ItemClassConsumable        ItemClass = "consumable"
	ItemClassThrowable         ItemClass = "throwable"
	ItemClassTrap              ItemClass = "trap"
	ItemClassBlock             ItemClass = "block"
	ItemClassContainer         ItemClass = "container"
	ItemClassTool              ItemClass = "tool"
	ItemClassProcessedMaterial ItemClass = "processed_material"
)

var validItemClasses = map[ItemClass]struct{}{
	ItemClassWeapon:            {},
	ItemClassShield:            {},
	ItemClassArmor:             {},
	ItemClassAccessory:         {},
	ItemClassCoating:           {},
	ItemClassConsumable:        {},
	ItemClassThrowable:         {},
	ItemClassTrap:              {},
	ItemClassBlock:             {},
	ItemClassContainer:         {},
	ItemClassTool:              {},
	ItemClassProcessedMaterial: {},
}

// EquipSlot represents the seven equipable slots defined in the itemization plan.
type EquipSlot string

const (
	EquipSlotMainHand  EquipSlot = "MainHand"
	EquipSlotOffHand   EquipSlot = "OffHand"
	EquipSlotHead      EquipSlot = "Head"
	EquipSlotBody      EquipSlot = "Body"
	EquipSlotGloves    EquipSlot = "Gloves"
	EquipSlotBoots     EquipSlot = "Boots"
	EquipSlotAccessory EquipSlot = "Accessory"
)

var validEquipSlots = map[EquipSlot]struct{}{
	EquipSlotMainHand:  {},
	EquipSlotOffHand:   {},
	EquipSlotHead:      {},
	EquipSlotBody:      {},
	EquipSlotGloves:    {},
	EquipSlotBoots:     {},
	EquipSlotAccessory: {},
}

var equipSlotsRequiredForClass = map[ItemClass]bool{
	ItemClassWeapon:    true,
	ItemClassShield:    true,
	ItemClassArmor:     true,
	ItemClassAccessory: true,
	ItemClassTool:      true,
}

// ItemAction enumerates deterministic verbs introduced by an item.
type ItemAction string

const (
	ItemActionAttack   ItemAction = "attack"
	ItemActionSpecial  ItemAction = "special"
	ItemActionCharge   ItemAction = "charge"
	ItemActionBlock    ItemAction = "block"
	ItemActionBash     ItemAction = "bash"
	ItemActionActivate ItemAction = "activate"
	ItemActionConsume  ItemAction = "consume"
	ItemActionApply    ItemAction = "apply"
	ItemActionThrow    ItemAction = "throw"
	ItemActionPlace    ItemAction = "place"
	ItemActionMine     ItemAction = "mine"
	ItemActionHarvest  ItemAction = "harvest"
	ItemActionOpen     ItemAction = "open"
	ItemActionLock     ItemAction = "lock"
	ItemActionUnlock   ItemAction = "unlock"
	ItemActionPickup   ItemAction = "pickup"
	ItemActionRemove   ItemAction = "remove"
	ItemActionRepair   ItemAction = "repair"
)

var validItemActions = map[ItemAction]struct{}{
	ItemActionAttack:   {},
	ItemActionSpecial:  {},
	ItemActionCharge:   {},
	ItemActionBlock:    {},
	ItemActionBash:     {},
	ItemActionActivate: {},
	ItemActionConsume:  {},
	ItemActionApply:    {},
	ItemActionThrow:    {},
	ItemActionPlace:    {},
	ItemActionMine:     {},
	ItemActionHarvest:  {},
	ItemActionOpen:     {},
	ItemActionLock:     {},
	ItemActionUnlock:   {},
	ItemActionPickup:   {},
	ItemActionRemove:   {},
	ItemActionRepair:   {},
}

// ItemModifier defines a deterministic payload applied when an item is equipped or consumed.
type ItemModifier struct {
	Type            string  `json:"type"`
	Magnitude       float64 `json:"magnitude"`
	DurationSeconds int     `json:"duration_seconds"`
}

// ItemDefinition describes metadata for an item kind that can appear in the world. The fields mirror the taxonomy in
// docs/gameplay-design/itemization-and-equipment.md so downstream systems share a deterministic schema.
type ItemDefinition struct {
	ID             ItemType       `json:"id"`
	Class          ItemClass      `json:"class"`
	Tier           int            `json:"tier"`
	Stackable      bool           `json:"stackable"`
	FungibilityKey string         `json:"fungibility_key"`
	EquipSlot      EquipSlot      `json:"equip_slot,omitempty"`
	Actions        []ItemAction   `json:"actions"`
	Modifiers      []ItemModifier `json:"modifiers"`
	RecycleValue   float64        `json:"recycle_value"`
	// Deprecated: these fields support the current placeholder UI only. Inventories/renderers should migrate to schema-driven
	// presentation once the new UI lands.
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ItemDefinitionParams describes the configurable fields used when constructing an ItemDefinition.
type ItemDefinitionParams struct {
	ID           ItemType
	Class        ItemClass
	Tier         int
	Stackable    bool
	EquipSlot    EquipSlot
	Actions      []ItemAction
	Modifiers    []ItemModifier
	RecycleValue float64
	QualityTags  []string
	Name         string
	Description  string
}

// NewItemDefinition validates and constructs a canonical ItemDefinition.
func NewItemDefinition(params ItemDefinitionParams) (ItemDefinition, error) {
	if params.ID == "" {
		return ItemDefinition{}, fmt.Errorf("item id must be provided")
	}
	if _, ok := validItemClasses[params.Class]; !ok {
		return ItemDefinition{}, fmt.Errorf("invalid item class %q", params.Class)
	}

	equipSlot := params.EquipSlot
	if equipSlotsRequiredForClass[params.Class] {
		if equipSlot == "" {
			return ItemDefinition{}, fmt.Errorf("item class %s requires equip slot", params.Class)
		}
	}
	if equipSlot != "" {
		if _, ok := validEquipSlots[equipSlot]; !ok {
			return ItemDefinition{}, fmt.Errorf("invalid equip slot %q", equipSlot)
		}
	}

	actionSet := make([]ItemAction, 0, len(params.Actions))
	seenActions := make(map[ItemAction]struct{}, len(params.Actions))
	for _, action := range params.Actions {
		if _, ok := validItemActions[action]; !ok {
			return ItemDefinition{}, fmt.Errorf("invalid item action %q", action)
		}
		if _, seen := seenActions[action]; seen {
			continue
		}
		seenActions[action] = struct{}{}
		actionSet = append(actionSet, action)
	}
	sort.Slice(actionSet, func(i, j int) bool { return actionSet[i] < actionSet[j] })

	modifiers := make([]ItemModifier, len(params.Modifiers))
	copy(modifiers, params.Modifiers)
	sort.Slice(modifiers, func(i, j int) bool {
		if modifiers[i].Type == modifiers[j].Type {
			if modifiers[i].Magnitude == modifiers[j].Magnitude {
				return modifiers[i].DurationSeconds < modifiers[j].DurationSeconds
			}
			return modifiers[i].Magnitude < modifiers[j].Magnitude
		}
		return modifiers[i].Type < modifiers[j].Type
	})

	recycleValue := params.RecycleValue
	if recycleValue <= 0 {
		recycleValue = defaultRecycleValue
	}

	key := ComposeFungibilityKey(params.ID, params.Tier, params.QualityTags...)

	return ItemDefinition{
		ID:             params.ID,
		Class:          params.Class,
		Tier:           params.Tier,
		Stackable:      params.Stackable,
		FungibilityKey: key,
		EquipSlot:      equipSlot,
		Actions:        actionSet,
		Modifiers:      modifiers,
		RecycleValue:   recycleValue,
		Name:           params.Name,
		Description:    params.Description,
	}, nil
}

// ComposeFungibilityKey builds a deterministic key from the item id, tier, and optional quality tags.
func ComposeFungibilityKey(id ItemType, tier int, qualityTags ...string) string {
	tags := make([]string, len(qualityTags))
	copy(tags, qualityTags)
	sort.Strings(tags)
	builder := strings.Builder{}
	builder.WriteString(string(id))
	builder.WriteString(":")
	builder.WriteString(fmt.Sprintf("%d", tier))
	if len(tags) > 0 {
		builder.WriteString(":")
		builder.WriteString(strings.Join(tags, ","))
	}
	return builder.String()
}

// MarshalItemDefinitions returns the stable JSON representation for a slice of definitions.
func MarshalItemDefinitions(defs []ItemDefinition) ([]byte, error) {
	stable := make([]ItemDefinition, len(defs))
	copy(stable, defs)
	sort.Slice(stable, func(i, j int) bool {
		return stable[i].ID < stable[j].ID
	})
	return json.Marshal(stable)
}
