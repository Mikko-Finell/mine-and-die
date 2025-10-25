package state

import "time"

type StatusEffectType string

type StatusEffectInstance struct {
	Definition     any
	SourceID       string
	AppliedAt      time.Time
	ExpiresAt      time.Time
	NextTick       time.Time
	LastTick       time.Time
	attachedEffect any
	actor          *ActorState
}

func (inst *StatusEffectInstance) AttachEffect(value any) {
	if inst == nil || value == nil {
		return
	}
	inst.attachedEffect = value
}

func (inst *StatusEffectInstance) DefinitionType() string {
	if inst == nil {
		return ""
	}
	if provider, ok := inst.Definition.(interface{ StatusEffectType() string }); ok && provider != nil {
		return provider.StatusEffectType()
	}
	return ""
}

func (inst *StatusEffectInstance) AttachedEffect() any {
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
