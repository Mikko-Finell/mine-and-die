package typed

import (
	"mine-and-die/server/internal/sim"

	effectcontract "mine-and-die/server/effects/contract"
)

type PatchKind = sim.PatchKind

const (
	PatchPlayerPos       = sim.PatchPlayerPos
	PatchPlayerFacing    = sim.PatchPlayerFacing
	PatchPlayerIntent    = sim.PatchPlayerIntent
	PatchPlayerHealth    = sim.PatchPlayerHealth
	PatchPlayerInventory = sim.PatchPlayerInventory
	PatchPlayerEquipment = sim.PatchPlayerEquipment
	PatchPlayerRemoved   = sim.PatchPlayerRemoved

	PatchNPCPos       = sim.PatchNPCPos
	PatchNPCFacing    = sim.PatchNPCFacing
	PatchNPCHealth    = sim.PatchNPCHealth
	PatchNPCInventory = sim.PatchNPCInventory
	PatchNPCEquipment = sim.PatchNPCEquipment

	PatchEffectPos    = sim.PatchEffectPos
	PatchEffectParams = sim.PatchEffectParams

	PatchGroundItemPos = sim.PatchGroundItemPos
	PatchGroundItemQty = sim.PatchGroundItemQty
)

type Patch = sim.Patch

type PositionPayload = sim.PositionPayload

type PlayerPosPayload = sim.PlayerPosPayload

type NPCPosPayload = sim.NPCPosPayload

type EffectPosPayload = sim.EffectPosPayload

type GroundItemPosPayload = sim.GroundItemPosPayload

type FacingPayload = sim.FacingPayload

type PlayerFacingPayload = sim.PlayerFacingPayload

type NPCFacingPayload = sim.NPCFacingPayload

type PlayerIntentPayload = sim.PlayerIntentPayload

type HealthPayload = sim.HealthPayload

type PlayerHealthPayload = sim.PlayerHealthPayload

type NPCHealthPayload = sim.NPCHealthPayload

type InventoryPayload = sim.InventoryPayload

type PlayerInventoryPayload = sim.PlayerInventoryPayload

type NPCInventoryPayload = sim.NPCInventoryPayload

type EquipmentPayload = sim.EquipmentPayload

type PlayerEquipmentPayload = sim.PlayerEquipmentPayload

type NPCEquipmentPayload = sim.NPCEquipmentPayload

type EffectParamsPayload = sim.EffectParamsPayload

type GroundItemQtyPayload = sim.GroundItemQtyPayload

type EffectEventBatch = sim.EffectEventBatch

type EffectSpawnEvent = effectcontract.EffectSpawnEvent

type EffectUpdateEvent = effectcontract.EffectUpdateEvent

type EffectEndEvent = effectcontract.EffectEndEvent

type EffectInstance = effectcontract.EffectInstance

type EffectDefinition = effectcontract.EffectDefinition

type EffectDeliveryState = effectcontract.EffectDeliveryState

type EffectBehaviorState = effectcontract.EffectBehaviorState

type EffectGeometry = effectcontract.EffectGeometry

type EffectMotionState = effectcontract.EffectMotionState

type ReplicationSpec = effectcontract.ReplicationSpec

type EffectResyncSignal = sim.EffectResyncSignal

type EffectResyncReason = sim.EffectResyncReason

type EffectSeq = effectcontract.Seq
