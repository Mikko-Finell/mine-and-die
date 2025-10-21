package world

import stats "mine-and-die/server/stats"

// StatsActor captures the dependencies required to resolve stat timers for an actor
// and propagate any derived max-health adjustments back to the caller.
type StatsActor struct {
	Component     *stats.Component
	SyncMaxHealth func(maxHealth float64)
}

// ResolveStats advances each actor's stat component for the given tick and applies
// any resulting max-health adjustments through the provided sync callbacks.
func ResolveStats(tick uint64, actors []StatsActor) {
	if len(actors) == 0 {
		return
	}

	for i := range actors {
		actor := actors[i]
		if actor.Component == nil {
			continue
		}

		actor.Component.Resolve(tick)
		SyncMaxHealth(actor.Component, actor.SyncMaxHealth)
	}
}

// SyncMaxHealth computes the derived max health and invokes the provided callback
// when a positive value is available.
func SyncMaxHealth(component *stats.Component, sync func(maxHealth float64)) {
	if component == nil || sync == nil {
		return
	}

	maxHealth := component.GetDerived(stats.DerivedMaxHealth)
	if maxHealth <= 0 {
		return
	}

	sync(maxHealth)
}
