package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// ContractProjectileLifecycleHookConfig bundles the world-facing dependencies
// required to keep contract-managed projectile instances synchronized with the
// legacy effect runtime. Callers provide lookups and callbacks so the hook can
// operate without importing server packages.
type ContractProjectileLifecycleHookConfig struct {
	TileSize          float64
	TickRate          int
	LookupTemplate    func(definitionID string) *ProjectileTemplate
	LookupOwner       func(actorID string) ProjectileOwner
	PruneExpired      func(time.Time)
	RecordEffectSpawn func(effectType, category string)
	AdvanceProjectile func(effect *State, now time.Time, dt float64) bool
}

// ContractProjectileLifecycleHook returns the spawn and tick handlers that keep
// contract-managed projectile instances in sync with the shared runtime state.
func ContractProjectileLifecycleHook(cfg ContractProjectileLifecycleHookConfig) HookSet {
	return HookSet{
		OnSpawn: func(rt Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
			if instance == nil {
				return
			}

			effect := LoadRuntimeEffect(rt, instance.ID)
			owner := lookupProjectileOwner(cfg.LookupOwner, instance.OwnerActorID)

			if effect != nil {
				SyncContractProjectileInstance(ContractProjectileSyncConfig{
					Instance: instance,
					Owner:    owner,
					Effect:   effect,
					TileSize: cfg.TileSize,
					TickRate: cfg.TickRate,
				})
				return
			}

			tpl := lookupProjectileTemplate(cfg.LookupTemplate, instance.DefinitionID)
			if tpl == nil || owner == nil {
				return
			}

			if cfg.PruneExpired != nil {
				cfg.PruneExpired(now)
			}

			effect = SpawnContractProjectileFromInstance(ProjectileSpawnConfig{
				Instance: instance,
				Owner:    owner,
				Template: tpl,
				Now:      now,
				TileSize: cfg.TileSize,
				TickRate: cfg.TickRate,
			})
			if effect == nil {
				return
			}
			if !RegisterRuntimeEffect(rt, effect) {
				instance.BehaviorState.TicksRemaining = 0
				return
			}

			if cfg.RecordEffectSpawn != nil {
				cfg.RecordEffectSpawn(tpl.Type, "projectile")
			}

			StoreRuntimeEffect(rt, instance.ID, effect)
			SyncContractProjectileInstance(ContractProjectileSyncConfig{
				Instance: instance,
				Owner:    owner,
				Effect:   effect,
				TileSize: cfg.TileSize,
				TickRate: cfg.TickRate,
			})
		},
		OnTick: func(rt Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
			if instance == nil {
				return
			}

			effect := LoadRuntimeEffect(rt, instance.ID)
			tpl := lookupProjectileTemplate(cfg.LookupTemplate, instance.DefinitionID)
			owner := lookupProjectileOwner(cfg.LookupOwner, instance.OwnerActorID)

			if effect == nil {
				if tpl == nil || owner == nil {
					return
				}
				if cfg.PruneExpired != nil {
					cfg.PruneExpired(now)
				}
				effect = SpawnContractProjectileFromInstance(ProjectileSpawnConfig{
					Instance: instance,
					Owner:    owner,
					Template: tpl,
					Now:      now,
					TileSize: cfg.TileSize,
					TickRate: cfg.TickRate,
				})
				if effect == nil {
					return
				}
				if !RegisterRuntimeEffect(rt, effect) {
					instance.BehaviorState.TicksRemaining = 0
					return
				}
				if cfg.RecordEffectSpawn != nil {
					cfg.RecordEffectSpawn(tpl.Type, "projectile")
				}
				StoreRuntimeEffect(rt, instance.ID, effect)
			}

			tickRate := cfg.TickRate
			if tickRate <= 0 {
				tickRate = defaultTickRate
			}
			dt := 1.0 / float64(tickRate)

			ended := false
			if cfg.AdvanceProjectile != nil {
				ended = cfg.AdvanceProjectile(effect, now, dt)
			}

			SyncContractProjectileInstance(ContractProjectileSyncConfig{
				Instance: instance,
				Owner:    owner,
				Effect:   effect,
				TileSize: cfg.TileSize,
				TickRate: cfg.TickRate,
			})

			if ended {
				instance.BehaviorState.TicksRemaining = 0
				UnregisterRuntimeEffect(rt, effect)
				StoreRuntimeEffect(rt, instance.ID, nil)
			}
		},
	}
}

func lookupProjectileOwner(lookup func(string) ProjectileOwner, actorID string) ProjectileOwner {
	if lookup == nil || actorID == "" {
		return nil
	}
	return lookup(actorID)
}

func lookupProjectileTemplate(lookup func(string) *ProjectileTemplate, definitionID string) *ProjectileTemplate {
	if lookup == nil || definitionID == "" {
		return nil
	}
	return lookup(definitionID)
}
