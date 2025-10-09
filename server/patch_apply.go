package main

import "fmt"

// PlayerPatchView mirrors the fields required to replay player patches onto a
// snapshot. It embeds the broadcast-facing Player representation and extends it
// with intent vectors that only exist in diff payloads today.
type PlayerPatchView struct {
	Player
	IntentDX float64
	IntentDY float64
}

// clonePlayerPatchView performs a deep copy of the view to avoid sharing slice
// memory between snapshots.
func clonePlayerPatchView(view PlayerPatchView) PlayerPatchView {
	cloned := view
	cloned.Inventory = view.Inventory.Clone()
	return cloned
}

// ApplyPatches applies a series of diff patches to the provided player
// snapshot, returning a new copy that reflects all mutations.
func ApplyPatches(base map[string]PlayerPatchView, patches []Patch) (map[string]PlayerPatchView, error) {
	if base == nil {
		base = make(map[string]PlayerPatchView)
	}
	next := make(map[string]PlayerPatchView, len(base))
	for id, view := range base {
		next[id] = clonePlayerPatchView(view)
	}
	for _, patch := range patches {
		if patch.EntityID == "" {
			return nil, fmt.Errorf("apply patches: missing entity id for kind %q", patch.Kind)
		}
		view, ok := next[patch.EntityID]
		if !ok {
			return nil, fmt.Errorf("apply patches: unknown entity %q for kind %q", patch.EntityID, patch.Kind)
		}
		switch patch.Kind {
		case PatchPlayerPos:
			payload, ok := payloadAsPlayerPos(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.X = payload.X
			view.Y = payload.Y
		case PatchPlayerFacing:
			payload, ok := payloadAsPlayerFacing(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Facing = payload.Facing
		case PatchPlayerIntent:
			payload, ok := payloadAsPlayerIntent(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.IntentDX = payload.DX
			view.IntentDY = payload.DY
		case PatchPlayerHealth:
			payload, ok := payloadAsPlayerHealth(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Health = payload.Health
			if payload.MaxHealth > 0 {
				view.MaxHealth = payload.MaxHealth
			}
		case PatchPlayerInventory:
			payload, ok := payloadAsPlayerInventory(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Inventory = Inventory{Slots: cloneInventorySlots(payload.Slots)}
		default:
			return nil, fmt.Errorf("apply patches: unsupported patch kind %q", patch.Kind)
		}
		next[patch.EntityID] = view
	}
	return next, nil
}

func payloadAsPlayerPos(value any) (PlayerPosPayload, bool) {
	switch v := value.(type) {
	case PlayerPosPayload:
		return v, true
	case *PlayerPosPayload:
		if v == nil {
			return PlayerPosPayload{}, false
		}
		return *v, true
	default:
		return PlayerPosPayload{}, false
	}
}

func payloadAsPlayerFacing(value any) (PlayerFacingPayload, bool) {
	switch v := value.(type) {
	case PlayerFacingPayload:
		return v, true
	case *PlayerFacingPayload:
		if v == nil {
			return PlayerFacingPayload{}, false
		}
		return *v, true
	default:
		return PlayerFacingPayload{}, false
	}
}

func payloadAsPlayerIntent(value any) (PlayerIntentPayload, bool) {
	switch v := value.(type) {
	case PlayerIntentPayload:
		return v, true
	case *PlayerIntentPayload:
		if v == nil {
			return PlayerIntentPayload{}, false
		}
		return *v, true
	default:
		return PlayerIntentPayload{}, false
	}
}

func payloadAsPlayerHealth(value any) (PlayerHealthPayload, bool) {
	switch v := value.(type) {
	case PlayerHealthPayload:
		return v, true
	case *PlayerHealthPayload:
		if v == nil {
			return PlayerHealthPayload{}, false
		}
		return *v, true
	default:
		return PlayerHealthPayload{}, false
	}
}

func payloadAsPlayerInventory(value any) (PlayerInventoryPayload, bool) {
	switch v := value.(type) {
	case PlayerInventoryPayload:
		return v, true
	case *PlayerInventoryPayload:
		if v == nil {
			return PlayerInventoryPayload{}, false
		}
		return *v, true
	default:
		return PlayerInventoryPayload{}, false
	}
}
