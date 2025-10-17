package main

import (
	"os"
	"strconv"
	"sync"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

const defaultJournalKeyframeCapacity = 8
const defaultJournalKeyframeMaxAge = 5 * time.Second

const (
	envJournalCapacity = "KEYFRAME_JOURNAL_CAPACITY"
	envJournalMaxAgeMS = "KEYFRAME_JOURNAL_MAX_AGE_MS"
)

// PatchKind identifies the type of diff entry.
type PatchKind string

const (
	// PatchPlayerPos updates a player's position.
	PatchPlayerPos PatchKind = "player_pos"
	// PatchPlayerFacing updates a player's facing direction.
	PatchPlayerFacing PatchKind = "player_facing"
	// PatchPlayerIntent updates a player's movement intent vector.
	PatchPlayerIntent PatchKind = "player_intent"
	// PatchPlayerHealth updates a player's health pool.
	PatchPlayerHealth PatchKind = "player_health"
	// PatchPlayerInventory updates a player's inventory slots.
	PatchPlayerInventory PatchKind = "player_inventory"
	// PatchPlayerEquipment updates a player's equipment loadout.
	PatchPlayerEquipment PatchKind = "player_equipment"
	// PatchPlayerRemoved signals that a player has been removed from the world.
	PatchPlayerRemoved PatchKind = "player_removed"

	// PatchNPCPos updates an NPC's position.
	PatchNPCPos PatchKind = "npc_pos"
	// PatchNPCFacing updates an NPC's facing direction.
	PatchNPCFacing PatchKind = "npc_facing"
	// PatchNPCHealth updates an NPC's health pool.
	PatchNPCHealth PatchKind = "npc_health"
	// PatchNPCInventory updates an NPC's inventory slots.
	PatchNPCInventory PatchKind = "npc_inventory"
	// PatchNPCEquipment updates an NPC's equipment loadout.
	PatchNPCEquipment PatchKind = "npc_equipment"

	// PatchEffectPos updates an effect's position.
	PatchEffectPos PatchKind = "effect_pos"
	// PatchEffectParams updates an effect's parameter map.
	PatchEffectParams PatchKind = "effect_params"

	// PatchGroundItemPos updates a ground item's position.
	PatchGroundItemPos PatchKind = "ground_item_pos"
	// PatchGroundItemQty updates a ground item's quantity.
	PatchGroundItemQty PatchKind = "ground_item_qty"
)

// Patch represents a diff entry that can be applied to the client state.
type Patch struct {
	Kind     PatchKind `json:"kind"`
	EntityID string    `json:"entityId"`
	Payload  any       `json:"payload,omitempty"`
}

// PositionPayload captures the coordinates for an entity position patch.
type PositionPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PlayerPosPayload captures the coordinates for a player position patch.
type PlayerPosPayload = PositionPayload

// NPCPosPayload captures the coordinates for an NPC position patch.
type NPCPosPayload = PositionPayload

// EffectPosPayload captures the coordinates for an effect position patch.
type EffectPosPayload = PositionPayload

// GroundItemPosPayload captures the coordinates for a ground item position patch.
type GroundItemPosPayload = PositionPayload

// FacingPayload captures the facing for an entity patch.
type FacingPayload struct {
	Facing FacingDirection `json:"facing"`
}

// PlayerFacingPayload captures the facing for a player patch.
type PlayerFacingPayload = FacingPayload

// NPCFacingPayload captures the facing for an NPC patch.
type NPCFacingPayload = FacingPayload

// PlayerIntentPayload captures the movement intent vector for a player patch.
type PlayerIntentPayload struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
}

// HealthPayload captures the health for an entity patch.
type HealthPayload struct {
	Health    float64 `json:"health"`
	MaxHealth float64 `json:"maxHealth,omitempty"`
}

// PlayerHealthPayload captures the health for a player patch.
type PlayerHealthPayload = HealthPayload

// NPCHealthPayload captures the health for an NPC patch.
type NPCHealthPayload = HealthPayload

// InventoryPayload captures the inventory slots for an entity patch.
type InventoryPayload struct {
	Slots []InventorySlot `json:"slots"`
}

// PlayerInventoryPayload captures the inventory slots for a player patch.
type PlayerInventoryPayload = InventoryPayload

