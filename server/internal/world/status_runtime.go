package world

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	state "mine-and-die/server/internal/world/state"
	statuspkg "mine-and-die/server/internal/world/status"
)

const (
	// StatusEffectBurning identifies the burning status effect shared across
	// the internal world and legacy façade.
	StatusEffectBurning state.StatusEffectType = "burning"

	burningStatusEffectDuration = 3 * time.Second
	burningTickInterval         = 200 * time.Millisecond

	effectTypeBurningTick   = string(effectcontract.EffectIDBurningTick)
	effectTypeBurningVisual = string(effectcontract.EffectIDBurningVisual)
)

func (w *World) buildStatusEffectDefinitions() map[string]statuspkg.ApplyStatusEffectDefinition {
	return statuspkg.NewStatusEffectDefinitions(statuspkg.StatusEffectDefinitionsConfig{
		Burning: statuspkg.BurningStatusEffectDefinitionConfig{
			Type:               string(StatusEffectBurning),
			Duration:           burningStatusEffectDuration,
			TickInterval:       burningTickInterval,
			InitialTick:        true,
			FallbackAttachment: statuspkg.AttachStatusEffectVisual,
		},
	})
}
