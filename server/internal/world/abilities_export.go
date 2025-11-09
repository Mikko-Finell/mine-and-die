package world

import "mine-and-die/server/internal/world/abilities"

type (
	AbilityOwnerStateLookupConfig[State any]                  = abilities.AbilityOwnerStateLookupConfig[State]
	AbilityOwnerLookupConfig[State any, Owner any]            = abilities.AbilityOwnerLookupConfig[State, Owner]
	AbilityOwnerStateLookup[State any]                        = abilities.AbilityOwnerStateLookup[State]
	AbilityOwnerLookup[State any, Owner any]                  = abilities.AbilityOwnerLookup[State, Owner]
	AbilityGateConfig[Owner any]                              = abilities.AbilityGateConfig[Owner]
	AbilityGateOptions[State any, Owner any]                  = abilities.AbilityGateOptions[State, Owner]
	AbilityGateFactory[Owner any, Gate any]                   = abilities.AbilityGateFactory[Owner, Gate]
	AbilityGateBindingOptions[State any, Owner any, Gate any] = abilities.AbilityGateBindingOptions[State, Owner, Gate]
	WorldAbilityGateOptions                                   = abilities.WorldAbilityGateOptions
)

func NewAbilityOwnerStateLookup[State any](cfg abilities.AbilityOwnerStateLookupConfig[State]) abilities.AbilityOwnerStateLookup[State] {
	return abilities.NewAbilityOwnerStateLookup(cfg)
}

func NewAbilityOwnerLookup[State any, Owner any](cfg abilities.AbilityOwnerLookupConfig[State, Owner]) abilities.AbilityOwnerLookup[State, Owner] {
	return abilities.NewAbilityOwnerLookup(cfg)
}

func NewMeleeAbilityGateConfig[State any, Owner any](opts abilities.AbilityGateOptions[State, Owner]) (abilities.AbilityGateConfig[Owner], bool) {
	return abilities.NewMeleeAbilityGateConfig(opts)
}

func NewProjectileAbilityGateConfig[State any, Owner any](opts abilities.AbilityGateOptions[State, Owner]) (abilities.AbilityGateConfig[Owner], bool) {
	return abilities.NewProjectileAbilityGateConfig(opts)
}

func BindMeleeAbilityGate[State any, Owner any, Gate any](opts abilities.AbilityGateBindingOptions[State, Owner, Gate]) (Gate, bool) {
	return abilities.BindMeleeAbilityGate(opts)
}

func BindProjectileAbilityGate[State any, Owner any, Gate any](opts abilities.AbilityGateBindingOptions[State, Owner, Gate]) (Gate, bool) {
	return abilities.BindProjectileAbilityGate(opts)
}
