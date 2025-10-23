package effects

import "mine-and-die/server/internal/sim"

// Trigger represents a one-shot visual instruction that the client may
// execute without additional server updates.
type Trigger struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Start    int64              `json:"start,omitempty"`
	Duration int64              `json:"duration,omitempty"`
	X        float64            `json:"x,omitempty"`
	Y        float64            `json:"y,omitempty"`
	Width    float64            `json:"width,omitempty"`
	Height   float64            `json:"height,omitempty"`
	Params   map[string]float64 `json:"params,omitempty"`
	Colors   []string           `json:"colors,omitempty"`
}

// SimEffectTriggersFromLegacy converts legacy effect triggers into their
// simulation equivalents, cloning any map or slice fields so callers receive
// independent data structures.
func SimEffectTriggersFromLegacy(triggers []Trigger) []sim.EffectTrigger {
	if len(triggers) == 0 {
		return nil
	}
	converted := make([]sim.EffectTrigger, len(triggers))
	for i, trigger := range triggers {
		converted[i] = sim.EffectTrigger{
			ID:       trigger.ID,
			Type:     trigger.Type,
			Start:    trigger.Start,
			Duration: trigger.Duration,
			X:        trigger.X,
			Y:        trigger.Y,
			Width:    trigger.Width,
			Height:   trigger.Height,
			Params:   cloneFloatMap(trigger.Params),
			Colors:   cloneStringSlice(trigger.Colors),
		}
	}
	return converted
}

// LegacyEffectTriggersFromSim converts simulation effect triggers into their
// legacy equivalents, cloning any map or slice fields so callers receive
// independent data structures.
func LegacyEffectTriggersFromSim(triggers []sim.EffectTrigger) []Trigger {
	if len(triggers) == 0 {
		return nil
	}
	converted := make([]Trigger, len(triggers))
	for i, trigger := range triggers {
		converted[i] = Trigger{
			ID:       trigger.ID,
			Type:     trigger.Type,
			Start:    trigger.Start,
			Duration: trigger.Duration,
			X:        trigger.X,
			Y:        trigger.Y,
			Width:    trigger.Width,
			Height:   trigger.Height,
			Params:   cloneFloatMap(trigger.Params),
			Colors:   cloneStringSlice(trigger.Colors),
		}
	}
	return converted
}