// NPCInventoryPayload captures the inventory slots for an NPC patch.
type NPCInventoryPayload = InventoryPayload

// EquipmentPayload captures the equipped items for an entity patch.
type EquipmentPayload struct {
	Slots []EquippedItem `json:"slots"`
}

// PlayerEquipmentPayload captures the equipped items for a player patch.
type PlayerEquipmentPayload = EquipmentPayload

// NPCEquipmentPayload captures the equipped items for an NPC patch.
type NPCEquipmentPayload = EquipmentPayload

// EffectParamsPayload captures the mutable parameters for an effect patch.
type EffectParamsPayload struct {
	Params map[string]float64 `json:"params"`
}

// GroundItemQtyPayload captures the quantity for a ground item patch.
type GroundItemQtyPayload struct {
	Qty int `json:"qty"`
}

// Journal accumulates patches generated during a tick and keeps a rolling
// buffer of recent keyframes so future diff recovery can rehydrate state.
type Journal struct {
	mu            sync.RWMutex
	patches       []Patch
	keyframes     []keyframe
	maxFrames     int
	maxAge        time.Duration
	effectSeq     map[string]effectcontract.Seq
	effects       effectEventBuffer
	endedIDs      []string
	recentlyEnded map[string]effectcontract.Tick
	telemetry     *telemetryCounters
	resync        *resyncPolicy
}

// newJournal constructs a journal with storage for the configured number of
// keyframes and retention window.
func newJournal(keyframeCapacity int, maxAge time.Duration) Journal {
	if keyframeCapacity < 0 {
		keyframeCapacity = 0
	}
	if maxAge < 0 {
		maxAge = 0
	}
	return Journal{
		patches:   make([]Patch, 0),
		keyframes: make([]keyframe, 0, keyframeCapacity),
		maxFrames: keyframeCapacity,
		maxAge:    maxAge,
		effectSeq: make(map[string]effectcontract.Seq),
		effects: effectEventBuffer{
			spawns:  make([]effectcontract.EffectSpawnEvent, 0),
			updates: make([]effectcontract.EffectUpdateEvent, 0),
			ends:    make([]effectcontract.EffectEndEvent, 0),
		},
		endedIDs:      make([]string, 0),
		recentlyEnded: make(map[string]effectcontract.Tick),
		resync:        newResyncPolicy(),
	}
}

const journalRecentlyEndedWindow effectcontract.Tick = 4

type effectEventBuffer struct {
	spawns  []effectcontract.EffectSpawnEvent
	updates []effectcontract.EffectUpdateEvent
	ends    []effectcontract.EffectEndEvent
}

// EffectEventBatch captures the lifecycle envelopes recorded for the current
// journal window alongside the per-effect sequence counters used for
// idempotency in replay tooling.
type EffectEventBatch struct {
	Spawns      []effectcontract.EffectSpawnEvent  `json:"effect_spawned,omitempty"`
	Updates     []effectcontract.EffectUpdateEvent `json:"effect_update,omitempty"`
	Ends        []effectcontract.EffectEndEvent    `json:"effect_ended,omitempty"`
	LastSeqByID map[string]effectcontract.Seq      `json:"effect_seq_cursors,omitempty"`
}

// journalConfig loads retention settings from the environment falling back to
// defaults when unset or invalid.
func journalConfig() (int, time.Duration) {
	capacity := defaultJournalKeyframeCapacity
	if raw := os.Getenv(envJournalCapacity); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			capacity = parsed
		}
	}

	maxAge := defaultJournalKeyframeMaxAge
	if raw := os.Getenv(envJournalMaxAgeMS); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			maxAge = time.Duration(parsed) * time.Millisecond
		}
	}

	if capacity < 0 {
		capacity = 0
	}
	if maxAge < 0 {
		maxAge = 0
	}

	return capacity, maxAge
}

// AppendPatch records a patch for the current tick.
func (j *Journal) AppendPatch(p Patch) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.patches = append(j.patches, p)
}

