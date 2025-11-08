package world

import (
	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// EffectTelemetry records effect lifecycle telemetry for the world.
type EffectTelemetry interface {
	RecordEffectSpawned(effectType, producer string)
	RecordEffectUpdated(effectType, mutation string)
	RecordEffectEnded(effectType, reason string)
	RecordEffectTrigger(triggerType string)
	RecordEffectParity(summary EffectTelemetrySummary)
	RecordEffectSpatialOverflow(effectType string)
}

// EffectTelemetrySummary aggregates effect lifecycle metrics for parity checks.
type EffectTelemetrySummary struct {
	EffectType    string
	Hits          int
	UniqueVictims int
	TotalDamage   float64
	SpawnTick     effectcontract.Tick
	FirstHitTick  effectcontract.Tick
}

func (w *World) AttachEffectTelemetry(t EffectTelemetry) {
	if w == nil {
		return
	}
	w.effectTelemetry = t
	registry := w.EffectRegistry()
	if t != nil {
		registry.RecordSpatialOverflow = t.RecordEffectSpatialOverflow
	} else {
		registry.RecordSpatialOverflow = nil
	}
	w.effectsRegistry = registry
}

func (w *World) recordEffectSpawn(effectType, producer string) {
	if w == nil || w.effectTelemetry == nil {
		return
	}
	w.effectTelemetry.RecordEffectSpawned(effectType, producer)
}

func (w *World) recordEffectUpdate(eff *worldeffects.State, mutation string) {
	if w == nil || eff == nil || w.effectTelemetry == nil {
		return
	}
	w.effectTelemetry.RecordEffectUpdated(eff.Type, mutation)
}

func (w *World) recordEffectEnd(eff *worldeffects.State, reason string) {
	if w == nil || eff == nil {
		return
	}
	if !eff.TelemetryEnded {
		w.flushEffectTelemetry(eff)
		eff.TelemetryEnded = true
	}
	if w.effectTelemetry != nil {
		w.effectTelemetry.RecordEffectEnded(eff.Type, reason)
	}
}

func (w *World) recordEffectTrigger(triggerType string) {
	if w == nil || w.effectTelemetry == nil {
		return
	}
	w.effectTelemetry.RecordEffectTrigger(triggerType)
}

func (w *World) recordEffectHitTelemetry(eff *worldeffects.State, targetID string, delta float64) {
	if w == nil || eff == nil {
		return
	}
	if eff.TelemetrySpawnTick == 0 {
		eff.TelemetrySpawnTick = effectcontract.Tick(int64(w.currentTick))
	}
	if eff.TelemetryFirstHitTick == 0 {
		eff.TelemetryFirstHitTick = effectcontract.Tick(int64(w.currentTick))
	}
	eff.TelemetryHitCount++
	if eff.TelemetryVictims == nil {
		eff.TelemetryVictims = make(map[string]struct{})
	}
	if targetID != "" {
		eff.TelemetryVictims[targetID] = struct{}{}
	}
	if delta < 0 {
		eff.TelemetryDamage += -delta
	}
}

func (w *World) flushEffectTelemetry(eff *worldeffects.State) {
	if w == nil || eff == nil || w.effectTelemetry == nil {
		return
	}
	victims := 0
	if len(eff.TelemetryVictims) > 0 {
		victims = len(eff.TelemetryVictims)
	}
	spawnTick := eff.TelemetrySpawnTick
	if spawnTick == 0 {
		spawnTick = effectcontract.Tick(int64(w.currentTick))
	}
	summary := EffectTelemetrySummary{
		EffectType:    eff.Type,
		Hits:          eff.TelemetryHitCount,
		UniqueVictims: victims,
		TotalDamage:   eff.TelemetryDamage,
		SpawnTick:     spawnTick,
		FirstHitTick:  eff.TelemetryFirstHitTick,
	}
	w.effectTelemetry.RecordEffectParity(summary)
	eff.TelemetryHitCount = 0
	eff.TelemetryDamage = 0
	eff.TelemetryVictims = nil
	eff.TelemetryFirstHitTick = 0
}
