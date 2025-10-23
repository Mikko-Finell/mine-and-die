package effects

import (
	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
)

// CloneEffectParams returns a deep copy of the provided effect parameter map.
func CloneEffectParams(params map[string]float64) map[string]float64 {
	return cloneFloatMap(params)
}

// SimEffectParamsPayloadFromLegacy converts a legacy effect-params payload into
// its simulation equivalent, cloning the mutable parameter map so callers
// receive an independent copy.
func SimEffectParamsPayloadFromLegacy(payload journal.EffectParamsPayload) sim.EffectParamsPayload {
	return sim.EffectParamsPayload{Params: CloneEffectParams(payload.Params)}
}

// SimEffectParamsPayloadFromLegacyPtr converts a legacy effect-params payload
// pointer into its simulation equivalent. Nil pointers return nil.
func SimEffectParamsPayloadFromLegacyPtr(payload *journal.EffectParamsPayload) *sim.EffectParamsPayload {
	if payload == nil {
		return nil
	}
	converted := SimEffectParamsPayloadFromLegacy(*payload)
	return &converted
}

// LegacyEffectParamsPayloadFromSim converts a simulation effect-params payload
// into its legacy equivalent, cloning the mutable parameter map so callers
// receive an independent copy.
func LegacyEffectParamsPayloadFromSim(payload sim.EffectParamsPayload) journal.EffectParamsPayload {
	return journal.EffectParamsPayload{Params: CloneEffectParams(payload.Params)}
}

// LegacyEffectParamsPayloadFromSimPtr converts a simulation effect-params
// payload pointer into its legacy equivalent. Nil pointers return nil.
func LegacyEffectParamsPayloadFromSimPtr(payload *sim.EffectParamsPayload) *journal.EffectParamsPayload {
	if payload == nil {
		return nil
	}
	converted := LegacyEffectParamsPayloadFromSim(*payload)
	return &converted
}