// RecordEffectSpawn registers an effect_spawned envelope in the journal.
// The journal owns the per-effect sequence counter so replay tooling can drop
// duplicates deterministically. The returned event mirrors the stored payload.
func (j *Journal) RecordEffectSpawn(event effectcontract.EffectSpawnEvent) effectcontract.EffectSpawnEvent {
	if event.Instance.ID == "" {
		return effectcontract.EffectSpawnEvent{}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.resync != nil {
		j.resync.noteEvent()
	}
	j.clearPendingEndLocked(event.Instance.ID)
	delete(j.recentlyEnded, event.Instance.ID)
	j.effectSeq[event.Instance.ID] = 0
	if event.Seq <= 0 {
		event.Seq = j.nextEffectSeqLocked(event.Instance.ID)
	} else {
		j.effectSeq[event.Instance.ID] = event.Seq
	}
	event.Instance = cloneEffectInstance(event.Instance)
	j.effects.spawns = append(j.effects.spawns, event)
	return event
}

// RecordEffectUpdate registers an effect_update envelope in the journal and
// returns the stored event with the assigned sequence value.
func (j *Journal) RecordEffectUpdate(event effectcontract.EffectUpdateEvent) effectcontract.EffectUpdateEvent {
	if event.ID == "" {
		return effectcontract.EffectUpdateEvent{}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.resync != nil {
		j.resync.noteEvent()
	}
	last, ok := j.effectSeq[event.ID]
	if !ok || last == 0 {
		j.recordJournalDropLocked(metricJournalUnknownIDUpdate)
		if j.resync != nil {
			j.resync.noteLostSpawn(metricJournalUnknownIDUpdate, event.ID)
		}
		return effectcontract.EffectUpdateEvent{}
	}
	j.pruneRecentlyEndedLocked(event.Tick)
	if _, recently := j.recentlyEnded[event.ID]; recently {
		j.recordJournalDropLocked(metricJournalUpdateAfterEnd)
		if j.resync != nil {
			j.resync.noteLostSpawn(metricJournalUpdateAfterEnd, event.ID)
		}
		return effectcontract.EffectUpdateEvent{}
	}
	if event.Seq <= 0 {
		event.Seq = j.nextEffectSeqLocked(event.ID)
	} else if event.Seq <= last {
		j.recordJournalDropLocked(metricJournalNonMonotonicSeq)
		return effectcontract.EffectUpdateEvent{}
	} else {
		j.effectSeq[event.ID] = event.Seq
	}
	cloned := effectcontract.EffectUpdateEvent{
		Tick: event.Tick,
		Seq:  event.Seq,
		ID:   event.ID,
	}
	if event.DeliveryState != nil {
		delivery := cloneEffectDeliveryState(*event.DeliveryState)
		cloned.DeliveryState = &delivery
	}
	if event.BehaviorState != nil {
		behavior := cloneEffectBehaviorState(*event.BehaviorState)
		cloned.BehaviorState = &behavior
	}
	if len(event.Params) > 0 {
		cloned.Params = copyIntMap(event.Params)
	}
	j.effects.updates = append(j.effects.updates, cloned)
	return cloned
}

// RecordEffectEnd registers an effect_ended envelope in the journal. The
// journal retains the final sequence cursor until the batch is drained so
// replay tooling can confirm ordering before the id is reclaimed.
func (j *Journal) RecordEffectEnd(event effectcontract.EffectEndEvent) effectcontract.EffectEndEvent {
	if event.ID == "" {
		return effectcontract.EffectEndEvent{}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.resync != nil {
		j.resync.noteEvent()
	}
	last, ok := j.effectSeq[event.ID]
	if !ok || last == 0 {
		j.recordJournalDropLocked(metricJournalUnknownIDUpdate)
		if j.resync != nil {
			j.resync.noteLostSpawn(metricJournalUnknownIDUpdate, event.ID)
		}
		return effectcontract.EffectEndEvent{}
	}
	j.pruneRecentlyEndedLocked(event.Tick)
	if event.Seq <= 0 {
		event.Seq = j.nextEffectSeqLocked(event.ID)
	} else if event.Seq <= last {
		j.recordJournalDropLocked(metricJournalNonMonotonicSeq)
		return effectcontract.EffectEndEvent{}
	} else {
		j.effectSeq[event.ID] = event.Seq
	}
	j.effects.ends = append(j.effects.ends, event)
	j.endedIDs = append(j.endedIDs, event.ID)
	j.recentlyEnded[event.ID] = event.Tick
	return event
}

// PurgeEntity drops all staged patches that reference the provided entity ID.
// It keeps the journal internally consistent when actors are removed before
// the next broadcast.
func (j *Journal) PurgeEntity(entityID string) {
	if entityID == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.patches) == 0 {
		return
	}
	filtered := j.patches[:0]
	for _, patch := range j.patches {
		if patch.EntityID == entityID {
			continue
		}
		filtered = append(filtered, patch)
	}
	if len(filtered) == len(j.patches) {
		return
	}
	j.patches = filtered
}

// DrainPatches returns all staged patches and clears the in-memory slice.
func (j *Journal) DrainPatches() []Patch {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.patches) == 0 {
		return nil
	}
	drained := make([]Patch, len(j.patches))
	copy(drained, j.patches)
	j.patches = j.patches[:0]
	return drained
}

// SnapshotPatches returns a copy of the staged patches without clearing the
// journal.
func (j *Journal) SnapshotPatches() []Patch {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if len(j.patches) == 0 {
		return nil
	}
	snapshot := make([]Patch, len(j.patches))
	copy(snapshot, j.patches)
	return snapshot
}

// RestorePatches prepends the provided patches back into the journal. It is
// used when a caller drains the journal but later needs to roll the operation
// back (for example, if encoding fails and the state message cannot be sent).
func (j *Journal) RestorePatches(p []Patch) {
	if len(p) == 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	restored := make([]Patch, 0, len(p)+len(j.patches))
	restored = append(restored, p...)
	restored = append(restored, j.patches...)
	j.patches = restored
}

// DrainEffectEvents returns the recorded lifecycle envelopes for the journal
// window along with the current per-effect sequence cursors. Slices are copied
// so callers can mutate the results without impacting the journal. After the
// drain the buffered events are cleared and sequence entries for ended effects
// are released.
func (j *Journal) DrainEffectEvents() EffectEventBatch {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.effects.spawns) == 0 && len(j.effects.updates) == 0 && len(j.effects.ends) == 0 {
		return EffectEventBatch{}
	}
	batch := EffectEventBatch{
		Spawns:      cloneSpawnEvents(j.effects.spawns),
		Updates:     cloneUpdateEvents(j.effects.updates),
		Ends:        cloneEndEvents(j.effects.ends),
		LastSeqByID: copySeqMap(j.effectSeq),
	}
	j.effects.spawns = j.effects.spawns[:0]
	j.effects.updates = j.effects.updates[:0]
	j.effects.ends = j.effects.ends[:0]
	if len(j.endedIDs) > 0 {
		for _, id := range j.endedIDs {
			delete(j.effectSeq, id)
			delete(j.recentlyEnded, id)
		}
		j.endedIDs = j.endedIDs[:0]
	}
	return batch
}

