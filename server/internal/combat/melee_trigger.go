package combat

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// MeleeAbilityTriggerConfig bundles the adapters required to stage a melee
// ability intent without importing the legacy server package.
type MeleeAbilityTriggerConfig struct {
	AbilityGate  MeleeAbilityGate
	IntentConfig MeleeIntentConfig
}

// StageMeleeIntent applies the provided melee ability gate and intent
// configuration to return a contract intent ready for enqueueing. Callers
// supply the actor identifier and current wall-clock time alongside the staged
// owner so the helper can remain agnostic of legacy world types.
func StageMeleeIntent(cfg MeleeAbilityTriggerConfig, actorID string, now time.Time) (effectcontract.EffectIntent, bool) {
	if cfg.AbilityGate == nil {
		return effectcontract.EffectIntent{}, false
	}

	owner, ok := cfg.AbilityGate(actorID, now)
	if !ok {
		return effectcontract.EffectIntent{}, false
	}

	return NewMeleeIntent(cfg.IntentConfig, owner)
}
