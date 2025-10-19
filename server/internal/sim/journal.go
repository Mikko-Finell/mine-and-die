package sim

import effectcontract "mine-and-die/server/effects/contract"

// EffectEventBatch mirrors the journal payloads emitted for effect lifecycle events.
type EffectEventBatch struct {
	Spawns      []effectcontract.EffectSpawnEvent  `json:"effect_spawned,omitempty"`
	Updates     []effectcontract.EffectUpdateEvent `json:"effect_update,omitempty"`
	Ends        []effectcontract.EffectEndEvent    `json:"effect_ended,omitempty"`
	LastSeqByID map[string]effectcontract.Seq      `json:"effect_seq_cursors,omitempty"`
}

// EffectResyncReason captures the trigger that requested a client resynchronisation.
type EffectResyncReason struct {
	Kind     string `json:"kind,omitempty"`
	EffectID string `json:"effect_id,omitempty"`
}

// EffectResyncSignal mirrors the legacy journal's resync hint payload.
type EffectResyncSignal struct {
	LostSpawns  uint64               `json:"lost_spawns,omitempty"`
	TotalEvents uint64               `json:"total_events,omitempty"`
	Reasons     []EffectResyncReason `json:"reasons,omitempty"`
}
