package world

import (
	"time"

	internaleffects "mine-and-die/server/internal/effects"
)

// EffectTrigger mirrors the legacy trigger payload for fire-and-forget visuals.
type EffectTrigger = internaleffects.Trigger

// QueueEffectTrigger appends a one-shot visual trigger to the staged batch.
func (w *World) QueueEffectTrigger(trigger EffectTrigger, now time.Time) EffectTrigger {
	if w == nil {
		return EffectTrigger{}
	}
	if trigger.Type == "" {
		return EffectTrigger{}
	}

	if trigger.ID == "" {
		trigger.ID = w.allocateEffectID()
	}
	if trigger.Start == 0 {
		trigger.Start = now.UnixMilli()
	}

	w.effectTriggers = append(w.effectTriggers, trigger)
	w.recordEffectTrigger(trigger.Type)
	return trigger
}

// DrainEffectTriggers returns the queued effect triggers and clears the batch.
func (w *World) DrainEffectTriggers() []EffectTrigger {
	if w == nil || len(w.effectTriggers) == 0 {
		return nil
	}

	drained := make([]EffectTrigger, len(w.effectTriggers))
	copy(drained, w.effectTriggers)
	for i := range w.effectTriggers {
		w.effectTriggers[i] = EffectTrigger{}
	}
	w.effectTriggers = w.effectTriggers[:0]
	return drained
}

// SnapshotEffectTriggers returns a copy of the staged trigger batch.
func (w *World) SnapshotEffectTriggers() []EffectTrigger {
	if w == nil || len(w.effectTriggers) == 0 {
		return nil
	}

	snapshot := make([]EffectTrigger, len(w.effectTriggers))
	copy(snapshot, w.effectTriggers)
	return snapshot
}
