package server

import (
	effectcontract "mine-and-die/server/effects/contract"
	journalpkg "mine-and-die/server/internal/journal"
)

// AppendPatch records a patch for the current tick in the world journal.
func (w *World) AppendPatch(p Patch) {
	if w == nil {
		return
	}
	w.journal.AppendPatch(p)
}

// PurgeEntity drops staged patches referencing the provided entity ID.
func (w *World) PurgeEntity(entityID string) {
	if w == nil {
		return
	}
	w.journal.PurgeEntity(entityID)
}

// DrainPatches returns all staged patches from the journal and clears them.
func (w *World) DrainPatches() []Patch {
	if w == nil {
		return nil
	}
	return w.journal.DrainPatches()
}

// SnapshotPatches returns a copy of the staged patches without clearing them.
func (w *World) SnapshotPatches() []Patch {
	if w == nil {
		return nil
	}
	return w.journal.SnapshotPatches()
}

// RestorePatches reinserts drained patches back into the journal.
func (w *World) RestorePatches(patches []Patch) {
	if w == nil || len(patches) == 0 {
		return
	}
	w.journal.RestorePatches(patches)
}

// RecordEffectSpawn journals an effect spawn envelope and returns the stored copy.
func (w *World) RecordEffectSpawn(event effectcontract.EffectSpawnEvent) effectcontract.EffectSpawnEvent {
	if w == nil {
		return effectcontract.EffectSpawnEvent{}
	}
	return w.journal.RecordEffectSpawn(event)
}

// RecordEffectUpdate journals an effect update envelope and returns the stored copy.
func (w *World) RecordEffectUpdate(event effectcontract.EffectUpdateEvent) effectcontract.EffectUpdateEvent {
	if w == nil {
		return effectcontract.EffectUpdateEvent{}
	}
	return w.journal.RecordEffectUpdate(event)
}

// RecordEffectEnd journals an effect end envelope and returns the stored copy.
func (w *World) RecordEffectEnd(event effectcontract.EffectEndEvent) effectcontract.EffectEndEvent {
	if w == nil {
		return effectcontract.EffectEndEvent{}
	}
	return w.journal.RecordEffectEnd(event)
}

// DrainEffectEvents returns the staged effect lifecycle batch and clears it.
func (w *World) DrainEffectEvents() journalpkg.EffectEventBatch {
	if w == nil {
		return journalpkg.EffectEventBatch{}
	}
	return w.journal.DrainEffectEvents()
}

// SnapshotEffectEvents returns a copy of the staged effect lifecycle batch.
func (w *World) SnapshotEffectEvents() journalpkg.EffectEventBatch {
	if w == nil {
		return journalpkg.EffectEventBatch{}
	}
	return w.journal.SnapshotEffectEvents()
}

// RestoreEffectEvents reinserts a drained lifecycle batch back into the journal.
func (w *World) RestoreEffectEvents(batch journalpkg.EffectEventBatch) {
	if w == nil {
		return
	}
	w.journal.RestoreEffectEvents(batch)
}

// ConsumeResyncHint reports whether the journal observed a resync-worthy pattern.
func (w *World) ConsumeResyncHint() (journalpkg.ResyncSignal, bool) {
	if w == nil {
		return journalpkg.ResyncSignal{}, false
	}
	return w.journal.ConsumeResyncHint()
}

// RecordKeyframe stores a keyframe in the journal enforcing retention limits.
func (w *World) RecordKeyframe(frame keyframe) keyframeRecordResult {
	if w == nil {
		return keyframeRecordResult{}
	}
	return w.journal.RecordKeyframe(frame)
}

// KeyframeBySequence looks up a keyframe by sequence number.
func (w *World) KeyframeBySequence(sequence uint64) (keyframe, bool) {
	if w == nil {
		return keyframe{}, false
	}
	return w.journal.KeyframeBySequence(sequence)
}

// KeyframeWindow reports the current keyframe buffer size and bounds.
func (w *World) KeyframeWindow() (int, uint64, uint64) {
	if w == nil {
		return 0, 0, 0
	}
	return w.journal.KeyframeWindow()
}

// AttachJournalTelemetry wires telemetry counters into the journal.
func (w *World) AttachJournalTelemetry(t journalpkg.Telemetry) {
	if w == nil {
		return
	}
	w.journal.AttachTelemetry(t)
}

// SwapJournal replaces the journal backing the world, returning the previous instance.
func (w *World) SwapJournal(j Journal) (previous Journal) {
	if w == nil {
		return Journal{}
	}
	previous = w.journal
	w.journal = j
	return previous
}
