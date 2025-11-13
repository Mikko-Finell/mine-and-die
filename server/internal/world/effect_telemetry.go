package world

import (
	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// EffectTelemetry exposes the telemetry counters consumed by effect emitters.
// Callers provide the legacy metrics implementation so the helpers can mirror
// historical behaviour while running inside the internal world package.
type EffectTelemetry interface {
	RecordEffectSpawned(effectType, producer string)
	RecordEffectUpdated(effectType, mutation string)
	RecordEffectEnded(effectType, reason string)
	RecordEffectTrigger(triggerType string)
	RecordEffectParity(summary EffectTelemetrySummary)
}

// EffectTelemetrySummary captures the parity counters emitted when an effect
// finishes processing. The struct mirrors the legacy telemetry payload so the
// internal helpers can reuse the existing aggregators without translation.
type EffectTelemetrySummary struct {
	EffectType    string
	Hits          int
	UniqueVictims int
	TotalDamage   float64
	SpawnTick     effectcontract.Tick
	FirstHitTick  effectcontract.Tick
}

// RecordEffectSpawnTelemetry increments the spawn counter for the provided
// effect type and producer category.
func RecordEffectSpawnTelemetry(telemetry EffectTelemetry, effectType, producer string) {
	if telemetry == nil || effectType == "" {
		return
	}
	telemetry.RecordEffectSpawned(effectType, producer)
}

// RecordEffectUpdateTelemetry increments the update counter for the supplied
// effect type and mutation label.
func RecordEffectUpdateTelemetry(telemetry EffectTelemetry, effectType, mutation string) {
	if telemetry == nil || effectType == "" {
		return
	}
	telemetry.RecordEffectUpdated(effectType, mutation)
}

// RecordEffectTriggerTelemetry increments the trigger counter for the provided
// trigger type.
func RecordEffectTriggerTelemetry(telemetry EffectTelemetry, triggerType string) {
	if telemetry == nil || triggerType == "" {
		return
	}
	telemetry.RecordEffectTrigger(triggerType)
}

// RecordEffectHitTelemetry updates the in-memory telemetry bookkeeping for the
// provided effect when it strikes a target.
func RecordEffectHitTelemetry(effect *worldeffects.State, targetID string, delta float64, tick effectcontract.Tick) {
	if effect == nil {
		return
	}
	if effect.TelemetrySpawnTick == 0 {
		effect.TelemetrySpawnTick = tick
	}
	if effect.TelemetryFirstHitTick == 0 {
		effect.TelemetryFirstHitTick = tick
	}
	effect.TelemetryHitCount++
	if targetID != "" {
		if effect.TelemetryVictims == nil {
			effect.TelemetryVictims = make(map[string]struct{})
		}
		effect.TelemetryVictims[targetID] = struct{}{}
	}
	if delta < 0 {
		effect.TelemetryDamage += -delta
	}
}

// RecordEffectHitTelemetry updates telemetry counters for the provided effect.
func (w *World) RecordEffectHitTelemetry(effect *worldeffects.State, targetID string, delta float64) {
	if w == nil {
		return
	}
	RecordEffectHitTelemetry(effect, targetID, delta, effectcontract.Tick(int64(w.currentTick())))
}

// FlushEffectTelemetry emits the accumulated parity counters for the provided
// effect and resets its bookkeeping to mirror the legacy lifecycle handling.
func FlushEffectTelemetry(telemetry EffectTelemetry, effect *worldeffects.State, tick effectcontract.Tick) {
	if telemetry == nil || effect == nil {
		return
	}

	victims := len(effect.TelemetryVictims)
	spawn := effect.TelemetrySpawnTick
	if spawn == 0 {
		spawn = tick
	}

	summary := EffectTelemetrySummary{
		EffectType:    effect.Type,
		Hits:          effect.TelemetryHitCount,
		UniqueVictims: victims,
		TotalDamage:   effect.TelemetryDamage,
		SpawnTick:     spawn,
		FirstHitTick:  effect.TelemetryFirstHitTick,
	}
	telemetry.RecordEffectParity(summary)

	effect.TelemetryHitCount = 0
	effect.TelemetryDamage = 0
	effect.TelemetryVictims = nil
	effect.TelemetryFirstHitTick = 0
}

// RecordEffectEndTelemetry finalises telemetry bookkeeping for the provided
// effect and records the end reason in the supplied counters.
func RecordEffectEndTelemetry(telemetry EffectTelemetry, effect *worldeffects.State, reason string, tick effectcontract.Tick) {
	if effect == nil {
		return
	}
	if !effect.TelemetryEnded {
		FlushEffectTelemetry(telemetry, effect, tick)
		effect.TelemetryEnded = true
	}
	if telemetry != nil {
		telemetry.RecordEffectEnded(effect.Type, reason)
	}
}