// SnapshotEffectEvents returns a copy of the currently staged lifecycle
// envelopes without clearing the journal.
func (j *Journal) SnapshotEffectEvents() EffectEventBatch {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if len(j.effects.spawns) == 0 && len(j.effects.updates) == 0 && len(j.effects.ends) == 0 {
		return EffectEventBatch{}
	}
	return EffectEventBatch{
		Spawns:      cloneSpawnEvents(j.effects.spawns),
		Updates:     cloneUpdateEvents(j.effects.updates),
		Ends:        cloneEndEvents(j.effects.ends),
		LastSeqByID: copySeqMap(j.effectSeq),
	}
}

// RestoreEffectEvents reinserts a drained lifecycle batch. It keeps the
// journal consistent when callers encounter an error after draining and need to
// retry without losing events.
func (j *Journal) RestoreEffectEvents(batch EffectEventBatch) {
	if len(batch.Spawns) == 0 && len(batch.Updates) == 0 && len(batch.Ends) == 0 && len(batch.LastSeqByID) == 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()

	if len(batch.Spawns) > 0 {
		restored := make([]effectcontract.EffectSpawnEvent, 0, len(batch.Spawns)+len(j.effects.spawns))
		restored = append(restored, batch.Spawns...)
		restored = append(restored, j.effects.spawns...)
		j.effects.spawns = restored
	}
	if len(batch.Updates) > 0 {
		restored := make([]effectcontract.EffectUpdateEvent, 0, len(batch.Updates)+len(j.effects.updates))
		restored = append(restored, batch.Updates...)
		restored = append(restored, j.effects.updates...)
		j.effects.updates = restored
	}
	if len(batch.Ends) > 0 {
		restored := make([]effectcontract.EffectEndEvent, 0, len(batch.Ends)+len(j.effects.ends))
		restored = append(restored, batch.Ends...)
		restored = append(restored, j.effects.ends...)
		j.effects.ends = restored

		if j.recentlyEnded == nil {
			j.recentlyEnded = make(map[string]effectcontract.Tick)
		}

		seen := make(map[string]struct{}, len(batch.Ends))
		ended := make([]string, 0, len(batch.Ends)+len(j.endedIDs))
		for _, evt := range batch.Ends {
			if evt.ID == "" {
				continue
			}
			seen[evt.ID] = struct{}{}
			j.recentlyEnded[evt.ID] = evt.Tick
			ended = append(ended, evt.ID)
		}
		for _, id := range j.endedIDs {
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			ended = append(ended, id)
		}
		j.endedIDs = ended
	}
	if len(batch.LastSeqByID) > 0 {
		if j.effectSeq == nil {
			j.effectSeq = make(map[string]effectcontract.Seq, len(batch.LastSeqByID))
		}
		for id, seq := range batch.LastSeqByID {
			if id == "" {
				continue
			}
			if current, ok := j.effectSeq[id]; ok && current > seq {
				continue
			}
			j.effectSeq[id] = seq
		}
	}
}

