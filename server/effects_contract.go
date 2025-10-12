package main

// This file mirrors the canonical contract described in docs/architecture/effect-system-unification.md
// so the server and documentation remain synchronized as the unified effect system is implemented.

// -----------------------------
// Determinism & Quantization
// -----------------------------
// All effect geometry and motion use fixed-timestep integer math.
// COORD_SCALE defines the number of sub-units per tile (e.g., 16 => 1/16 tile precision).
const COORD_SCALE = 16

// Seq is a monotonic sequence id used for idempotency in transport events.
// Tick is the authoritative simulation tick number.
type Seq int64
type Tick int64

// Facing/Arc are expressed in quantized degrees (0..359) unless otherwise stated.

// DeliveryKind enumerates how an effect is delivered in the world simulation.
type DeliveryKind string

const (
	// DeliveryKindArea represents spatial effects resolved via geometry queries.
	DeliveryKindArea DeliveryKind = "area"
	// DeliveryKindTarget represents effects anchored to a specific actor.
	DeliveryKindTarget DeliveryKind = "target"
	// DeliveryKindVisual represents cosmetic-only effects with no gameplay impact.
	DeliveryKindVisual DeliveryKind = "visual"
)

// FollowMode decouples "is Target delivery" from how an instance anchors/updates its transform.
type FollowMode string

const (
	FollowNone   FollowMode = "none"
	FollowOwner  FollowMode = "owner"
	FollowTarget FollowMode = "target"
)

// GeometryShape enumerates the supported collision volumes for effects.
type GeometryShape string

const (
	GeometryShapeCircle  GeometryShape = "circle"
	GeometryShapeRect    GeometryShape = "rect"
	GeometryShapeArc     GeometryShape = "arc"
	GeometryShapeSegment GeometryShape = "segment"
	GeometryShapeCapsule GeometryShape = "capsule"
)

// MotionKind enumerates movement profiles applied to effect instances.
type MotionKind string

const (
	MotionKindNone      MotionKind = "none"
	MotionKindInstant   MotionKind = "instant"
	MotionKindLinear    MotionKind = "linear"
	MotionKindParabolic MotionKind = "parabolic"
	MotionKindFollow    MotionKind = "follow"
)

// ImpactPolicy controls how an effect resolves collisions.
type ImpactPolicy string

const (
	ImpactPolicyFirstHit   ImpactPolicy = "first-hit"
	ImpactPolicyAllInPath  ImpactPolicy = "all-in-path"
	ImpactPolicyPierceMany ImpactPolicy = "pierce"
)

// EndReason qualifies why an effect ended; used in EffectEndEvent and for analytics.
type EndReason string

const (
	EndReasonExpired   EndReason = "expired"
	EndReasonOwnerLost EndReason = "ownerLost"
	EndReasonCancelled EndReason = "cancelled"
	EndReasonMapChange EndReason = "mapChange"
)

// EffectGeometry captures the spatial payload carried by intents and instances.
type EffectGeometry struct {
	Shape    GeometryShape  `json:"shape"`
	OffsetX  int            `json:"offsetX,omitempty"`
	OffsetY  int            `json:"offsetY,omitempty"`
	Facing   int            `json:"facing,omitempty"`
	Arc      int            `json:"arc,omitempty"`
	Length   int            `json:"length,omitempty"`
	Width    int            `json:"width,omitempty"`
	Height   int            `json:"height,omitempty"`
	Radius   int            `json:"radius,omitempty"`
	Extent   int            `json:"extent,omitempty"`
	Variants map[string]int `json:"variants,omitempty"`
}

// EffectIntent represents an authoritative request to spawn an effect.
type EffectIntent struct {
	TypeID        string         `json:"typeId"`
	Delivery      DeliveryKind   `json:"delivery"`
	SourceActorID string         `json:"sourceActorId"`
	TargetActorID string         `json:"targetActorId,omitempty"`
	Geometry      EffectGeometry `json:"geometry"`
	DurationTicks int            `json:"durationTicks,omitempty"`
	TickCadence   int            `json:"tickCadence,omitempty"`
	// Parameters are small numeric knobs (damage, speed, tint indexes, etc.).
	Params map[string]int `json:"params,omitempty"`
}

