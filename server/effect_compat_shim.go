package main

import (
	"fmt"
	"time"
)

type legacyEffectCompat struct {
	world        *World
	mirroredByID map[string]string
}

func newLegacyEffectCompat(world *World) *legacyEffectCompat {
	if world == nil {
		return nil
	}
	return &legacyEffectCompat{
		world:        world,
		mirroredByID: make(map[string]string),
	}
}

func (c *legacyEffectCompat) mirrorAreaEffect(effect *effectState) {
	if c == nil || c.world == nil || effect == nil {
		return
	}
	if effect.contractManaged {
		return
	}
	if effect.ID == "" {
		return
	}
	if _, exists := c.mirroredByID[effect.ID]; exists {
		return
	}

	tick := Tick(int64(c.world.currentTick))
	compatID := c.compatID(effect.ID)
	geometry := EffectGeometry{
		Shape:  GeometryShapeRect,
		Width:  quantizeWorldCoord(effect.Effect.Width),
		Height: quantizeWorldCoord(effect.Effect.Height),
	}
	if geometry.Width == 0 {
		geometry.Width = quantizeWorldCoord(effect.Width)
	}
	if geometry.Height == 0 {
		geometry.Height = quantizeWorldCoord(effect.Height)
	}

	extra := copyFloatParams(effect.Effect.Params)
	if extra == nil {
		extra = make(map[string]int)
	}
	extra["centerX"] = quantizeWorldCoord(centerX(effect))
	extra["centerY"] = quantizeWorldCoord(centerY(effect))

	lifetime := time.Duration(effect.Duration) * time.Millisecond
	behavior := EffectBehaviorState{
		Extra: extra,
	}
	if lifetime > 0 {
		behavior.TicksRemaining = durationToTicks(lifetime)
	}

	instance := EffectInstance{
		ID:           compatID,
		DefinitionID: effect.Type,
		StartTick:    tick,
		DeliveryState: EffectDeliveryState{
			Geometry: geometry,
		},
		BehaviorState: behavior,
		OwnerActorID:  effect.Owner,
		Replication: ReplicationSpec{
			SendSpawn:   true,
			SendUpdates: false,
			SendEnd:     true,
		},
		End: EndPolicy{Kind: EndDuration},
	}

	spawn := EffectSpawnEvent{
		Tick:     tick,
		Instance: instance,
	}
	c.world.journal.RecordEffectSpawn(spawn)
	c.mirroredByID[effect.ID] = compatID
}

func (c *legacyEffectCompat) noteLegacyEffectEnd(effect *effectState, reason EndReason) {
	if c == nil || effect == nil || effect.ID == "" {
		return
	}
	compatID, ok := c.mirroredByID[effect.ID]
	if !ok {
		return
	}
	tick := Tick(int64(c.world.currentTick))
	end := EffectEndEvent{
		Tick:   tick,
		ID:     compatID,
		Reason: reason,
	}
	c.world.journal.RecordEffectEnd(end)
	delete(c.mirroredByID, effect.ID)
}

func (c *legacyEffectCompat) compatID(id string) string {
	if id == "" {
		return ""
	}
	return fmt.Sprintf("legacy-%s", id)
}
