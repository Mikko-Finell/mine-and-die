package world

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// MeleeHookConfig captures the dependencies required to construct the melee
// spawn hook. Callers may leave the config zero-valued to skip registration.
type MeleeHookConfig struct {
	TileSize        float64
	DefaultWidth    float64
	DefaultReach    float64
	DefaultDamage   float64
	DefaultDuration time.Duration

	LookupOwner   func(actorID string) *internaleffects.MeleeOwner
	ResolveImpact func(effect *worldeffects.State, owner *internaleffects.MeleeOwner, actorID string, tick effectcontract.Tick, now time.Time, area internaleffects.MeleeImpactArea)
}

// ProjectileHookConfig bundles callbacks required to reproduce the projectile
// lifecycle wiring for contract-managed fireballs. When left zero-valued the
// projectile hook is omitted.
type ProjectileHookConfig struct {
	TileSize float64
	TickRate int

	LookupTemplate    func(definitionID string) *internaleffects.ProjectileTemplate
	LookupOwner       func(actorID string) internaleffects.ProjectileOwner
	PruneExpired      func(at time.Time)
	RecordEffectSpawn func(effectType, category string)
	AdvanceProjectile func(effect *worldeffects.State, now time.Time, dt float64) bool
}

// BloodHookConfig carries the helpers required to ensure blood decal follow-up
// effects stay in sync with the legacy runtime behaviour. The hook is skipped
// when Ensure function data is unavailable.
type BloodHookConfig struct {
	TileSize        float64
	TickRate        int
	DefaultSize     float64
	DefaultDuration time.Duration
	Params          func() map[string]float64
	Colors          func() []string

	PruneExpired      func(at time.Time)
	RecordEffectSpawn func(effectType, category string)
}

// EffectManagerHooksConfig aggregates the optional hook configurations used to
// build the effect manager registry. Individual hooks are only registered when
// their configs provide the minimum required callbacks.
type EffectManagerHooksConfig struct {
	Melee      MeleeHookConfig
	Projectile ProjectileHookConfig
	Blood      BloodHookConfig
}

func BuildEffectManagerHooks(cfg EffectManagerHooksConfig) map[string]worldeffects.HookSet {
	return buildEffectManagerHooks(cfg)
}

func (w *World) effectManagerHooksConfig() EffectManagerHooksConfig {
	if w == nil {
		return EffectManagerHooksConfig{}
	}

	cfg := EffectManagerHooksConfig{}

	stateLookup := w.AbilityOwnerStateLookup()
	if stateLookup != nil {
		cfg.Melee.LookupOwner = func(actorID string) *internaleffects.MeleeOwner {
			state, _, ok := stateLookup(actorID)
			if !ok || state == nil {
				return nil
			}
			return &internaleffects.MeleeOwner{X: state.X, Y: state.Y, Reference: state}
		}
		cfg.Projectile.LookupOwner = func(actorID string) internaleffects.ProjectileOwner {
			state, _, ok := stateLookup(actorID)
			if !ok || state == nil {
				return nil
			}
			snapshot := abilityActorSnapshot(state)
			if snapshot == nil {
				return nil
			}
			return worldeffects.ProjectileOwnerSnapshot{X: snapshot.X, Y: snapshot.Y, FacingValue: snapshot.Facing}
		}
	}

	cfg.Projectile.RecordEffectSpawn = func(effectType, category string) {
		if w != nil {
			w.recordEffectSpawn(effectType, category)
		}
	}
	cfg.Blood.RecordEffectSpawn = func(effectType, category string) {
		if w != nil {
			w.recordEffectSpawn(effectType, category)
		}
	}

	return cfg
}

func buildEffectManagerHooks(cfg EffectManagerHooksConfig) map[string]worldeffects.HookSet {
	hooks := make(map[string]worldeffects.HookSet)

	if cfg.Melee.LookupOwner != nil && cfg.Melee.ResolveImpact != nil {
		hooks[effectcontract.HookMeleeSpawn] = internaleffects.MeleeSpawnHook(internaleffects.MeleeSpawnHookConfig{
			TileSize:        cfg.Melee.TileSize,
			DefaultWidth:    cfg.Melee.DefaultWidth,
			DefaultReach:    cfg.Melee.DefaultReach,
			DefaultDamage:   cfg.Melee.DefaultDamage,
			DefaultDuration: cfg.Melee.DefaultDuration,
			LookupOwner:     cfg.Melee.LookupOwner,
			ResolveImpact: func(effect *internaleffects.State, owner *internaleffects.MeleeOwner, actorID string, tick effectcontract.Tick, now time.Time, area internaleffects.MeleeImpactArea) {
				if cfg.Melee.ResolveImpact == nil {
					return
				}
				cfg.Melee.ResolveImpact((*worldeffects.State)(effect), owner, actorID, tick, now, area)
			},
		})
	}

	if cfg.Projectile.LookupTemplate != nil && cfg.Projectile.LookupOwner != nil && cfg.Projectile.AdvanceProjectile != nil {
		hooks[effectcontract.HookProjectileLifecycle] = internaleffects.ContractProjectileLifecycleHook(internaleffects.ContractProjectileLifecycleHookConfig{
			TileSize:          cfg.Projectile.TileSize,
			TickRate:          cfg.Projectile.TickRate,
			LookupTemplate:    cfg.Projectile.LookupTemplate,
			LookupOwner:       cfg.Projectile.LookupOwner,
			PruneExpired:      cfg.Projectile.PruneExpired,
			RecordEffectSpawn: cfg.Projectile.RecordEffectSpawn,
			AdvanceProjectile: func(effect *internaleffects.State, now time.Time, dt float64) bool {
				if cfg.Projectile.AdvanceProjectile == nil {
					return false
				}
				return cfg.Projectile.AdvanceProjectile((*worldeffects.State)(effect), now, dt)
			},
		})
	}

	if cfg.Blood.PruneExpired != nil && cfg.Blood.RecordEffectSpawn != nil {
		params := cfg.Blood.Params
		colors := cfg.Blood.Colors

		hooks[effectcontract.HookVisualBloodSplatter] = worldeffects.HookSet{
			OnSpawn: ensureBloodHook(cfg.Blood, params, colors),
			OnTick:  ensureBloodHook(cfg.Blood, params, colors),
		}
	}

	return hooks
}

func ensureBloodHook(cfg BloodHookConfig, params func() map[string]float64, colors func() []string) worldeffects.HookFunc {
	if cfg.PruneExpired == nil || cfg.RecordEffectSpawn == nil {
		return nil
	}

	return func(rt worldeffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
		if instance == nil {
			return
		}

		internaleffects.EnsureBloodDecalInstance(internaleffects.BloodDecalInstanceConfig{
			Runtime:         rt,
			Instance:        instance,
			Now:             now,
			TileSize:        cfg.TileSize,
			TickRate:        cfg.TickRate,
			DefaultSize:     cfg.DefaultSize,
			DefaultDuration: cfg.DefaultDuration,
			Params: func() map[string]float64 {
				if params == nil {
					return nil
				}
				return params()
			}(),
			Colors: func() []string {
				if colors == nil {
					return nil
				}
				return colors()
			}(),
			PruneExpired: cfg.PruneExpired,
			RecordSpawn:  cfg.RecordEffectSpawn,
		})
	}
}
