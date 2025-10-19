package sim

import effectcontract "mine-and-die/server/effects/contract"

// EffectEventBatch mirrors the journal payloads emitted for effect lifecycle events.
type EffectEventBatch struct {
	Spawns      []effectcontract.EffectSpawnEvent  `json:"effect_spawned,omitempty"`
	Updates     []effectcontract.EffectUpdateEvent `json:"effect_update,omitempty"`
	Ends        []effectcontract.EffectEndEvent    `json:"effect_ended,omitempty"`
	LastSeqByID map[string]effectcontract.Seq      `json:"effect_seq_cursors,omitempty"`
}
