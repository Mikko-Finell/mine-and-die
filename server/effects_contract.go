package main

import effectcontract "mine-and-die/server/effects/contract"

// The legacy server package historically hosted the contract types while the
// pipeline matured. These aliases keep existing code compiling while the
// canonical definitions live in server/effects/contract.

const COORD_SCALE = effectcontract.CoordScale

type (
	Seq                  = effectcontract.Seq
	Tick                 = effectcontract.Tick
	DeliveryKind         = effectcontract.DeliveryKind
	FollowMode           = effectcontract.FollowMode
	GeometryShape        = effectcontract.GeometryShape
	MotionKind           = effectcontract.MotionKind
	ImpactPolicy         = effectcontract.ImpactPolicy
	EndReason            = effectcontract.EndReason
	EndPolicyKind        = effectcontract.EndPolicyKind
	EndConditions        = effectcontract.EndConditions
	EndPolicy            = effectcontract.EndPolicy
	EffectGeometry       = effectcontract.EffectGeometry
	EffectIntent         = effectcontract.EffectIntent
	EffectMotionState    = effectcontract.EffectMotionState
	EffectDeliveryState  = effectcontract.EffectDeliveryState
	EffectBehaviorState  = effectcontract.EffectBehaviorState
	ReplicationSpec      = effectcontract.ReplicationSpec
	EffectInstance       = effectcontract.EffectInstance
	EffectHooks          = effectcontract.EffectHooks
	EffectDefinition     = effectcontract.EffectDefinition
	EffectSpawnEvent     = effectcontract.EffectSpawnEvent
	EffectUpdateEvent    = effectcontract.EffectUpdateEvent
	EffectEndEvent       = effectcontract.EffectEndEvent
	EffectLifecycleEvent = effectcontract.EffectLifecycleEvent
	Payload              = effectcontract.Payload
	ContractPayload      = effectcontract.ContractPayload
	Definition           = effectcontract.Definition
	Registry             = effectcontract.Registry
)

const (
	DeliveryKindArea   = effectcontract.DeliveryKindArea
	DeliveryKindTarget = effectcontract.DeliveryKindTarget
	DeliveryKindVisual = effectcontract.DeliveryKindVisual

	FollowNone   = effectcontract.FollowNone
	FollowOwner  = effectcontract.FollowOwner
	FollowTarget = effectcontract.FollowTarget

	GeometryShapeCircle  = effectcontract.GeometryShapeCircle
	GeometryShapeRect    = effectcontract.GeometryShapeRect
	GeometryShapeArc     = effectcontract.GeometryShapeArc
	GeometryShapeSegment = effectcontract.GeometryShapeSegment
	GeometryShapeCapsule = effectcontract.GeometryShapeCapsule

	MotionKindNone      = effectcontract.MotionKindNone
	MotionKindInstant   = effectcontract.MotionKindInstant
	MotionKindLinear    = effectcontract.MotionKindLinear
	MotionKindParabolic = effectcontract.MotionKindParabolic
	MotionKindFollow    = effectcontract.MotionKindFollow

	ImpactPolicyFirstHit   = effectcontract.ImpactPolicyFirstHit
	ImpactPolicyAllInPath  = effectcontract.ImpactPolicyAllInPath
	ImpactPolicyPierceMany = effectcontract.ImpactPolicyPierceMany
	ImpactPolicyNone       = effectcontract.ImpactPolicyNone

	EndReasonExpired   = effectcontract.EndReasonExpired
	EndReasonOwnerLost = effectcontract.EndReasonOwnerLost
	EndReasonCancelled = effectcontract.EndReasonCancelled
	EndReasonMapChange = effectcontract.EndReasonMapChange

	EndDuration  = effectcontract.EndDuration
	EndInstant   = effectcontract.EndInstant
	EndCondition = effectcontract.EndCondition

	EffectIDAttack        = effectcontract.EffectIDAttack
	EffectIDFireball      = effectcontract.EffectIDFireball
	EffectIDBloodSplatter = effectcontract.EffectIDBloodSplatter
	EffectIDBurningTick   = effectcontract.EffectIDBurningTick
	EffectIDBurningVisual = effectcontract.EffectIDBurningVisual
)

var NoPayload = effectcontract.NoPayload

var BuiltInRegistry = effectcontract.BuiltInRegistry
