package journal

import (
	"fmt"
)

type ResyncReason struct {
	Kind     string
	EffectID string
}

type ResyncSignal struct {
	LostSpawns  uint64
	TotalEvents uint64
	Reasons     []ResyncReason
}

type Policy struct {
	totalEvents uint64
	lostSpawns  uint64
	pending     bool
	reasons     []ResyncReason
}

const lostSpawnThresholdPerTenThousand = 1
const resyncReasonLimit = 8

func NewPolicy() *Policy {
	return &Policy{reasons: make([]ResyncReason, 0, resyncReasonLimit)}
}

func (p *Policy) NoteEvent() {
	if p == nil {
		return
	}
	if p.totalEvents == ^uint64(0) {
		p.totalEvents = p.totalEvents / 2
		p.lostSpawns = p.lostSpawns / 2
	}
	p.totalEvents++
}

func (p *Policy) NoteLostSpawn(kind, effectID string) {
	if p == nil {
		return
	}
	p.lostSpawns++
	if len(p.reasons) < resyncReasonLimit {
		p.reasons = append(p.reasons, ResyncReason{Kind: kind, EffectID: effectID})
	}
	p.evaluate()
}

func (p *Policy) evaluate() {
	if p == nil || p.pending || p.lostSpawns == 0 {
		return
	}
	total := p.totalEvents
	if total == 0 {
		total = 1
	}
	if p.lostSpawns*10000 >= total*lostSpawnThresholdPerTenThousand {
		p.pending = true
	}
}

func (p *Policy) Consume() (ResyncSignal, bool) {
	if p == nil || !p.pending {
		return ResyncSignal{}, false
	}
	signal := ResyncSignal{
		LostSpawns:  p.lostSpawns,
		TotalEvents: p.totalEvents,
		Reasons:     append([]ResyncReason(nil), p.reasons...),
	}
	p.pending = false
	p.totalEvents = 0
	p.lostSpawns = 0
	if len(p.reasons) > 0 {
		p.reasons = p.reasons[:0]
	}
	return signal, true
}

func (s ResyncSignal) Summary() string {
	if s.LostSpawns == 0 && s.TotalEvents == 0 {
		return ""
	}
	return fmt.Sprintf("lost_spawns=%d total_events=%d reasons=%v", s.LostSpawns, s.TotalEvents, s.Reasons)
}
