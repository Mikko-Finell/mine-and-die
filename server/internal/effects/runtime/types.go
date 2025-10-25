package runtime

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// StatusEffectType identifies a status effect applied to an actor.
type StatusEffectType string

// State stores the authoritative runtime bookkeeping for contract-driven
// effects. The struct mirrors key fields from the historical legacy pipeline so
// existing gameplay systems and telemetry remain stable while the contract
// manager owns instance lifecycles.
type State struct {
	ID       string
	Type     string
	Owner    string
	Start    int64
	Duration int64
	X        float64
	Y        float64
	Width    float64
	Height   float64
	Params   map[string]float64
	Colors   []string

	Instance              effectcontract.EffectInstance
	ExpiresAt             time.Time
	Projectile            *ProjectileState
	FollowActorID         string
	StatusEffect          StatusEffectType
	Version               uint64
	TelemetryEnded        bool
	ContractManaged       bool
	TelemetrySpawnTick    effectcontract.Tick
	TelemetryFirstHitTick effectcontract.Tick
	TelemetryHitCount     int
	TelemetryVictims      map[string]struct{}
	TelemetryDamage       float64
}

// ProjectileTemplate feeds the legacy projectile factory. Future contract
// catalogs will replace these ad-hoc templates with structured definitions.
type ProjectileTemplate struct {
	Type           string
	Speed          float64
	MaxDistance    float64
	Lifetime       time.Duration
	SpawnRadius    float64
	SpawnOffset    float64
	CollisionShape CollisionShapeConfig
	TravelMode     TravelModeConfig
	ImpactRules    ImpactRuleConfig
	Params         map[string]float64
	Cooldown       time.Duration
}

// CollisionShapeConfig encodes the legacy projectile collision shape data.
type CollisionShapeConfig struct {
	RectWidth  float64
	RectHeight float64
	UseRect    bool
}

// TravelModeConfig captures the legacy projectile motion configuration.
type TravelModeConfig struct {
	StraightLine bool
}

// ImpactRuleConfig encodes legacy projectile impact policies.
type ImpactRuleConfig struct {
	StopOnHit          bool
	MaxTargets         int
	AffectsOwner       bool
	ExplodeOnImpact    *ExplosionSpec
	ExplodeOnExpiry    *ExplosionSpec
	ExpiryOnlyIfNoHits bool
}

// ExplosionSpec is part of the legacy projectile explosion flow.
type ExplosionSpec struct {
	EffectType string
	Radius     float64
	Duration   time.Duration
	Params     map[string]float64
}

// ProjectileState tracks runtime state for legacy projectiles. The contract
// manager maintains it only to bridge existing mechanics until structured
// definitions own motion and collision.
type ProjectileState struct {
	Template       *ProjectileTemplate
	VelocityUnitX  float64
	VelocityUnitY  float64
	RemainingRange float64
	HitCount       int
	ExpiryResolved bool
	HitActors      map[string]struct{}
}

// MarkHit records that the projectile has struck the provided actor ID.
// It returns true when the hit is new so callers can gate duplicate events.
func (p *ProjectileState) MarkHit(id string) bool {
	if p == nil || id == "" {
		return false
	}
	if p.HitActors == nil {
		p.HitActors = make(map[string]struct{})
	}
	if _, exists := p.HitActors[id]; exists {
		return false
	}
	p.HitActors[id] = struct{}{}
	p.HitCount++
	return true
}
