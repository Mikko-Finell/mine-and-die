package world

import (
	"time"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// EffectManager orchestrates contract-managed effects for the internal world.
// The manager owns its runtime core directly so constructor callers can operate
// without relying on the legacy façade wiring.
type EffectManager struct {
	world *World
	core  *worldeffects.Manager
}

func newEffectManager(world *World) *EffectManager {
	manager := &EffectManager{world: world}
	if world != nil {
		manager.core = world.buildEffectManagerCore()
	}
	return manager
}

// Core exposes the underlying runtime manager, enabling callers to share the
// runtime state with parity checks.
func (m *EffectManager) Core() *worldeffects.Manager {
	if m == nil {
		return nil
	}
	return m.core
}

// Definitions returns the known effect definitions.
func (m *EffectManager) Definitions() map[string]*effectcontract.EffectDefinition {
	if m == nil {
		return nil
	}
	if m.core == nil {
		return nil
	}
	return m.core.Definitions()
}

// Hooks exposes the installed hook registry.
func (m *EffectManager) Hooks() map[string]worldeffects.HookSet {
	if m == nil {
		return nil
	}
	if m.core == nil {
		return nil
	}
	return m.core.Hooks()
}

// Instances returns the active contract instances keyed by ID.
func (m *EffectManager) Instances() map[string]*effectcontract.EffectInstance {
	if m == nil {
		return nil
	}
	if m.core == nil {
		return nil
	}
	return m.core.Instances()
}

// Catalog returns the loaded effect catalog resolver.
func (m *EffectManager) Catalog() *effectcatalog.Resolver {
	if m == nil {
		return nil
	}
	if m.core == nil {
		return nil
	}
	return m.core.Catalog()
}

// TotalEnqueued reports the number of intents enqueued since construction.
func (m *EffectManager) TotalEnqueued() int {
	if m == nil {
		return 0
	}
	if m.core == nil {
		return 0
	}
	return m.core.TotalEnqueued()
}

// TotalDrained reports the number of intents drained since construction.
func (m *EffectManager) TotalDrained() int {
	if m == nil {
		return 0
	}
	if m.core == nil {
		return 0
	}
	return m.core.TotalDrained()
}

// LastTickProcessed reports the most recent tick processed by the manager.
func (m *EffectManager) LastTickProcessed() effectcontract.Tick {
	if m == nil {
		return 0
	}
	if m.core == nil {
		return 0
	}
	return m.core.LastTickProcessed()
}

// PendingIntentCount returns the number of queued intents awaiting processing.
func (m *EffectManager) PendingIntentCount() int {
	if m == nil {
		return 0
	}
	if m.core == nil {
		return 0
	}
	return m.core.PendingIntentCount()
}

// PendingIntents exposes the queued intents awaiting processing.
func (m *EffectManager) PendingIntents() []effectcontract.EffectIntent {
	if m == nil {
		return nil
	}
	if m.core == nil {
		return nil
	}
	return m.core.PendingIntents()
}

// ResetPendingIntents clears any staged intents.
func (m *EffectManager) ResetPendingIntents() {
	if m == nil {
		return
	}
	if m.core == nil {
		return
	}
	m.core.ResetPendingIntents()
}

// EnqueueIntent adds an intent to the pending queue.
func (m *EffectManager) EnqueueIntent(intent effectcontract.EffectIntent) {
	if m == nil {
		return
	}
	if m.core == nil {
		return
	}
	m.core.EnqueueIntent(intent)
}

// RunTick advances the manager by one tick.
func (m *EffectManager) RunTick(tick effectcontract.Tick, now time.Time, emit func(effectcontract.EffectLifecycleEvent)) {
	if m == nil {
		return
	}
	if m.core == nil {
		return
	}
	m.core.RunTick(tick, now, emit)
}

// RuntimeEffect looks up the runtime state for the provided identifier.
func (m *EffectManager) RuntimeEffect(id string) *worldeffects.State {
	if m == nil || id == "" {
		return nil
	}
	if m.core == nil {
		return nil
	}
	return m.core.RuntimeEffect(id)
}

func (w *World) buildEffectManagerCore() *worldeffects.Manager {
	if w == nil {
		return nil
	}

	definitions := effectcontract.BuiltInDefinitions()

	var resolver *effectcatalog.Resolver
	if loaded, err := effectcatalog.Load(effectcontract.BuiltInRegistry, effectcatalog.DefaultPaths()...); err == nil {
		if defs := loaded.DefinitionsByContractID(); len(defs) > 0 {
			for id, def := range defs {
				if _, exists := definitions[id]; exists {
					continue
				}
				definitions[id] = def
			}
		}
		resolver = loaded
	}

	registryProvider := func() worldeffects.Registry {
		registry := w.EffectRegistry()
		return worldeffects.Registry(registry)
	}

	return worldeffects.NewManager(worldeffects.ManagerConfig{
		Definitions: definitions,
		Catalog:     resolver,
		Hooks:       w.effectManagerHooks(),
		OwnerMissing: func(actorID string) bool {
			return w.effectOwnerMissing(actorID)
		},
		Registry: registryProvider,
	})
}

func (w *World) effectManagerHooks() map[string]worldeffects.HookSet {
	if w == nil {
		return nil
	}
	return buildEffectManagerHooks(w.effectManagerHooksConfig())
}

func (w *World) effectManagerHooksConfig() EffectManagerHooksConfig {
	if w == nil {
		return EffectManagerHooksConfig{}
	}

	cfg := EffectManagerHooksConfig{}

	cfg.Projectile.RecordEffectSpawn = func(effectType, category string) {
		w.recordEffectSpawn(effectType, category)
	}

	cfg.Blood.RecordEffectSpawn = func(effectType, category string) {
		w.recordEffectSpawn(effectType, category)
	}

	return cfg
}
