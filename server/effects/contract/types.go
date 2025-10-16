package contract

// CoordScale defines the number of sub-units per tile (e.g., 16 => 1/16 tile precision).
const CoordScale = 16

// Seq is a monotonic sequence id used for idempotency in transport events.
// Tick is the authoritative simulation tick number.
type Seq int64
type Tick int64

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
	ImpactPolicyNone       ImpactPolicy = "none"
)

// EndReason qualifies why an effect ended; used in EffectEndEvent and for analytics.
type EndReason string

const (
	EndReasonExpired   EndReason = "expired"
	EndReasonOwnerLost EndReason = "ownerLost"
	EndReasonCancelled EndReason = "cancelled"
	EndReasonMapChange EndReason = "mapChange"
)

// EndPolicyKind describes how an effect instance determines when it ends.
type EndPolicyKind uint8

const (
	// EndDuration ends an instance after its configured lifetime elapses.
	EndDuration EndPolicyKind = iota
	// EndInstant ends an instance in the same tick after it applies once.
	EndInstant
	// EndCondition ends when runtime conditions evaluate to true.
	EndCondition
)

// EndConditions enumerates the runtime checks an EndCondition policy can perform.
type EndConditions struct {
	OnUnequip        bool `json:"onUnequip" jsonschema:"description=End when the owning actor unequips the source item."`
	OnOwnerDeath     bool `json:"onOwnerDeath" jsonschema:"description=End when the owning actor dies."`
	OnOwnerLost      bool `json:"onOwnerLost" jsonschema:"description=End when the effect loses its owning actor (e.g. despawn)."`
	OnZoneChange     bool `json:"onZoneChange" jsonschema:"description=End when the owning actor changes zones."`
	OnExplicitCancel bool `json:"onExplicitCancel" jsonschema:"description=End when gameplay explicitly cancels the effect."`
}

// EndPolicy captures the configured lifecycle policy for an effect definition.
type EndPolicy struct {
	Kind       EndPolicyKind `json:"kind" jsonschema:"title=Termination Policy,description=Determines how the effect instance decides to stop.,enum=0,enum=1,enum=2,required"`
	Conditions EndConditions `json:"conditions,omitempty" jsonschema:"description=Conditional checks evaluated when Kind is set to EndCondition."`
}

// EffectGeometry captures the spatial payload carried by intents and instances.
type EffectGeometry struct {
	Shape    GeometryShape  `json:"shape" jsonschema:"title=Collision Shape,description=Defines the geometry used for collision tests.,enum=circle,enum=rect,enum=arc,enum=segment,enum=capsule,required"`
	OffsetX  int            `json:"offsetX,omitempty" jsonschema:"description=Horizontal offset from the source position."`
	OffsetY  int            `json:"offsetY,omitempty" jsonschema:"description=Vertical offset from the source position."`
	Facing   int            `json:"facing,omitempty" jsonschema:"description=Facing override when the geometry has orientation."`
	Arc      int            `json:"arc,omitempty" jsonschema:"description=Arc angle in degrees for arc shapes."`
	Length   int            `json:"length,omitempty" jsonschema:"description=Length of the geometry in world units."`
	Width    int            `json:"width,omitempty" jsonschema:"description=Width of the geometry in world units."`
	Height   int            `json:"height,omitempty" jsonschema:"description=Height of the geometry in world units."`
	Radius   int            `json:"radius,omitempty" jsonschema:"description=Radius used by circular and capsule geometries."`
	Extent   int            `json:"extent,omitempty" jsonschema:"description=Additional extent used by segment shapes."`
	Variants map[string]int `json:"variants,omitempty" jsonschema:"description=Named geometry presets keyed by designer-defined identifiers."`
}

// EffectIntent represents an authoritative request to spawn an effect.
type EffectIntent struct {
	EntryID       string         `json:"entryId,omitempty"`
	TypeID        string         `json:"typeId"`
	Delivery      DeliveryKind   `json:"delivery"`
	SourceActorID string         `json:"sourceActorId"`
	TargetActorID string         `json:"targetActorId,omitempty"`
	Geometry      EffectGeometry `json:"geometry"`
	DurationTicks int            `json:"durationTicks,omitempty"`
	TickCadence   int            `json:"tickCadence,omitempty"`
	// Params are small numeric knobs (damage, speed, tint indexes, etc.).
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
	TickCadence       int            `json:"tickCadence,omitempty"`
	AccumulatedDamage int            `json:"accumulatedDamage,omitempty"`
	Stacks            map[string]int `json:"stacks,omitempty"`
	Extra             map[string]int `json:"extra,omitempty"`
}

// ReplicationSpec describes which lifecycle payloads the server emits for an effect,
// (optionally) a whitelist of fields included in updates, and who manages the
// visual lifecycle once the contract signals completion.
type ReplicationSpec struct {
	SendSpawn       bool            `json:"sendSpawn" jsonschema:"description=Whether the server emits spawn payloads to clients.,required"`
	SendUpdates     bool            `json:"sendUpdates" jsonschema:"description=Whether the server emits incremental update payloads.,required"`
	SendEnd         bool            `json:"sendEnd" jsonschema:"description=Whether the server emits end payloads.,required"`
	ManagedByClient bool            `json:"managedByClient,omitempty" jsonschema:"description=Marks effects that transition to client-side ownership after spawn."`
	UpdateFields    map[string]bool `json:"updateFields,omitempty" jsonschema:"description=Optional whitelist of fields sent during updates."`
}

