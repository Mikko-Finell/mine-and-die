package combat

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// ProjectileAbilityTriggerConfig bundles the adapters required to stage a
// projectile ability intent without importing the legacy server package.
type ProjectileAbilityTriggerConfig struct {
	AbilityGate  ProjectileAbilityGate
	IntentConfig ProjectileIntentConfig
	Template     ProjectileIntentTemplate
}

// StageProjectileIntent applies the provided projectile ability gate and
// template to return a contract intent ready for enqueueing. Callers supply the
// actor identifier and current wall-clock time alongside the staged owner so the
// helper can remain agnostic of legacy world types.
func StageProjectileIntent(cfg ProjectileAbilityTriggerConfig, actorID string, now time.Time) (effectcontract.EffectIntent, bool) {
	if cfg.AbilityGate == nil {
		return effectcontract.EffectIntent{}, false
	}
	if cfg.Template.Type == "" {
		return effectcontract.EffectIntent{}, false
	}

	owner, ok := cfg.AbilityGate(actorID, now)
	if !ok {
		return effectcontract.EffectIntent{}, false
	}

	return NewProjectileIntent(cfg.IntentConfig, owner, cfg.Template)
}
