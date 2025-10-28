package world

import (
	"time"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// EffectManagerFacade exposes the legacy façade manager operations used during the
// transition. Implementations forward into the existing server effect manager so
// behaviour stays identical while the internal scaffolding lands.
type EffectManagerFacade interface {
	Definitions() map[string]*effectcontract.EffectDefinition
	Hooks() map[string]internaleffects.HookSet
	Instances() map[string]*effectcontract.EffectInstance
	Catalog() *effectcatalog.Resolver
	TotalEnqueued() int
	TotalDrained() int
	LastTickProcessed() effectcontract.Tick
	PendingIntentCount() int
	PendingIntents() []effectcontract.EffectIntent
	ResetPendingIntents()
	EnqueueIntent(effectcontract.EffectIntent)
	RunTick(effectcontract.Tick, time.Time, func(effectcontract.EffectLifecycleEvent))
	RuntimeEffect(string) *worldeffects.State
	Core() *worldeffects.Manager
}

// EffectManager orchestrates contract-managed effects for the internal world. The
// manager begins life with local bookkeeping so standalone constructor callers
// continue to work, and can bind to the legacy façade for behaviour parity when
// the legacy world is present.
type EffectManager struct {
	world  *World
	core   *worldeffects.Manager
	facade EffectManagerFacade
}

func newEffectManager(world *World) *EffectManager {
	manager := &EffectManager{world: world}
	if world != nil {
		manager.core = world.buildEffectManagerCore()
	}
	return manager
}

// BindLegacyEffectManager attaches the façade-backed manager so internal callers
// forward through the legacy wiring until the migration completes.
func (w *World) BindLegacyEffectManager(facade EffectManagerFacade) {
	if w == nil || w.effectManager == nil {
		return
	}
	w.effectManager.bindFacade(facade)
}

func (m *EffectManager) bindFacade(facade EffectManagerFacade) {
	if m == nil {
		return
	}
	m.facade = facade
	if facade != nil {
		if core := facade.Core(); core != nil {
			m.core = core
		}
	}
}

// Core exposes the underlying runtime manager, enabling callers to share the
// façade-owned core while the migration is in flight.
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
	if m.facade != nil {
		return m.facade.Definitions()
	}
	if m.core == nil {
		return nil
	}
	return m.core.Definitions()
}

// Hooks exposes the installed hook registry.
func (m *EffectManager) Hooks() map[string]internaleffects.HookSet {
	if m == nil {
		return nil
	}
	if m.facade != nil {
		return m.facade.Hooks()
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
	if m.facade != nil {
		return m.facade.Instances()
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
	if m.facade != nil {
		return m.facade.Catalog()
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
	if m.facade != nil {
		return m.facade.TotalEnqueued()
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
	if m.facade != nil {
		return m.facade.TotalDrained()
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
	if m.facade != nil {
		return m.facade.LastTickProcessed()
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
	if m.facade != nil {
		return m.facade.PendingIntentCount()
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
	if m.facade != nil {
		return m.facade.PendingIntents()
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
	if m.facade != nil {
		m.facade.ResetPendingIntents()
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
	if m.facade != nil {
		m.facade.EnqueueIntent(intent)
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
	if m.facade != nil {
		m.facade.RunTick(tick, now, emit)
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
	if m.facade != nil {
		return m.facade.RuntimeEffect(id)
	}
	if m.core == nil {
		return nil
	}
	return internaleffects.LoadRuntimeEffect(m.core, id)
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
		OwnerMissing: func(actorID string) bool {
			return w.effectOwnerMissing(actorID)
		},
		Registry: registryProvider,
	})
}
