package world

import (
	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// EffectTelemetry captures the telemetry counters consumed by effect emitter
// helpers. Implementations mirror the legacy telemetry adapter so the internal
// world can reproduce spawn/update/end parity without importing the server
// package.
type EffectTelemetry interface {
	RecordEffectSpawned(effectType, producer string)
	RecordEffectUpdated(effectType, mutation string)
	RecordEffectEnded(effectType, reason string)
	RecordEffectTrigger(triggerType string)
	RecordEffectParity(summary EffectParitySummary)
}

// EffectParitySummary aggregates hit/miss statistics for an effect instance so
// telemetry counters can mirror the legacy parity metrics.
type EffectParitySummary struct {
	EffectType    string
	Hits          int
	UniqueVictims int
	TotalDamage   float64
	SpawnTick     effectcontract.Tick
	FirstHitTick  effectcontract.Tick
}

// EffectTelemetryEmitters coordinates telemetry emission for effect lifecycle
// events. Callers populate the telemetry implementation and an optional current
// tick provider to keep effect state parity with the legacy world.
type EffectTelemetryEmitters struct {
	Telemetry   EffectTelemetry
	CurrentTick func() effectcontract.Tick
}

// RecordEffectSpawn records a spawn event for the given effect type and
// producer category.
func (e EffectTelemetryEmitters) RecordEffectSpawn(effectType, producer string) {
	if e.Telemetry == nil {
		return
	}
	e.Telemetry.RecordEffectSpawned(effectType, producer)
}

// RecordEffectUpdate records an update event for the provided effect instance.
func (e EffectTelemetryEmitters) RecordEffectUpdate(effect *worldeffects.State, mutation string) {
	if e.Telemetry == nil || effect == nil {
		return
	}
	e.Telemetry.RecordEffectUpdated(effect.Type, mutation)
}

// RecordEffectEnd records the termination reason for the effect, flushing any
// pending parity statistics before emitting the end event.
func (e EffectTelemetryEmitters) RecordEffectEnd(effect *worldeffects.State, reason string) {
	if effect == nil {
		return
	}
	if !effect.TelemetryEnded {
		e.FlushEffect(effect)
		effect.TelemetryEnded = true
	}
	if e.Telemetry != nil {
		e.Telemetry.RecordEffectEnded(effect.Type, reason)
	}
}

// RecordEffectTrigger records a fire-and-forget trigger event.
func (e EffectTelemetryEmitters) RecordEffectTrigger(triggerType string) {
	if e.Telemetry == nil {
		return
	}
	e.Telemetry.RecordEffectTrigger(triggerType)
}

// RecordEffectHit updates hit telemetry counters for the provided effect and
// victim.
func (e EffectTelemetryEmitters) RecordEffectHit(effect *worldeffects.State, targetID string, delta float64) {
	if effect == nil {
		return
	}

	tick := e.currentTick()
	if effect.TelemetrySpawnTick == 0 {
		effect.TelemetrySpawnTick = tick
	}
	if effect.TelemetryFirstHitTick == 0 {
		effect.TelemetryFirstHitTick = tick
	}

	effect.TelemetryHitCount++
	if effect.TelemetryVictims == nil {
		effect.TelemetryVictims = make(map[string]struct{})
	}
	if targetID != "" {
		effect.TelemetryVictims[targetID] = struct{}{}
	}
	if delta < 0 {
		effect.TelemetryDamage += -delta
	}
}

// FlushEffect emits the accumulated parity summary for the effect and resets
// the per-effect counters.
func (e EffectTelemetryEmitters) FlushEffect(effect *worldeffects.State) {
	if effect == nil || e.Telemetry == nil {
		return
	}

	victims := 0
	if len(effect.TelemetryVictims) > 0 {
		victims = len(effect.TelemetryVictims)
	}

	spawnTick := effect.TelemetrySpawnTick
	if spawnTick == 0 {
		spawnTick = e.currentTick()
	}

	summary := EffectParitySummary{
		EffectType:    effect.Type,
		Hits:          effect.TelemetryHitCount,
		UniqueVictims: victims,
		TotalDamage:   effect.TelemetryDamage,
		SpawnTick:     spawnTick,
		FirstHitTick:  effect.TelemetryFirstHitTick,
	}
	e.Telemetry.RecordEffectParity(summary)

	effect.TelemetryHitCount = 0
	effect.TelemetryDamage = 0
	effect.TelemetryVictims = nil
	effect.TelemetryFirstHitTick = 0
}

func (e EffectTelemetryEmitters) currentTick() effectcontract.Tick {
	if e.CurrentTick == nil {
		return 0
	}
	return e.CurrentTick()
}
