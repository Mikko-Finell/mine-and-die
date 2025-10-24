package effects

import (
	"mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
)

// CloneEffectParams returns a deep copy of the provided effect parameter map.
func CloneEffectParams(params map[string]float64) map[string]float64 {
	return cloneFloatMap(params)
}

// SimEffectParamsPayloadFromTyped converts a typed effect-params payload into
// its simulation equivalent, cloning the mutable parameter map so callers
// receive an independent copy.
func SimEffectParamsPayloadFromTyped(payload simpatches.EffectParamsPayload) sim.EffectParamsPayload {
	return sim.EffectParamsPayload{Params: CloneEffectParams(payload.Params)}
}

// SimEffectParamsPayloadFromTypedPtr converts a typed effect-params payload
// pointer into its simulation equivalent. Nil pointers return nil.
func SimEffectParamsPayloadFromTypedPtr(payload *simpatches.EffectParamsPayload) *sim.EffectParamsPayload {
	if payload == nil {
		return nil
	}
	converted := SimEffectParamsPayloadFromTyped(*payload)
	return &converted
}

// TypedEffectParamsPayloadFromSim converts a simulation effect-params payload
// into its typed equivalent, cloning the mutable parameter map so callers
// receive an independent copy.
func TypedEffectParamsPayloadFromSim(payload sim.EffectParamsPayload) simpatches.EffectParamsPayload {
	return simpatches.EffectParamsPayload{Params: CloneEffectParams(payload.Params)}
}

// TypedEffectParamsPayloadFromSimPtr converts a simulation effect-params
// payload pointer into its typed equivalent. Nil pointers return nil.
func TypedEffectParamsPayloadFromSimPtr(payload *sim.EffectParamsPayload) *simpatches.EffectParamsPayload {
	if payload == nil {
		return nil
	}
	converted := TypedEffectParamsPayloadFromSim(*payload)
	return &converted
}
