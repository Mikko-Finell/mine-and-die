package patches

import (
	"fmt"

	"mine-and-die/server/internal/items/simsnapshots"
	"mine-and-die/server/internal/sim"
)

// PlayerView mirrors the state required to replay player patches onto a snapshot.
type PlayerView struct {
	Player   sim.Player
	IntentDX float64
	IntentDY float64
}

// Clone returns a deep copy of the player view to avoid shared slice memory.
func (view PlayerView) Clone() PlayerView {
	cloned := view
	cloned.Player = clonePlayer(view.Player)
	return cloned
}

func clonePlayer(player sim.Player) sim.Player {
	cloned := player
	cloned.Inventory = cloneInventory(player.Inventory)
	cloned.Equipment = cloneEquipment(player.Equipment)
	return cloned
}

func cloneInventory(inv sim.Inventory) sim.Inventory {
	return simsnapshots.InventoryFromSlots(inv.Slots)
}

func cloneEquipment(eq sim.Equipment) sim.Equipment {
	return simsnapshots.EquipmentFromSlots(eq.Slots)
}

// ApplyPlayers applies player-related patches to the provided snapshot view.
func ApplyPlayers(base map[string]PlayerView, patches []sim.Patch) (map[string]PlayerView, error) {
	if base == nil {
		base = make(map[string]PlayerView)
	}
	next := make(map[string]PlayerView, len(base))
	for id, view := range base {
		next[id] = view.Clone()
	}

	for _, patch := range patches {
		if patch.EntityID == "" {
			return nil, fmt.Errorf("apply patches: missing entity id for kind %q", patch.Kind)
		}

		if patch.Kind == sim.PatchPlayerRemoved {
			delete(next, patch.EntityID)
			continue
		}

		view, ok := next[patch.EntityID]
		if !ok {
			return nil, fmt.Errorf("apply patches: unknown entity %q for kind %q", patch.EntityID, patch.Kind)
		}

		switch patch.Kind {
		case sim.PatchPlayerPos:
			payload, ok := payloadAsPlayerPos(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Player.X = payload.X
			view.Player.Y = payload.Y
		case sim.PatchPlayerFacing:
			payload, ok := payloadAsPlayerFacing(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Player.Facing = payload.Facing
		case sim.PatchPlayerIntent:
			payload, ok := payloadAsPlayerIntent(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.IntentDX = payload.DX
			view.IntentDY = payload.DY
		case sim.PatchPlayerHealth:
			payload, ok := payloadAsPlayerHealth(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Player.Health = payload.Health
			if payload.MaxHealth > 0 {
				view.Player.MaxHealth = payload.MaxHealth
			}
		case sim.PatchPlayerInventory:
			payload, ok := payloadAsPlayerInventory(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Player.Inventory = simsnapshots.InventoryFromSlots(payload.Slots)
		case sim.PatchPlayerEquipment:
			payload, ok := payloadAsPlayerEquipment(patch.Payload)
			if !ok {
				return nil, fmt.Errorf("apply patches: unexpected payload %T for %q", patch.Payload, patch.Kind)
			}
			view.Player.Equipment = simsnapshots.EquipmentFromSlots(payload.Slots)
		default:
			return nil, fmt.Errorf("apply patches: unsupported patch kind %q", patch.Kind)
		}

		next[patch.EntityID] = view
	}

	return next, nil
}

func payloadAsPlayerPos(value any) (sim.PlayerPosPayload, bool) {
	switch v := value.(type) {
	case sim.PlayerPosPayload:
		return v, true
	case *sim.PlayerPosPayload:
		if v == nil {
			return sim.PlayerPosPayload{}, false
		}
		return *v, true
	default:
		return sim.PlayerPosPayload{}, false
	}
}

func payloadAsPlayerFacing(value any) (sim.PlayerFacingPayload, bool) {
	switch v := value.(type) {
	case sim.PlayerFacingPayload:
		return v, true
	case *sim.PlayerFacingPayload:
		if v == nil {
			return sim.PlayerFacingPayload{}, false
		}
		return *v, true
	default:
		return sim.PlayerFacingPayload{}, false
	}
}

func payloadAsPlayerIntent(value any) (sim.PlayerIntentPayload, bool) {
	switch v := value.(type) {
	case sim.PlayerIntentPayload:
		return v, true
	case *sim.PlayerIntentPayload:
		if v == nil {
			return sim.PlayerIntentPayload{}, false
		}
		return *v, true
	default:
		return sim.PlayerIntentPayload{}, false
	}
}

func payloadAsPlayerHealth(value any) (sim.PlayerHealthPayload, bool) {
	switch v := value.(type) {
	case sim.PlayerHealthPayload:
		return v, true
	case *sim.PlayerHealthPayload:
		if v == nil {
			return sim.PlayerHealthPayload{}, false
		}
		return *v, true
	default:
		return sim.PlayerHealthPayload{}, false
	}
}

func payloadAsPlayerInventory(value any) (sim.PlayerInventoryPayload, bool) {
	switch v := value.(type) {
	case sim.PlayerInventoryPayload:
		return v, true
	case *sim.PlayerInventoryPayload:
		if v == nil {
			return sim.PlayerInventoryPayload{}, false
		}
		return *v, true
	default:
		return sim.PlayerInventoryPayload{}, false
	}
}

func payloadAsPlayerEquipment(value any) (sim.PlayerEquipmentPayload, bool) {
	switch v := value.(type) {
	case sim.PlayerEquipmentPayload:
		return v, true
	case *sim.PlayerEquipmentPayload:
		if v == nil {
			return sim.PlayerEquipmentPayload{}, false
		}
		return *v, true
	default:
		return sim.PlayerEquipmentPayload{}, false
	}
}
