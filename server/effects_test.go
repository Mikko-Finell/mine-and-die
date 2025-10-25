package server

import (
	"testing"
	"time"

	itemspkg "mine-and-die/server/internal/items"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestNPCMiningEmitsInventoryPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	w.players = make(map[string]*playerState)
	w.npcs = make(map[string]*npcState)
	w.obstacles = []Obstacle{{
		ID:     "ore-test-1",
		Type:   obstacleTypeGoldOre,
		X:      0,
		Y:      0,
		Width:  64,
		Height: 64,
	}}

	npc := &npcState{
		ActorState: actorState{Actor: Actor{
			ID:        "npc-miner",
			X:         2,
			Y:         2,
			Facing:    defaultFacing,
			Health:    baselinePlayerMaxHealth,
			MaxHealth: baselinePlayerMaxHealth,
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
		}},
		Stats: stats.DefaultComponent(stats.ArchetypeGoblin),
	}
	w.npcs[npc.ID] = npc

	effect := &effectState{Type: effectTypeAttack}
	area := Obstacle{X: 0, Y: 0, Width: 64, Height: 64}

	now := time.Now()
	worldpkg.ResolveMeleeImpact(worldpkg.ResolveMeleeImpactConfig{
		EffectType: effect.Type,
		Effect:     effect,
		Owner:      &npc.ActorState,
		ActorID:    npc.ID,
		Tick:       1,
		Now:        now,
		Area: worldpkg.Obstacle{
			X:      area.X,
			Y:      area.Y,
			Width:  area.Width,
			Height: area.Height,
		},
		Obstacles: w.obstacles,
		ForEachPlayer: func(visit func(id string, x, y float64, reference any)) {
			for id, player := range w.players {
				if player == nil {
					continue
				}
				visit(id, player.X, player.Y, player)
			}
		},
		ForEachNPC: func(visit func(id string, x, y float64, reference any)) {
			for id, state := range w.npcs {
				if state == nil {
					continue
				}
				visit(id, state.X, state.Y, state)
			}
		},
		GivePlayerGold: func(id string) (bool, error) {
			if _, ok := w.players[id]; !ok {
				return false, nil
			}
			err := w.MutateInventory(id, func(inv *Inventory) error {
				if inv == nil {
					return nil
				}
				_, addErr := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
				return addErr
			})
			return true, err
		},
		GiveNPCGold: func(id string) (bool, error) {
			if _, ok := w.npcs[id]; !ok {
				return false, nil
			}
			err := w.MutateNPCInventory(id, func(inv *Inventory) error {
				if inv == nil {
					return nil
				}
				_, addErr := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
				return addErr
			})
			return true, err
		},
		GiveOwnerGold: func(ref any) error {
			actor, ok := ref.(*actorState)
			if !ok || actor == nil {
				return nil
			}
			_, err := actor.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
			return err
		},
		ApplyPlayerHit: func(effectRef any, target any, when time.Time) {
			if w.playerHitCallback == nil {
				return
			}
			w.playerHitCallback(effectRef, target, when)
		},
		ApplyNPCHit: func(effectRef any, target any, when time.Time) {
			if w.npcHitCallback == nil {
				return
			}
			w.npcHitCallback(effectRef, target, when)
		},
		RecordGoldGrantFailure: func(actorID string, obstacleID string, err error) {
			if err != nil {
				t.Fatalf("unexpected gold grant failure: %v", err)
			}
		},
		RecordAttackOverlap: func(actorID string, tick uint64, effectType string, playerHits []string, npcHits []string) {
			// The mining scenario only awards ore and should not record hits.
			if len(playerHits) > 0 || len(npcHits) > 0 {
				t.Fatalf("expected no attack overlap telemetry, got players=%v npcs=%v", playerHits, npcHits)
			}
		},
	})

	if qty := npc.Inventory.QuantityOf(ItemTypeGold); qty != 1 {
		t.Fatalf("expected npc to receive 1 gold, got %d", qty)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchNPCInventory {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCInventory, patch.Kind)
	}

	payload, ok := patch.Payload.(NPCInventoryPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCInventoryPayload, got %T", patch.Payload)
	}
	slots := itemspkg.SimInventorySlotsFromAny(payload.Slots)
	if len(slots) != 1 {
		t.Fatalf("expected payload to contain 1 slot, got %d", len(slots))
	}
	if ItemType(slots[0].Item.Type) != ItemTypeGold {
		t.Fatalf("expected slot item type %q, got %q", ItemTypeGold, slots[0].Item.Type)
	}
	if slots[0].Item.Quantity != 1 {
		t.Fatalf("expected slot quantity 1, got %d", slots[0].Item.Quantity)
	}
}

func TestWorldAttachTelemetryConfiguresEffectRegistryOverflow(t *testing.T) {
	w := legacyConstructWorld(worldpkg.DefaultConfig(), logging.NopPublisher{}, worldpkg.Deps{Publisher: logging.NopPublisher{}})
	if w == nil {
		t.Fatalf("expected world to construct")
	}
	if reg := w.effectRegistry(); reg.RecordSpatialOverflow != nil {
		t.Fatalf("expected overflow callback to be unset before telemetry attaches")
	}

	telemetry := newTelemetryCounters(nil)
	w.attachTelemetry(telemetry)

	reg := w.effectRegistry()
	if reg.RecordSpatialOverflow == nil {
		t.Fatalf("expected overflow callback after telemetry attaches")
	}

	reg.RecordSpatialOverflow("spark")

	snapshot := telemetry.effectsSpatialOverflow.snapshot()
	if got := snapshot["spark"]; got != 1 {
		t.Fatalf("expected overflow counter to increment, got %d", got)
	}
}