// ConsumeResyncHint reports whether the journal observed a lost-spawn pattern
// that should trigger a client resynchronisation. Counters reset after each
// consumption so the caller can re-evaluate on subsequent ticks.
func (j *Journal) ConsumeResyncHint() (resyncSignal, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.resync == nil {
		return resyncSignal{}, false
	}
	return j.resync.consume()
}

// RecordKeyframe stores a keyframe in the buffer enforcing retention limits
// by count and age.
func (j *Journal) RecordKeyframe(frame keyframe) keyframeRecordResult {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.maxFrames == 0 {
		j.keyframes = j.keyframes[:0]
		return keyframeRecordResult{}
	}

	frame.RecordedAt = time.Now()
	j.keyframes = append(j.keyframes, frame)

	cutoff := time.Time{}
	if j.maxAge > 0 {
		cutoff = frame.RecordedAt.Add(-j.maxAge)
	}

	evicted := make([]journalEviction, 0)
	if !cutoff.IsZero() {
		idx := 0
		for idx < len(j.keyframes) {
			if !j.keyframes[idx].RecordedAt.Before(cutoff) {
				break
			}
			evicted = append(evicted, journalEviction{
				Sequence: j.keyframes[idx].Sequence,
				Tick:     j.keyframes[idx].Tick,
				Reason:   "expired",
			})
			idx++
		}
		if idx > 0 {
			copy(j.keyframes, j.keyframes[idx:])
			j.keyframes = j.keyframes[:len(j.keyframes)-idx]
		}
	}

	if j.maxFrames > 0 && len(j.keyframes) > j.maxFrames {
		overflow := len(j.keyframes) - j.maxFrames
		for i := 0; i < overflow; i++ {
			frame := j.keyframes[i]
			evicted = append(evicted, journalEviction{
				Sequence: frame.Sequence,
				Tick:     frame.Tick,
				Reason:   "count",
			})
		}
		copy(j.keyframes, j.keyframes[overflow:])
		j.keyframes = j.keyframes[:len(j.keyframes)-overflow]
	}

	size := len(j.keyframes)
	result := keyframeRecordResult{Size: size}
	if size > 0 {
		result.OldestSequence = j.keyframes[0].Sequence
		result.NewestSequence = j.keyframes[size-1].Sequence
	}
	result.Evicted = evicted
	return result
}

// Keyframes exposes the current keyframe buffer contents in chronological
// order. Callers receive a copy to avoid holding references into the buffer.
func (j *Journal) Keyframes() []keyframe {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if len(j.keyframes) == 0 {
		return nil
	}
	frames := make([]keyframe, len(j.keyframes))
	copy(frames, j.keyframes)
	return frames
}

