package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// ContractBurningDamageHookConfig bundles the inputs required to apply lava
// damage for contract-managed burning status effects without reaching into the
// legacy world state directly.
type ContractBurningDamageHookConfig struct {
	StatusEffect    StatusEffectType
	DamagePerSecond float64
	TickInterval    time.Duration
	LookupActor     func(actorID string) *ContractStatusActor
}

// ContractBurningDamageHook returns the hook set that applies lava damage when a
// contract-managed burning status effect processes its tick.
func ContractBurningDamageHook(cfg ContractBurningDamageHookConfig) HookSet {
	return HookSet{
		OnSpawn: func(_ Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
			applyContractBurningDamage(cfg, instance, now)
		},
	}
}

func applyContractBurningDamage(cfg ContractBurningDamageHookConfig, instance *effectcontract.EffectInstance, now time.Time) {
	if instance == nil {
		return
	}

	actor := lookupContractStatusActor(cfg.LookupActor, instance)
	if actor == nil || actor.ApplyBurningDamage == nil {
		return
	}

	statusType := cfg.StatusEffect
	if actor.StatusInstance != nil && actor.StatusInstance.Instance != nil {
		if typ := actor.StatusInstance.Instance.DefinitionType(); typ != "" {
			statusType = StatusEffectType(typ)
		}
	}

	delta := contractBurningDamageDelta(cfg, instance)
	actor.ApplyBurningDamage(instance.OwnerActorID, statusType, delta, now)
}

func contractBurningDamageDelta(cfg ContractBurningDamageHookConfig, instance *effectcontract.EffectInstance) float64 {
	if instance != nil && instance.BehaviorState.Extra != nil {
		if value, ok := instance.BehaviorState.Extra["healthDelta"]; ok {
			delta := float64(value)
			if delta != 0 {
				return delta
			}
		}
	}

	if cfg.DamagePerSecond == 0 || cfg.TickInterval <= 0 {
		return 0
	}

	return -cfg.DamagePerSecond * cfg.TickInterval.Seconds()
}
