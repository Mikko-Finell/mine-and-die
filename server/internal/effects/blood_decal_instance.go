package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// BloodDecalInstanceConfig wires the dependencies required to mirror the
// legacy blood decal lifecycle when synchronising contract-managed instances.
type BloodDecalInstanceConfig struct {
	Runtime         Runtime
	Instance        *effectcontract.EffectInstance
	Now             time.Time
	TileSize        float64
	TickRate        int
	DefaultSize     float64
	DefaultDuration time.Duration
	Params          map[string]float64
	Colors          []string
	PruneExpired    func(time.Time)
	RecordSpawn     func(effectType, producer string)
}

// EnsureBloodDecalInstance reproduces the historical ensure helper that kept
// contract-managed blood decals registered with the world registry and in sync
// with the instance payload. The helper hides runtime lookups and registry
// wiring so callers only provide the dependencies they already own.
func EnsureBloodDecalInstance(cfg BloodDecalInstanceConfig) *State {
	instance := cfg.Instance
	if instance == nil {
		return nil
	}

	effect := runtimeEffect(cfg.Runtime, instance.ID)
	if effect == nil {
		if cfg.PruneExpired != nil {
			cfg.PruneExpired(cfg.Now)
		}
		effect = SpawnContractBloodDecalFromInstance(BloodDecalSpawnConfig{
			Instance:        instance,
			Now:             cfg.Now,
			TileSize:        cfg.TileSize,
			TickRate:        cfg.TickRate,
			DefaultSize:     cfg.DefaultSize,
			DefaultDuration: cfg.DefaultDuration,
			Params:          cfg.Params,
			Colors:          cfg.Colors,
		})
		if effect == nil {
			return nil
		}
		registry := Registry{}
		if cfg.Runtime != nil {
			registry = cfg.Runtime.Registry()
		}
		if !RegisterEffect(registry, effect) {
			instance.BehaviorState.TicksRemaining = 0
			return nil
		}
		storeRuntimeEffect(cfg.Runtime, instance.ID, effect)
		if cfg.RecordSpawn != nil {
			cfg.RecordSpawn(effect.Type, "blood-decal")
		}
	}

	SyncContractBloodDecalInstance(BloodDecalSyncConfig{
		Instance:    instance,
		Effect:      effect,
		TileSize:    cfg.TileSize,
		DefaultSize: cfg.DefaultSize,
		Colors:      cfg.Colors,
	})
	return effect
}

func runtimeEffect(rt Runtime, id string) *State {
	if id == "" || rt == nil {
		return nil
	}
	value := rt.InstanceState(id)
	if value == nil {
		return nil
	}
	effect, _ := value.(*State)
	return effect
}

func storeRuntimeEffect(rt Runtime, id string, effect *State) {
	if rt == nil || id == "" {
		return
	}
	if effect == nil {
		rt.ClearInstanceState(id)
		return
	}
	rt.SetInstanceState(id, effect)
}
