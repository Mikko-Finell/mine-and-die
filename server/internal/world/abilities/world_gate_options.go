package abilities

import (
	effectcontract "mine-and-die/server/effects/contract"
	state "mine-and-die/server/internal/world/state"
)

// WorldAbilityGateOptions bundles the pre-bound gate options used by the legacy
// server integration. Callers pass the returned structs into the shared gate
// binders so they do not need to rewire ability owner lookups themselves.
type WorldAbilityGateOptions struct {
	Melee      AbilityGateOptions[*state.ActorState, ActorSnapshot]
	Projectile AbilityGateOptions[*state.ActorState, ActorSnapshot]
}

// NewWorldAbilityGateOptions constructs the default melee and projectile gate
// options for the provided ability owner lookup. The helper keeps the cooldown
// metadata and snapshot conversion colocated with the internal owner code so
// façade callers can bind combat gates without duplicating configuration.
func NewWorldAbilityGateOptions(
	lookup AbilityOwnerLookup[*state.ActorState, ActorSnapshot],
) (WorldAbilityGateOptions, bool) {
	if lookup == nil {
		return WorldAbilityGateOptions{}, false
	}

	return WorldAbilityGateOptions{
		Melee: AbilityGateOptions[*state.ActorState, ActorSnapshot]{
			AbilityID: effectcontract.EffectIDAttack,
			Cooldown:  MeleeAttackCooldown,
			Lookup:    lookup,
		},
		Projectile: AbilityGateOptions[*state.ActorState, ActorSnapshot]{
			AbilityID: effectcontract.EffectIDFireball,
			Cooldown:  FireballCooldown,
			Lookup:    lookup,
		},
	}, true
}