// EffectInstance represents a server-owned effect with live state tracked by the simulation.
type EffectInstance struct {
	ID            string              `json:"id"`
	EntryID       string              `json:"entryId,omitempty"`
	DefinitionID  string              `json:"definitionId"`
	Definition    *EffectDefinition   `json:"definition,omitempty"`
	StartTick     Tick                `json:"startTick"`
	DeliveryState EffectDeliveryState `json:"deliveryState"`
	BehaviorState EffectBehaviorState `json:"behaviorState"`
	Params        map[string]int      `json:"params,omitempty"`
	Colors        []string            `json:"colors,omitempty"`
	FollowActorID string              `json:"followActorId,omitempty"`
	OwnerActorID  string              `json:"ownerActorId,omitempty"`
	Replication   ReplicationSpec     `json:"replication"`
	End           EndPolicy           `json:"end"`
}

// EffectHooks reference behavior callbacks associated with a definition.
type EffectHooks struct {
	OnSpawn  string `json:"onSpawn,omitempty" jsonschema:"description=Callback invoked when the effect instance spawns."`
	OnTick   string `json:"onTick,omitempty" jsonschema:"description=Callback invoked on each simulation tick."`
	OnHit    string `json:"onHit,omitempty" jsonschema:"description=Callback invoked when the effect collides with a target."`
	OnExpire string `json:"onExpire,omitempty" jsonschema:"description=Callback invoked when the effect expires naturally."`
}

// EffectDefinition describes the canonical behaviour for an effect type.
type EffectDefinition struct {
	TypeID        string          `json:"typeId" jsonschema:"title=Effect Type ID,description=Canonical identifier for the gameplay effect.,pattern=^[a-z0-9-]+$,minLength=1,required"`
	Delivery      DeliveryKind    `json:"delivery" jsonschema:"title=Delivery Mode,description=How the effect is delivered in the world.,enum=area,enum=target,enum=visual,required"`
	Shape         GeometryShape   `json:"shape" jsonschema:"title=Primary Shape,description=Default geometry used by the effect.,enum=circle,enum=rect,enum=arc,enum=segment,enum=capsule,required"`
	Motion        MotionKind      `json:"motion" jsonschema:"title=Motion Profile,description=Movement behaviour applied to the instance.,enum=none,enum=instant,enum=linear,enum=parabolic,enum=follow,required"`
	Impact        ImpactPolicy    `json:"impact" jsonschema:"title=Impact Policy,description=Collision resolution policy.,enum=first-hit,enum=all-in-path,enum=pierce,enum=none,required"`
	LifetimeTicks int             `json:"lifetimeTicks" jsonschema:"title=Lifetime Ticks,description=Duration in simulation ticks before expiry.,minimum=0,required"`
	PierceCount   int             `json:"pierceCount,omitempty" jsonschema:"description=Number of additional targets an instance may pierce.,minimum=0"`
	Params        map[string]int  `json:"params,omitempty" jsonschema:"description=Optional numeric designer parameters exposed to gameplay."`
	Hooks         EffectHooks     `json:"hooks" jsonschema:"description=Lifecycle callbacks executed by the server runtime.,required"`
	Client        ReplicationSpec `json:"client" jsonschema:"description=Authoritative replication contract for clients.,required"`
	End           EndPolicy       `json:"end" jsonschema:"description=Termination behaviour configuration.,required"`
}

// EffectSpawnEvent represents the authoritative spawn payload broadcast to clients.
type EffectSpawnEvent struct {
	Tick     Tick           `json:"tick"`
	Seq      Seq            `json:"seq"`
	Instance EffectInstance `json:"instance"` // baseline payload (may be a subset if UpdateFields used)
}

// EffectUpdateEvent captures partial updates emitted for an active effect instance.
type EffectUpdateEvent struct {
	Tick          Tick                 `json:"tick"`
	Seq           Seq                  `json:"seq"`
	ID            string               `json:"id"`
	DeliveryState *EffectDeliveryState `json:"deliveryState,omitempty"`
	BehaviorState *EffectBehaviorState `json:"behaviorState,omitempty"`
	Params        map[string]int       `json:"params,omitempty"`
}

// EffectEndEvent denotes the authoritative termination of an effect instance.
type EffectEndEvent struct {
	Tick   Tick      `json:"tick"`
	Seq    Seq       `json:"seq"`
	ID     string    `json:"id"`
	Reason EndReason `json:"reason"`
}

// EffectLifecycleEvent provides a shared type for callbacks that need to handle
// any of the lifecycle payloads emitted by the contract-driven manager.
type EffectLifecycleEvent interface {
	isEffectLifecycleEvent()
}

func (EffectSpawnEvent) isEffectLifecycleEvent()  {}
func (EffectUpdateEvent) isEffectLifecycleEvent() {}
func (EffectEndEvent) isEffectLifecycleEvent()    {}
