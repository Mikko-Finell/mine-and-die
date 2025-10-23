package effects

import "mine-and-die/server/internal/sim"

// CloneEffectTriggers returns a deep copy of the provided effect trigger slice.
func CloneEffectTriggers(triggers []sim.EffectTrigger) []sim.EffectTrigger {
	if len(triggers) == 0 {
		return nil
	}
	cloned := make([]sim.EffectTrigger, len(triggers))
	for i, trigger := range triggers {
		cloned[i] = CloneEffectTrigger(trigger)
	}
	return cloned
}

// CloneEffectTrigger returns a deep copy of the provided effect trigger.
func CloneEffectTrigger(trigger sim.EffectTrigger) sim.EffectTrigger {
	cloned := trigger
	cloned.Params = cloneFloatMap(trigger.Params)
	cloned.Colors = cloneStringSlice(trigger.Colors)
	return cloned
}
