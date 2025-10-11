package main

import "sort"

const (
	ItemTypeGold          ItemType = "gold"
	ItemTypeHealthPotion  ItemType = "health_potion"
	ItemTypeRatTail       ItemType = "rat_tail"
	ItemTypeIronDagger    ItemType = "iron_dagger"
	ItemTypeLeatherJerkin ItemType = "leather_jerkin"
	ItemTypeTravelerCharm ItemType = "traveler_charm"
	ItemTypeVenomCoating  ItemType = "venom_coating"
	ItemTypeBlastingOrb   ItemType = "blasting_orb"
	ItemTypeRefinedOre    ItemType = "refined_ore"
)

var itemCatalog = buildItemCatalog()

func buildItemCatalog() map[ItemType]ItemDefinition {
	defs := []ItemDefinition{
		mustDefine(ItemDefinitionParams{
			ID:          ItemTypeGold,
			Class:       ItemClassProcessedMaterial,
			Tier:        1,
			Stackable:   true,
			Actions:     nil,
			Modifiers:   nil,
			QualityTags: []string{"coin"},
			Name:        "Gold Coin",
			Description: "Currency minted by the colony. Stackable with no limits.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:        ItemTypeHealthPotion,
			Class:     ItemClassConsumable,
			Tier:      1,
			Stackable: true,
			Actions:   []ItemAction{ItemActionConsume},
			Modifiers: []ItemModifier{
				{Type: "heal_flat", Magnitude: 25},
			},
			QualityTags: []string{"lesser"},
			Name:        "Lesser Healing Potion",
			Description: "Restores a small amount of health when consumed.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:          ItemTypeRatTail,
			Class:       ItemClassProcessedMaterial,
			Tier:        0,
			Stackable:   true,
			Actions:     nil,
			QualityTags: []string{"catalyst"},
			Name:        "Rat Tail",
			Description: "A matted tail harvested from an oversized rat.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:        ItemTypeIronDagger,
			Class:     ItemClassWeapon,
			Tier:      1,
			Stackable: false,
			EquipSlot: EquipSlotMainHand,
			Actions:   []ItemAction{ItemActionAttack},
			Modifiers: []ItemModifier{
				{Type: "attack_power", Magnitude: 4},
			},
			QualityTags: []string{"iron", "dagger"},
			Name:        "Iron Dagger",
			Description: "A balanced dagger suited for close encounters.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:        ItemTypeLeatherJerkin,
			Class:     ItemClassArmor,
			Tier:      1,
			Stackable: false,
			EquipSlot: EquipSlotBody,
			Modifiers: []ItemModifier{
				{Type: "armor_flat", Magnitude: 6},
			},
			QualityTags: []string{"leather", "light_armor"},
			Name:        "Leather Jerkin",
			Description: "Simple body armor providing modest protection.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:        ItemTypeTravelerCharm,
			Class:     ItemClassAccessory,
			Tier:      1,
			Stackable: false,
			EquipSlot: EquipSlotAccessory,
			Actions:   []ItemAction{ItemActionActivate},
			Modifiers: []ItemModifier{
				{Type: "stamina_regen", Magnitude: 1.5},
			},
			QualityTags: []string{"charm"},
			Name:        "Traveler's Charm",
			Description: "An accessory that slightly improves stamina recovery.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:        ItemTypeVenomCoating,
			Class:     ItemClassCoating,
			Tier:      2,
			Stackable: true,
			Actions:   []ItemAction{ItemActionApply},
			Modifiers: []ItemModifier{
				{Type: "on_hit_poison", Magnitude: 8, DurationSeconds: 10},
			},
			QualityTags: []string{"venom"},
			Name:        "Venom Coating",
			Description: "A vial that imbues a weapon with poison on hit.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:        ItemTypeBlastingOrb,
			Class:     ItemClassThrowable,
			Tier:      2,
			Stackable: true,
			Actions:   []ItemAction{ItemActionThrow},
			Modifiers: []ItemModifier{
				{Type: "aoe_fire", Magnitude: 12, DurationSeconds: 4},
			},
			QualityTags: []string{"orb", "explosive"},
			Name:        "Blasting Orb",
			Description: "A throwable orb that explodes in a fiery burst.",
		}),
		mustDefine(ItemDefinitionParams{
			ID:          ItemTypeRefinedOre,
			Class:       ItemClassProcessedMaterial,
			Tier:        1,
			Stackable:   true,
			Actions:     nil,
			QualityTags: []string{"ingot"},
			Name:        "Refined Ore",
			Description: "Smelted ore ready for advanced crafting recipes.",
		}),
	}

	catalog := make(map[ItemType]ItemDefinition, len(defs))
	for _, def := range defs {
		catalog[def.ID] = def
	}
	return catalog
}

func mustDefine(params ItemDefinitionParams) ItemDefinition {
	def, err := NewItemDefinition(params)
	if err != nil {
		panic(err)
	}
	return def
}

// ItemDefinitionFor fetches the definition for a given item type.
func ItemDefinitionFor(itemType ItemType) (ItemDefinition, bool) {
	def, ok := itemCatalog[itemType]
	return def, ok
}

// ItemDefinitions returns the list of definitions sorted by identifier.
func ItemDefinitions() []ItemDefinition {
	defs := make([]ItemDefinition, 0, len(itemCatalog))
	for _, def := range itemCatalog {
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].ID < defs[j].ID
	})
	return defs
}
