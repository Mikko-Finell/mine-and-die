package main

const defaultJournalKeyframeCapacity = 8

// PatchKind identifies the type of diff entry.
type PatchKind string

const (
	// PatchPlayerPos updates a player's position.
	PatchPlayerPos PatchKind = "player_pos"
)

// Patch represents a diff entry that can be applied to the client state.
type Patch struct {
	Kind     PatchKind `json:"kind"`
	EntityID string    `json:"entityId"`
	Payload  any       `json:"payload,omitempty"`
}

// PlayerPosPayload captures the coordinates for a player position patch.
type PlayerPosPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Journal accumulates patches generated during a tick and keeps a rolling
// buffer of recent keyframes so future diff recovery can rehydrate state.
type Journal struct {
	patches   []Patch
	keyframes keyframeRing
}

// newJournal constructs a journal with storage for the configured number of
// keyframes.
func newJournal(keyframeCapacity int) Journal {
	journal := Journal{}
	if keyframeCapacity < 0 {
		keyframeCapacity = 0
	}
	journal.keyframes = newKeyframeRing(keyframeCapacity)
	journal.patches = make([]Patch, 0)
	return journal
}

// AppendPatch records a patch for the current tick.
func (j *Journal) AppendPatch(p Patch) {
	j.patches = append(j.patches, p)
}

// DrainPatches returns all staged patches and clears the in-memory slice.
func (j *Journal) DrainPatches() []Patch {
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
	if len(j.patches) == 0 {
		return nil
	}
	snapshot := make([]Patch, len(j.patches))
	copy(snapshot, j.patches)
	return snapshot
}

// RecordKeyframe stores a keyframe in the ring buffer.
func (j *Journal) RecordKeyframe(frame keyframe) {
	j.keyframes.Push(frame)
}

// Keyframes exposes the current keyframe ring contents in chronological
// order. Callers receive a copy to avoid holding references into the ring.
func (j *Journal) Keyframes() []keyframe {
	return j.keyframes.Frames()
}

// keyframe captures a full snapshot of the world state. The struct is
// intentionally minimal for now so future diffs can expand it without touching
// the broadcast layer again.
type keyframe struct {
	Tick uint64
}

// keyframeRing maintains a fixed-size circular buffer of keyframes.
type keyframeRing struct {
	frames []keyframe
	next   int
	filled bool
}

func newKeyframeRing(capacity int) keyframeRing {
	if capacity <= 0 {
		return keyframeRing{}
	}
	return keyframeRing{frames: make([]keyframe, capacity)}
}

// Push inserts a keyframe into the ring, overwriting the oldest entry when the
// capacity is exceeded.
func (r *keyframeRing) Push(frame keyframe) {
	if len(r.frames) == 0 {
		return
	}
	r.frames[r.next] = frame
	r.next = (r.next + 1) % len(r.frames)
	if r.next == 0 {
		r.filled = true
	}
}

// Frames returns a chronological copy of the buffered keyframes.
func (r *keyframeRing) Frames() []keyframe {
	if len(r.frames) == 0 {
		return nil
	}
	var count int
	if r.filled {
		count = len(r.frames)
	} else {
		count = r.next
	}
	if count == 0 {
		return nil
	}
	ordered := make([]keyframe, 0, count)
	if r.filled {
		ordered = append(ordered, r.frames[r.next:]...)
		ordered = append(ordered, r.frames[:r.next]...)
		return ordered
	}
	ordered = append(ordered, r.frames[:r.next]...)
	return ordered
}