// EffectMotionState tracks the in-flight motion of an instance.
type EffectMotionState struct {
	PositionX       int `json:"positionX"`
	PositionY       int `json:"positionY"`
	VelocityX       int `json:"velocityX"`
	VelocityY       int `json:"velocityY"`
	RangeRemaining  int `json:"rangeRemaining,omitempty"`
	TravelledLength int `json:"travelledLength,omitempty"`
}

// EffectDeliveryState stores the runtime state required to advance an instance.
type EffectDeliveryState struct {
	Geometry        EffectGeometry    `json:"geometry"`
	Motion          EffectMotionState `json:"motion"`
	AttachedActorID string            `json:"attachedActorId,omitempty"`
	Follow          FollowMode        `json:"follow,omitempty"`
}

// EffectBehaviorState stores cooldowns, counters, and other behavior-specific fields.
type EffectBehaviorState struct {
	TicksRemaining    int            `json:"ticksRemaining"`
	CooldownTicks     int            `json:"cooldownTicks,omitempty"`
	AccumulatedDamage int            `json:"accumulatedDamage,omitempty"`
	Stacks            map[string]int `json:"stacks,omitempty"`
	Extra             map[string]int `json:"extra,omitempty"`
}

// ReplicationSpec describes which lifecycle payloads the server emits for an effect,
// and (optionally) a whitelist of fields included in updates.
type ReplicationSpec struct {
	SendSpawn    bool            `json:"sendSpawn"`
	SendUpdates  bool            `json:"sendUpdates"`
	SendEnd      bool            `json:"sendEnd"`
	UpdateFields map[string]bool `json:"updateFields,omitempty"`
}

// EffectInstance represents a server-owned effect with live state tracked by the simulation.
type EffectInstance struct {
	ID            string              `json:"id"`
	DefinitionID  string              `json:"definitionId"`
	Definition    *EffectDefinition   `json:"definition,omitempty"`
	DeliveryState EffectDeliveryState `json:"deliveryState"`
	BehaviorState EffectBehaviorState `json:"behaviorState"`
	FollowActorID string              `json:"followActorId,omitempty"`
	Replication   ReplicationSpec     `json:"replication"`
}

// EffectHooks reference behavior callbacks associated with a definition.
type EffectHooks struct {
	OnSpawn  string `json:"onSpawn,omitempty"`
	OnTick   string `json:"onTick,omitempty"`
	OnHit    string `json:"onHit,omitempty"`
	OnExpire string `json:"onExpire,omitempty"`
}

// EffectDefinition describes the canonical behaviour for an effect type.
type EffectDefinition struct {
	TypeID        string          `json:"typeId"`
	Delivery      DeliveryKind    `json:"delivery"`
	Shape         GeometryShape   `json:"shape"`
	Motion        MotionKind      `json:"motion"`
	Impact        ImpactPolicy    `json:"impact"`
	LifetimeTicks int             `json:"lifetimeTicks"`
	PierceCount   int             `json:"pierceCount,omitempty"`
	Params        map[string]int  `json:"params,omitempty"`
	Hooks         EffectHooks     `json:"hooks"`
	Client        ReplicationSpec `json:"client"`
}

// -----------------------------
// Transport Events (Contract)
// -----------------------------
// These are journaled and broadcast. Ordering: spawn -> update -> end (per effect id).

type EffectSpawnEvent struct {
	Tick     Tick           `json:"tick"`
	Seq      Seq            `json:"seq"`
	Instance EffectInstance `json:"instance"` // baseline payload (may be a subset if UpdateFields used)
}

type EffectUpdateEvent struct {
	Tick          Tick                 `json:"tick"`
	Seq           Seq                  `json:"seq"`
	ID            string               `json:"id"`
	DeliveryState *EffectDeliveryState `json:"deliveryState,omitempty"`
	BehaviorState *EffectBehaviorState `json:"behaviorState,omitempty"`
	Params        map[string]int       `json:"params,omitempty"`
}

type EffectEndEvent struct {
	Tick   Tick      `json:"tick"`
	Seq    Seq       `json:"seq"`
	ID     string    `json:"id"`
	Reason EndReason `json:"reason"`
}

// TODO: integrate these contract types with a future EffectManager implementation so the simulation
// can migrate from legacy Effect/EffectTrigger models to the unified system described in the docs.