// KeyframeBySequence returns the keyframe matching the provided sequence.
func (j *Journal) KeyframeBySequence(sequence uint64) (keyframe, bool) {
	if sequence == 0 {
		return keyframe{}, false
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	for _, frame := range j.keyframes {
		if frame.Sequence == sequence {
			return frame, true
		}
	}
	return keyframe{}, false
}

// KeyframeWindow reports the current retention window.
func (j *Journal) KeyframeWindow() (size int, oldest, newest uint64) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	size = len(j.keyframes)
	if size == 0 {
		return size, 0, 0
	}
	oldest = j.keyframes[0].Sequence
	newest = j.keyframes[size-1].Sequence
	return size, oldest, newest
}

func (j *Journal) nextEffectSeqLocked(id string) effectcontract.Seq {
	if id == "" {
		return 0
	}
	next := j.effectSeq[id] + 1
	j.effectSeq[id] = next
	return next
}

func (j *Journal) pruneRecentlyEndedLocked(current effectcontract.Tick) {
	if len(j.recentlyEnded) == 0 || current <= 0 {
		return
	}
	cutoff := current - journalRecentlyEndedWindow
	for id, tick := range j.recentlyEnded {
		if tick <= 0 {
			continue
		}
		if tick <= cutoff {
			delete(j.recentlyEnded, id)
		}
	}
}

func (j *Journal) clearPendingEndLocked(id string) {
	if len(j.endedIDs) == 0 {
		return
	}
	filtered := j.endedIDs[:0]
	for _, endedID := range j.endedIDs {
		if endedID == id {
			continue
		}
		filtered = append(filtered, endedID)
	}
	j.endedIDs = filtered
}

const (
	metricJournalNonMonotonicSeq = "journal_nonmonotonic_seq"
	metricJournalUnknownIDUpdate = "journal_unknown_id_update"
	metricJournalUpdateAfterEnd  = "journal_update_after_end"
)

func (j *Journal) recordJournalDropLocked(metric string) {
	if j.telemetry == nil || metric == "" {
		return
	}
	j.telemetry.RecordJournalDrop(metric)
}

func (j *Journal) AttachTelemetry(t *telemetryCounters) {
	j.mu.Lock()
	j.telemetry = t
	j.mu.Unlock()
}

func cloneSpawnEvents(events []effectcontract.EffectSpawnEvent) []effectcontract.EffectSpawnEvent {
	if len(events) == 0 {
		return nil
	}
	clones := make([]effectcontract.EffectSpawnEvent, len(events))
	for i, evt := range events {
		clones[i] = effectcontract.EffectSpawnEvent{
			Tick:     evt.Tick,
			Seq:      evt.Seq,
			Instance: cloneEffectInstance(evt.Instance),
		}
	}
	return clones
}

func cloneUpdateEvents(events []effectcontract.EffectUpdateEvent) []effectcontract.EffectUpdateEvent {
	if len(events) == 0 {
		return nil
	}
	clones := make([]effectcontract.EffectUpdateEvent, len(events))
	for i, evt := range events {
		clone := effectcontract.EffectUpdateEvent{Tick: evt.Tick, Seq: evt.Seq, ID: evt.ID}
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

func cloneEndEvents(events []effectcontract.EffectEndEvent) []effectcontract.EffectEndEvent {
	if len(events) == 0 {
		return nil
	}
	clones := make([]effectcontract.EffectEndEvent, len(events))
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
	clone.Geometry = cloneGeometry(state.Geometry)
	return clone
}

func cloneEffectBehaviorState(state effectcontract.EffectBehaviorState) effectcontract.EffectBehaviorState {
	clone := state
	clone.Stacks = copyIntMap(state.Stacks)
	clone.Extra = copyIntMap(state.Extra)
	return clone
}

func copySeqMap(src map[string]effectcontract.Seq) map[string]effectcontract.Seq {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]effectcontract.Seq, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// keyframe captures a full snapshot of the world state. The struct is
// intentionally minimal for now so future diffs can expand it without touching
// the broadcast layer again.
type keyframe struct {
	Tick        uint64
	Sequence    uint64
	Players     []Player
	NPCs        []NPC
	Obstacles   []Obstacle
	Effects     []Effect
	GroundItems []GroundItem
	Config      worldConfig
	RecordedAt  time.Time
}

type journalEviction struct {
	Sequence uint64
	Tick     uint64
	Reason   string
}

type keyframeRecordResult struct {
	Size           int
	OldestSequence uint64
	NewestSequence uint64
	Evicted        []journalEviction
}
