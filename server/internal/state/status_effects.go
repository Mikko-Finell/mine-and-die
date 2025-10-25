package state

import (
	"time"

	internaleffects "mine-and-die/server/internal/effects"
)

type StatusEffectType = internaleffects.StatusEffectType

type StatusEffectDefinition struct {
	Type         string
	TickInterval time.Duration
	Runtime      any
}

type StatusEffectInstance struct {
	Definition     *StatusEffectDefinition
	SourceID       string
	AppliedAt      time.Time
	ExpiresAt      time.Time
	NextTick       time.Time
	LastTick       time.Time
	AttachedEffect any
	Actor          any
}

func (inst *StatusEffectInstance) AttachEffect(value any) {
	if inst == nil {
		return
	}
	inst.AttachedEffect = value
}

func (inst *StatusEffectInstance) DefinitionType() string {
	if inst == nil || inst.Definition == nil {
		return ""
	}
	return inst.Definition.Type
}
