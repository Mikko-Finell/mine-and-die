package effects

import (
	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
)

// SimEffectEventBatchFromTyped converts a typed effect event batch into its
// simulation equivalent, cloning the underlying slices and maps so callers
// receive independent copies.
func SimEffectEventBatchFromTyped(batch simpatches.EffectEventBatch) sim.EffectEventBatch {
	return sim.EffectEventBatch{
		Spawns:      cloneTypedEffectSpawnEvents(batch.Spawns),
		Updates:     cloneTypedEffectUpdateEvents(batch.Updates),
		Ends:        cloneTypedEffectEndEvents(batch.Ends),
		LastSeqByID: copyEffectSeqMap(batch.LastSeqByID),
	}
}

// TypedEffectEventBatchFromSim converts a simulation effect event batch into
// its typed equivalent, cloning the underlying slices and maps so callers
// receive independent copies.
func TypedEffectEventBatchFromSim(batch sim.EffectEventBatch) simpatches.EffectEventBatch {
	return simpatches.EffectEventBatch{
		Spawns:      cloneTypedEffectSpawnEvents(batch.Spawns),
		Updates:     cloneTypedEffectUpdateEvents(batch.Updates),
		Ends:        cloneTypedEffectEndEvents(batch.Ends),
		LastSeqByID: copyEffectSeqMap(batch.LastSeqByID),
	}
}

// SimEffectResyncSignalFromTyped converts a typed resync signal into its
// simulation equivalent, cloning the nested reasons slice.
func SimEffectResyncSignalFromTyped(signal simpatches.EffectResyncSignal) sim.EffectResyncSignal {
	return sim.EffectResyncSignal{
		LostSpawns:  signal.LostSpawns,
		TotalEvents: signal.TotalEvents,
		Reasons:     simResyncReasonsFromTyped(signal.Reasons),
	}
}

// TypedEffectResyncSignalFromSim converts a simulation resync signal into its
// typed equivalent, cloning the nested reasons slice.
func TypedEffectResyncSignalFromSim(signal sim.EffectResyncSignal) simpatches.EffectResyncSignal {
	return simpatches.EffectResyncSignal{
		LostSpawns:  signal.LostSpawns,
		TotalEvents: signal.TotalEvents,
		Reasons:     typedResyncReasonsFromSim(signal.Reasons),
	}
}

// simResyncReasonsFromTyped converts typed resync reasons into their
// simulation equivalents, cloning the slice to avoid sharing state.
func simResyncReasonsFromTyped(reasons []simpatches.EffectResyncReason) []sim.EffectResyncReason {
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

// typedResyncReasonsFromSim converts simulation resync reasons into their
// typed equivalents, cloning the slice to avoid sharing state.
func typedResyncReasonsFromSim(reasons []sim.EffectResyncReason) []simpatches.EffectResyncReason {
	if len(reasons) == 0 {
		return nil
	}
	converted := make([]simpatches.EffectResyncReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = simpatches.EffectResyncReason{
			Kind:     reason.Kind,
			EffectID: reason.EffectID,
		}
	}
	return converted
}

func cloneTypedEffectSpawnEvents(events []simpatches.EffectSpawnEvent) []simpatches.EffectSpawnEvent {
	if len(events) == 0 {
		return nil
	}
	clones := make([]simpatches.EffectSpawnEvent, len(events))
	for i, evt := range events {
		clones[i] = simpatches.EffectSpawnEvent{
			Tick:     evt.Tick,
			Seq:      evt.Seq,
			Instance: cloneEffectInstance(evt.Instance),
		}
	}
	return clones
}

func cloneTypedEffectUpdateEvents(events []simpatches.EffectUpdateEvent) []simpatches.EffectUpdateEvent {
	if len(events) == 0 {
		return nil
	}
	clones := make([]simpatches.EffectUpdateEvent, len(events))
	for i, evt := range events {
		clone := simpatches.EffectUpdateEvent{Tick: evt.Tick, Seq: evt.Seq, ID: evt.ID}
		if evt.DeliveryState != nil {
			delivery := cloneEffectDeliveryState(*evt.DeliveryState)
			clone.DeliveryState = &delivery
		}
		if evt.BehaviorState != nil {
			behavior := cloneEffectBehaviorState(*evt.BehaviorState)
			clone.BehaviorState = &behavior
		}
		if len(evt.Params) > 0 {
			clone.Params = copyIntMap(evt.Params)
		}
		clones[i] = clone
	}
	return clones
}

func cloneTypedEffectEndEvents(events []simpatches.EffectEndEvent) []simpatches.EffectEndEvent {
	if len(events) == 0 {
		return nil
	}
	clones := make([]simpatches.EffectEndEvent, len(events))
	copy(clones, events)
	return clones
}

func cloneEffectInstance(instance effectcontract.EffectInstance) effectcontract.EffectInstance {
	clone := instance
	clone.DeliveryState = cloneEffectDeliveryState(instance.DeliveryState)
	clone.BehaviorState = cloneEffectBehaviorState(instance.BehaviorState)
	clone.Params = copyIntMap(instance.Params)
	if len(clone.Colors) > 0 {
		clone.Colors = append([]string(nil), clone.Colors...)
	}
	clone.Replication.UpdateFields = copyBoolMap(instance.Replication.UpdateFields)
	if instance.Definition != nil {
		defCopy := *instance.Definition
		defCopy.Params = copyIntMap(instance.Definition.Params)
		defCopy.Client.UpdateFields = copyBoolMap(instance.Definition.Client.UpdateFields)
		clone.Definition = &defCopy
	}
	return clone
}

func cloneEffectDeliveryState(state effectcontract.EffectDeliveryState) effectcontract.EffectDeliveryState {
	clone := state
	clone.Geometry = cloneEffectGeometry(state.Geometry)
	return clone
}

func cloneEffectBehaviorState(state effectcontract.EffectBehaviorState) effectcontract.EffectBehaviorState {
	clone := state
	clone.Stacks = copyIntMap(state.Stacks)
	clone.Extra = copyIntMap(state.Extra)
	return clone
}

func cloneEffectGeometry(src effectcontract.EffectGeometry) effectcontract.EffectGeometry {
	dst := src
	if src.Variants != nil {
		dst.Variants = copyIntMap(src.Variants)
	}
	return dst
}

func copyEffectSeqMap(src map[string]simpatches.EffectSeq) map[string]simpatches.EffectSeq {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]simpatches.EffectSeq, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
