package server

import (
	"fmt"

	"mine-and-die/server/internal/state"
	stats "mine-and-die/server/stats"
)

type (
	EquippedItem = state.EquippedItem
	Equipment    = state.Equipment
)

func NewEquipment() Equipment {
	return state.NewEquipment()
}

func equipmentDeltaForDefinition(def ItemDefinition) (stats.StatDelta, error) {
	if def.ID == "" {
		return stats.NewStatDelta(), fmt.Errorf("item definition missing id")
	}
	delta := stats.NewStatDelta()
	for _, mod := range def.Modifiers {
		if mod.DurationSeconds > 0 {
			continue
		}
		switch mod.Type {
		case "attack_power":
			delta.Add[stats.StatMight] += mod.Magnitude
		case "armor_flat":
			delta.Add[stats.StatResonance] += mod.Magnitude
		case "focus_flat":
			delta.Add[stats.StatFocus] += mod.Magnitude
		case "speed_flat":
			delta.Add[stats.StatSpeed] += mod.Magnitude
		case "stamina_regen":
			delta.Add[stats.StatSpeed] += mod.Magnitude
		}
	}
	return delta, nil
}

func equipSlotRank(slot EquipSlot) int {
	return state.EquipSlotRank(slot)
}

func equipSlotFromOrdinal(idx int) (EquipSlot, bool) {
	return state.EquipSlotFromOrdinal(idx)
}

func equipmentsEqual(a, b Equipment) bool {
	return state.EquipmentsEqual(a, b)
}

func cloneEquipmentSlots(slots []EquippedItem) []EquippedItem {
	return state.CloneEquipmentSlots(slots)
}
