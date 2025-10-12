package main

// EffectManager owns the contract-driven effect pipeline. The current skeleton
// mirrors the contract types defined in effects_contract.go but leaves spawn,
// update, and end orchestration for future slices.
//
// The manager intentionally lives behind the enableContractEffectManager flag
// so it can collect intents and metrics without altering live gameplay until
// dual-write plumbing is ready. While wiring lands, totalEnqueued and
// totalDrained provide a temporary sanity check that tick execution drains all
// staged intents; expect these counters to be removed or repurposed once
// spawning transitions fully into the manager.
type EffectManager struct {
	intentQueue       []EffectIntent
	instances         map[string]*EffectInstance
	totalEnqueued     int
	totalDrained      int
	lastTickProcessed Tick
}

func newEffectManager() *EffectManager {
	return &EffectManager{
		intentQueue: make([]EffectIntent, 0),
		instances:   make(map[string]*EffectInstance),
	}
}

// EnqueueIntent stages an EffectIntent for future processing. The skeleton
// version simply records the request for observability while legacy systems
// remain authoritative.
func (m *EffectManager) EnqueueIntent(intent EffectIntent) {
	if m == nil {
		return
	}
	m.intentQueue = append(m.intentQueue, intent)
	m.totalEnqueued++
}

// RunTick advances the manager by one simulation tick. Until spawn/update/end
// orchestration lands, the stub only records the tick boundary and clears the
// staged intents so the queue does not grow without bound.
func (m *EffectManager) RunTick(tick Tick) {
	if m == nil {
		return
	}
	m.lastTickProcessed = tick
	if len(m.intentQueue) == 0 {
		return
	}
	drained := len(m.intentQueue)
	for i := range m.intentQueue {
		m.intentQueue[i] = EffectIntent{}
	}
	m.intentQueue = m.intentQueue[:0]
	m.totalDrained += drained
}
