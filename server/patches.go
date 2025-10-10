package main

import (
	"os"
	"strconv"
	"sync"
	"time"
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

	// PatchNPCPos updates an NPC's position.
	PatchNPCPos PatchKind = "npc_pos"
	// PatchNPCFacing updates an NPC's facing direction.
	PatchNPCFacing PatchKind = "npc_facing"
	// PatchNPCHealth updates an NPC's health pool.
	PatchNPCHealth PatchKind = "npc_health"
	// PatchNPCInventory updates an NPC's inventory slots.
	PatchNPCInventory PatchKind = "npc_inventory"

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
	mu        sync.RWMutex
	patches   []Patch
	keyframes []keyframe
	maxFrames int
	maxAge    time.Duration
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
	}
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
