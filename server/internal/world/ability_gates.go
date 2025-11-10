package world

import (
	"time"

	combat "mine-and-die/server/internal/combat"
	abilitiespkg "mine-and-die/server/internal/world/abilities"
	state "mine-and-die/server/internal/world/state"
)

// AbilityGateBindings bundles the melee and projectile gates bound to the
// world's ability owner lookup.
type AbilityGateBindings struct {
	Melee      combat.MeleeAbilityGate
	Projectile combat.ProjectileAbilityGate
}

// AbilityGates constructs the melee and projectile gates using the world's
// ability owner lookup. Callers use the returned bindings to install combat
// gates without duplicating façade glue.
func (w *World) AbilityGates() (AbilityGateBindings, bool) {
	var zero AbilityGateBindings
	if w == nil {
		return zero, false
	}

	options, ok := w.AbilityGateOptions()
	if !ok {
		return zero, false
	}

	melee, ok := w.BindMeleeAbilityGate(options.Melee)
	if !ok {
		return zero, false
	}

	projectile, ok := w.BindProjectileAbilityGate(options.Projectile)
	if !ok {
		return zero, false
	}

	return AbilityGateBindings{
		Melee:      melee,
		Projectile: projectile,
	}, true
}

// BindMeleeAbilityGate constructs a melee gate using the provided options.
func (w *World) BindMeleeAbilityGate(opts abilitiespkg.AbilityGateOptions[*state.ActorState, AbilityActorSnapshot]) (combat.MeleeAbilityGate, bool) {
	if w == nil {
		return nil, false
	}
	return bindMeleeAbilityGate(opts)
}

// BindProjectileAbilityGate constructs a projectile gate using the provided options.
func (w *World) BindProjectileAbilityGate(opts abilitiespkg.AbilityGateOptions[*state.ActorState, AbilityActorSnapshot]) (combat.ProjectileAbilityGate, bool) {
	if w == nil {
		return nil, false
	}
	return bindProjectileAbilityGate(opts)
}

func bindMeleeAbilityGate(
	opts abilitiespkg.AbilityGateOptions[*state.ActorState, AbilityActorSnapshot],
) (combat.MeleeAbilityGate, bool) {
	gate, ok := abilitiespkg.BindMeleeAbilityGate(abilitiespkg.AbilityGateBindingOptions[*state.ActorState, AbilityActorSnapshot, combat.MeleeAbilityGate]{
		AbilityGateOptions: opts,
		Factory: func(cfg abilitiespkg.AbilityGateConfig[AbilityActorSnapshot]) (combat.MeleeAbilityGate, bool) {
			constructed := combat.NewMeleeAbilityGate(combat.MeleeAbilityGateConfig{
				AbilityID:   cfg.AbilityID,
				Cooldown:    cfg.Cooldown,
				LookupOwner: wrapAbilityOwnerLookup(cfg.LookupOwner),
			})
			if constructed == nil {
				return nil, false
			}
			return constructed, true
		},
	})
	if !ok {
		return nil, false
	}
	return gate, true
}

func bindProjectileAbilityGate(
	opts abilitiespkg.AbilityGateOptions[*state.ActorState, AbilityActorSnapshot],
) (combat.ProjectileAbilityGate, bool) {
	gate, ok := abilitiespkg.BindProjectileAbilityGate(abilitiespkg.AbilityGateBindingOptions[*state.ActorState, AbilityActorSnapshot, combat.ProjectileAbilityGate]{
		AbilityGateOptions: opts,
		Factory: func(cfg abilitiespkg.AbilityGateConfig[AbilityActorSnapshot]) (combat.ProjectileAbilityGate, bool) {
			constructed := combat.NewProjectileAbilityGate(combat.ProjectileAbilityGateConfig{
				AbilityID:   cfg.AbilityID,
				Cooldown:    cfg.Cooldown,
				LookupOwner: wrapAbilityOwnerLookup(cfg.LookupOwner),
			})
			if constructed == nil {
				return nil, false
			}
			return constructed, true
		},
	})
	if !ok {
		return nil, false
	}
	return gate, true
}

func wrapAbilityOwnerLookup(
	lookup func(string) (*AbilityActorSnapshot, *map[string]time.Time, bool),
) func(string) (*combat.AbilityActor, *map[string]time.Time, bool) {
	if lookup == nil {
		return nil
	}
	return func(actorID string) (*combat.AbilityActor, *map[string]time.Time, bool) {
		owner, cooldowns, ok := lookup(actorID)
		if !ok || owner == nil {
			return nil, cooldowns, false
		}
		converted := abilityActorFromSnapshot(owner)
		if converted == nil {
			return nil, cooldowns, false
		}
		return converted, cooldowns, true
	}
}

func abilityActorFromSnapshot(snapshot *AbilityActorSnapshot) *combat.AbilityActor {
	if snapshot == nil {
		return nil
	}
	return &combat.AbilityActor{
		ID:     snapshot.ID,
		X:      snapshot.X,
		Y:      snapshot.Y,
		Facing: snapshot.Facing,
	}
}
