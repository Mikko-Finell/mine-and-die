package effects

import (
	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
)

// SimEffectEventBatchFromLegacy converts a legacy effect event batch into its
// simulation equivalent, cloning the underlying slices and maps so callers
// receive independent copies.
func SimEffectEventBatchFromLegacy(batch journal.EffectEventBatch) sim.EffectEventBatch {
	return sim.EffectEventBatch{
		Spawns:      journal.CloneEffectSpawnEvents(batch.Spawns),
		Updates:     journal.CloneEffectUpdateEvents(batch.Updates),
		Ends:        journal.CloneEffectEndEvents(batch.Ends),
		LastSeqByID: journal.CopySeqMap(batch.LastSeqByID),
	}
}

// LegacyEffectEventBatchFromSim converts a simulation effect event batch into
// its legacy equivalent, cloning the underlying slices and maps so callers
// receive independent copies.
func LegacyEffectEventBatchFromSim(batch sim.EffectEventBatch) journal.EffectEventBatch {
	return journal.EffectEventBatch{
		Spawns:      journal.CloneEffectSpawnEvents(batch.Spawns),
		Updates:     journal.CloneEffectUpdateEvents(batch.Updates),
		Ends:        journal.CloneEffectEndEvents(batch.Ends),
		LastSeqByID: journal.CopySeqMap(batch.LastSeqByID),
	}
}

// SimEffectResyncSignalFromLegacy converts a legacy resync signal into its
// simulation equivalent, cloning the nested reasons slice.
func SimEffectResyncSignalFromLegacy(signal journal.ResyncSignal) sim.EffectResyncSignal {
	return sim.EffectResyncSignal{
		LostSpawns:  signal.LostSpawns,
		TotalEvents: signal.TotalEvents,
		Reasons:     SimResyncReasonsFromLegacy(signal.Reasons),
	}
}

// LegacyEffectResyncSignalFromSim converts a simulation resync signal into its
// legacy equivalent, cloning the nested reasons slice.
func LegacyEffectResyncSignalFromSim(signal sim.EffectResyncSignal) journal.ResyncSignal {
	return journal.ResyncSignal{
		LostSpawns:  signal.LostSpawns,
		TotalEvents: signal.TotalEvents,
		Reasons:     LegacyResyncReasonsFromSim(signal.Reasons),
	}
}

// SimResyncReasonsFromLegacy converts legacy resync reasons into their
// simulation equivalents, cloning the slice to avoid sharing state.
func SimResyncReasonsFromLegacy(reasons []journal.ResyncReason) []sim.EffectResyncReason {
	if len(reasons) == 0 {
		return nil
	}
	converted := make([]sim.EffectResyncReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = sim.EffectResyncReason{
			Kind:     reason.Kind,
			EffectID: reason.EffectID,
		}
	}
	return converted
}

// LegacyResyncReasonsFromSim converts simulation resync reasons into their
// legacy equivalents, cloning the slice to avoid sharing state.
func LegacyResyncReasonsFromSim(reasons []sim.EffectResyncReason) []journal.ResyncReason {
	if len(reasons) == 0 {
		return nil
	}
	converted := make([]journal.ResyncReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = journal.ResyncReason{
			Kind:     reason.Kind,
			EffectID: reason.EffectID,
		}
	}
	return converted
}
