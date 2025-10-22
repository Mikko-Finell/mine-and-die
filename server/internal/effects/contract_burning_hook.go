package effects

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	worldpkg "mine-and-die/server/internal/world"
)

// ContractStatusInstance captures the status effect instance metadata required
// by the contract-managed burning visual hook. Callers provide the legacy
// instance so the hook can attach the spawned effect and read the expiry while
// keeping lifetime bookkeeping behind world adapters.
type ContractStatusInstance struct {
	Instance  worldpkg.StatusEffectInstance
	ExpiresAt func() time.Time
}

// ContractStatusActor exposes the subset of actor state required by the
// contract-managed burning visual hook. Position fields mirror the legacy
// world actor coordinates so the visual can follow the target deterministically.
type ContractStatusActor struct {
	ID                 string
	X                  float64
	Y                  float64
	StatusInstance     *ContractStatusInstance
	ApplyBurningDamage func(ownerID string, status StatusEffectType, delta float64, now time.Time)
}

// ContractBurningVisualHookConfig bundles the dependencies required to keep the
// contract-managed burning visual in sync with the legacy world state while the
// hook lives inside the effects package.
type ContractBurningVisualHookConfig struct {
	StatusEffect      StatusEffectType
	DefaultLifetime   time.Duration
	FallbackLifetime  time.Duration
	TileSize          float64
	DefaultFootprint  float64
	TickRate          int
	LookupActor       func(actorID string) *ContractStatusActor
	ExtendLifetime    func(worldpkg.StatusEffectLifetimeFields, time.Time)
	ExpireLifetime    func(worldpkg.StatusEffectLifetimeFields, time.Time)
	RecordEffectSpawn func(effectType, category string)
}

// ContractBurningVisualHook returns the spawn and tick handlers that keep the
// contract-managed burning visual effect synchronized with its world state.
func ContractBurningVisualHook(cfg ContractBurningVisualHookConfig) HookSet {
	return HookSet{
		OnSpawn: func(rt Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
			if instance == nil {
				return
			}

			actor := lookupContractStatusActor(cfg.LookupActor, instance)
			effect := LoadRuntimeEffect(rt, instance.ID)

			if effect == nil && actor != nil {
				lifetime := TicksToDuration(instance.BehaviorState.TicksRemaining, cfg.TickRate)
				if lifetime <= 0 {
					lifetime = cfg.DefaultLifetime
				}

				target := &StatusVisualTarget{ID: actor.ID, X: actor.X, Y: actor.Y}
				effect = SpawnContractStatusVisualFromInstance(StatusVisualSpawnConfig{
					Instance:         instance,
					Target:           target,
					Lifetime:         lifetime,
					Now:              now,
					DefaultFootprint: cfg.DefaultFootprint,
					FallbackLifetime: cfg.FallbackLifetime,
					StatusEffect:     cfg.StatusEffect,
				})
				if effect != nil {
					attachAndExtendStatusVisual(cfg, actor.StatusInstance, effect)
					if !RegisterRuntimeEffect(rt, effect) {
						instance.BehaviorState.TicksRemaining = 0
						effect = nil
					} else {
						recordContractStatusVisualSpawn(cfg, effect.Type)
						StoreRuntimeEffect(rt, instance.ID, effect)
					}
				}
			}

			syncContractStatusVisual(cfg, instance, effect, actor)
		},
		OnTick: func(rt Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
			if instance == nil {
				return
			}

			actor := lookupContractStatusActor(cfg.LookupActor, instance)
			effect := LoadRuntimeEffect(rt, instance.ID)

			if effect == nil && actor != nil {
				lifetime := TicksToDuration(instance.BehaviorState.TicksRemaining, cfg.TickRate)
				if lifetime <= 0 {
					lifetime = cfg.DefaultLifetime
				}
				target := &StatusVisualTarget{ID: actor.ID, X: actor.X, Y: actor.Y}
				effect = SpawnContractStatusVisualFromInstance(StatusVisualSpawnConfig{
					Instance:         instance,
					Target:           target,
					Lifetime:         lifetime,
					Now:              now,
					DefaultFootprint: cfg.DefaultFootprint,
					FallbackLifetime: cfg.FallbackLifetime,
					StatusEffect:     cfg.StatusEffect,
				})
				if effect != nil {
					attachAndExtendStatusVisual(cfg, actor.StatusInstance, effect)
					if !RegisterRuntimeEffect(rt, effect) {
						instance.BehaviorState.TicksRemaining = 0
						effect = nil
					} else {
						recordContractStatusVisualSpawn(cfg, effect.Type)
						StoreRuntimeEffect(rt, instance.ID, effect)
					}
				}
			}

			syncContractStatusVisual(cfg, instance, effect, actor)

			if effect == nil {
				return
			}

			if actor != nil && actor.StatusInstance != nil {
				extendContractStatusVisual(cfg, actor.StatusInstance, effect, now, instance)
			} else {
				expireContractStatusVisual(cfg, effect, now)
			}
		},
	}
}

