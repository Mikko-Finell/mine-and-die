package state

import (
	"time"

	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
)

type StatusEffectType = internaleffects.StatusEffectType

type StatusEffectInstance struct {
	Definition     *worldpkg.StatusEffectDefinition
	SourceID       string
	AppliedAt      time.Time
	ExpiresAt      time.Time
	NextTick       time.Time
	LastTick       time.Time
	attachedEffect *internaleffects.State
	actor          *ActorState
}

func (inst *StatusEffectInstance) AttachEffect(value any) {
	if inst == nil {
		return
	}
	eff, ok := value.(*internaleffects.State)
	if !ok || eff == nil {
		return
	}
	inst.attachedEffect = eff
}

func (inst *StatusEffectInstance) DefinitionType() string {
	if inst == nil || inst.Definition == nil {
		return ""
	}
	return inst.Definition.Type
}

func (inst *StatusEffectInstance) AttachedEffect() *internaleffects.State {
	if inst == nil {
		return nil
	}
	return inst.attachedEffect
}

func (inst *StatusEffectInstance) ActorState() *ActorState {
	if inst == nil {
		return nil
	}
	return inst.actor
}

func (inst *StatusEffectInstance) SetActorState(state *ActorState) {
	if inst == nil {
		return
	}
	inst.actor = state
}

func (inst *StatusEffectInstance) ClearAttachedEffect() {
	if inst == nil {
		return
	}
	inst.attachedEffect = nil
}
