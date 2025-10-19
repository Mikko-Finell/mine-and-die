package main

import "mine-and-die/server/internal/sim"

func toSimFacing(dir FacingDirection) sim.FacingDirection {
	switch dir {
	case FacingUp:
		return sim.FacingUp
	case FacingDown:
		return sim.FacingDown
	case FacingLeft:
		return sim.FacingLeft
	case FacingRight:
		return sim.FacingRight
	default:
		return ""
	}
}

func legacyFacingFromSim(dir sim.FacingDirection) FacingDirection {
	switch dir {
	case sim.FacingUp:
		return FacingUp
	case sim.FacingDown:
		return FacingDown
	case sim.FacingLeft:
		return FacingLeft
	case sim.FacingRight:
		return FacingRight
	default:
		return ""
	}
}

func toSimCommandType(value CommandType) sim.CommandType {
	switch value {
	case CommandMove:
		return sim.CommandMove
	case CommandAction:
		return sim.CommandAction
	case CommandHeartbeat:
		return sim.CommandHeartbeat
	case CommandSetPath:
		return sim.CommandSetPath
	case CommandClearPath:
		return sim.CommandClearPath
	default:
		return ""
	}
}

func legacyCommandTypeFromSim(value sim.CommandType) CommandType {
	switch value {
	case sim.CommandMove:
		return CommandMove
	case sim.CommandAction:
		return CommandAction
	case sim.CommandHeartbeat:
		return CommandHeartbeat
	case sim.CommandSetPath:
		return CommandSetPath
	case sim.CommandClearPath:
		return CommandClearPath
	default:
		return ""
	}
}

func toSimNPCType(value NPCType) sim.NPCType {
	switch value {
	case NPCTypeGoblin:
		return sim.NPCTypeGoblin
	case NPCTypeRat:
		return sim.NPCTypeRat
	default:
		return ""
	}
}

func legacyNPCTypeFromSim(value sim.NPCType) NPCType {
	switch value {
	case sim.NPCTypeGoblin:
		return NPCTypeGoblin
	case sim.NPCTypeRat:
		return NPCTypeRat
	default:
		return ""
	}
}

func toSimEquipSlot(value EquipSlot) sim.EquipSlot {
	switch value {
	case EquipSlotMainHand:
		return sim.EquipSlotMainHand
	case EquipSlotOffHand:
		return sim.EquipSlotOffHand
	case EquipSlotHead:
		return sim.EquipSlotHead
	case EquipSlotBody:
		return sim.EquipSlotBody
	case EquipSlotGloves:
		return sim.EquipSlotGloves
	case EquipSlotBoots:
		return sim.EquipSlotBoots
	case EquipSlotAccessory:
		return sim.EquipSlotAccessory
	default:
		return ""
	}
}

func legacyEquipSlotFromSim(value sim.EquipSlot) EquipSlot {
	switch value {
	case sim.EquipSlotMainHand:
		return EquipSlotMainHand
	case sim.EquipSlotOffHand:
		return EquipSlotOffHand
	case sim.EquipSlotHead:
		return EquipSlotHead
	case sim.EquipSlotBody:
		return EquipSlotBody
	case sim.EquipSlotGloves:
		return EquipSlotGloves
	case sim.EquipSlotBoots:
		return EquipSlotBoots
	case sim.EquipSlotAccessory:
		return EquipSlotAccessory
	default:
		return ""
	}
}

func toSimPatchKind(value PatchKind) sim.PatchKind {
	switch value {
	case PatchPlayerPos:
		return sim.PatchPlayerPos
	case PatchPlayerFacing:
		return sim.PatchPlayerFacing
	case PatchPlayerIntent:
		return sim.PatchPlayerIntent
	case PatchPlayerHealth:
		return sim.PatchPlayerHealth
	case PatchPlayerInventory:
		return sim.PatchPlayerInventory
	case PatchPlayerEquipment:
		return sim.PatchPlayerEquipment
	case PatchPlayerRemoved:
		return sim.PatchPlayerRemoved
	case PatchNPCPos:
		return sim.PatchNPCPos
	case PatchNPCFacing:
		return sim.PatchNPCFacing
	case PatchNPCHealth:
		return sim.PatchNPCHealth
	case PatchNPCInventory:
		return sim.PatchNPCInventory
	case PatchNPCEquipment:
		return sim.PatchNPCEquipment
	case PatchEffectPos:
		return sim.PatchEffectPos
	case PatchEffectParams:
		return sim.PatchEffectParams
	case PatchGroundItemPos:
		return sim.PatchGroundItemPos
	case PatchGroundItemQty:
		return sim.PatchGroundItemQty
	default:
		return ""
	}
}

func legacyPatchKindFromSim(value sim.PatchKind) PatchKind {
	switch value {
	case sim.PatchPlayerPos:
		return PatchPlayerPos
	case sim.PatchPlayerFacing:
		return PatchPlayerFacing
	case sim.PatchPlayerIntent:
		return PatchPlayerIntent
	case sim.PatchPlayerHealth:
		return PatchPlayerHealth
	case sim.PatchPlayerInventory:
		return PatchPlayerInventory
	case sim.PatchPlayerEquipment:
		return PatchPlayerEquipment
	case sim.PatchPlayerRemoved:
		return PatchPlayerRemoved
	case sim.PatchNPCPos:
		return PatchNPCPos
	case sim.PatchNPCFacing:
		return PatchNPCFacing
	case sim.PatchNPCHealth:
		return PatchNPCHealth
	case sim.PatchNPCInventory:
		return PatchNPCInventory
	case sim.PatchNPCEquipment:
		return PatchNPCEquipment
	case sim.PatchEffectPos:
		return PatchEffectPos
	case sim.PatchEffectParams:
		return PatchEffectParams
	case sim.PatchGroundItemPos:
		return PatchGroundItemPos
	case sim.PatchGroundItemQty:
		return PatchGroundItemQty
	default:
		return ""
	}
}

func toSimItemType(value ItemType) sim.ItemType {
	return sim.ItemType(string(value))
}

func legacyItemTypeFromSim(value sim.ItemType) ItemType {
	return ItemType(string(value))
}