func lookupContractStatusActor(lookup func(string) *ContractStatusActor, instance *effectcontract.EffectInstance) *ContractStatusActor {
	if instance == nil || lookup == nil {
		return nil
	}
	targetID := instance.FollowActorID
	if targetID == "" {
		targetID = instance.DeliveryState.AttachedActorID
	}
	if targetID == "" {
		return nil
	}
	return lookup(targetID)
}

func attachAndExtendStatusVisual(cfg ContractBurningVisualHookConfig, inst *ContractStatusInstance, effect *State) {
	if inst == nil || inst.Instance == nil || effect == nil {
		return
	}
	worldpkg.AttachStatusEffectVisual(worldpkg.AttachStatusEffectVisualConfig{
		Instance:    inst.Instance,
		Effect:      statusEffectVisualAdapter{state: effect},
		DefaultType: string(cfg.StatusEffect),
	})
	if cfg.ExtendLifetime != nil && inst.ExpiresAt != nil {
		cfg.ExtendLifetime(statusEffectLifetimeFields(effect), inst.ExpiresAt())
	}
}

func extendContractStatusVisual(cfg ContractBurningVisualHookConfig, inst *ContractStatusInstance, effect *State, now time.Time, instance *effectcontract.EffectInstance) {
	if inst == nil || inst.Instance == nil {
		expireContractStatusVisual(cfg, effect, now)
		return
	}
	if cfg.ExtendLifetime != nil && inst.ExpiresAt != nil {
		cfg.ExtendLifetime(statusEffectLifetimeFields(effect), inst.ExpiresAt())
	}
	if inst.ExpiresAt == nil {
		return
	}
	remaining := inst.ExpiresAt().Sub(now)
	if remaining < 0 {
		remaining = 0
	}
	ticksRemaining := durationToTicks(remaining, cfg.TickRate)
	if remaining > 0 && ticksRemaining < 1 {
		ticksRemaining = 1
	}
	if instance != nil {
		instance.BehaviorState.TicksRemaining = ticksRemaining
	}
}

func expireContractStatusVisual(cfg ContractBurningVisualHookConfig, effect *State, now time.Time) {
	if effect == nil || cfg.ExpireLifetime == nil {
		return
	}
	cfg.ExpireLifetime(statusEffectLifetimeFields(effect), now)
}

func statusEffectLifetimeFields(effect *State) worldpkg.StatusEffectLifetimeFields {
	if effect == nil {
		return worldpkg.StatusEffectLifetimeFields{}
	}
	return worldpkg.StatusEffectLifetimeFields{
		ExpiresAt:      &effect.ExpiresAt,
		StartMillis:    effect.Start,
		DurationMillis: &effect.Duration,
	}
}

func syncContractStatusVisual(cfg ContractBurningVisualHookConfig, instance *effectcontract.EffectInstance, effect *State, actor *ContractStatusActor) {
	if instance == nil || effect == nil {
		return
	}
	var actorPos *ActorPosition
	if actor != nil {
		actorPos = &ActorPosition{X: actor.X, Y: actor.Y}
	}
	SyncContractStatusVisualInstance(StatusVisualSyncConfig{
		Instance:         instance,
		Effect:           effect,
		Actor:            actorPos,
		TileSize:         cfg.TileSize,
		DefaultFootprint: cfg.DefaultFootprint,
	})
}

func recordContractStatusVisualSpawn(cfg ContractBurningVisualHookConfig, effectType string) {
	if cfg.RecordEffectSpawn == nil || effectType == "" {
		return
	}
	cfg.RecordEffectSpawn(effectType, "status-effect")
}

func durationToTicks(duration time.Duration, tickRate int) int {
	if duration <= 0 || tickRate <= 0 {
		return 0
	}
	ticks := int(math.Ceil(duration.Seconds() * float64(tickRate)))
	if ticks < 1 {
		ticks = 1
	}
	return ticks
}

type statusEffectVisualAdapter struct {
	state *State
}

func (a statusEffectVisualAdapter) SetStatusEffect(value string) {
	if a.state == nil {
		return
	}
	a.state.StatusEffect = StatusEffectType(value)
}

func (a statusEffectVisualAdapter) EffectState() any {
	if a.state == nil {
		return nil
	}
	return a.state
}
