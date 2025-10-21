package world

import "testing"

type statusEffectInstanceStub struct {
	attached any
	typ      string
}

func (s *statusEffectInstanceStub) AttachEffect(value any) {
	s.attached = value
}

func (s *statusEffectInstanceStub) DefinitionType() string {
	return s.typ
}

type statusEffectVisualStub struct {
	state  any
	status string
}

func (s *statusEffectVisualStub) SetStatusEffect(value string) {
	s.status = value
}

func (s *statusEffectVisualStub) EffectState() any {
	return s.state
}

func TestAttachStatusEffectVisualUsesInstanceType(t *testing.T) {
	instance := &statusEffectInstanceStub{typ: "poison"}
	state := struct{}{}
	visual := &statusEffectVisualStub{state: &state}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Instance:    instance,
		Effect:      visual,
		DefaultType: "burning",
	})

	if instance.attached != visual.state {
		t.Fatalf("expected instance to attach provided effect state")
	}
	if visual.status != "poison" {
		t.Fatalf("expected visual status to match instance type, got %q", visual.status)
	}
}

func TestAttachStatusEffectVisualFallsBackToDefault(t *testing.T) {
	instance := &statusEffectInstanceStub{}
	state := struct{}{}
	visual := &statusEffectVisualStub{state: &state}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Instance:    instance,
		Effect:      visual,
		DefaultType: "burning",
	})

	if visual.status != "burning" {
		t.Fatalf("expected fallback status effect 'burning', got %q", visual.status)
	}
}

func TestAttachStatusEffectVisualNoopWhenStateMissing(t *testing.T) {
	instance := &statusEffectInstanceStub{typ: "burning"}
	visual := &statusEffectVisualStub{}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Instance:    instance,
		Effect:      visual,
		DefaultType: "burning",
	})

	if instance.attached != nil {
		t.Fatalf("expected instance attachment to remain nil when state missing")
	}
	if visual.status != "" {
		t.Fatalf("expected visual status to remain empty when state missing")
	}
}

func TestAttachStatusEffectVisualNoopWhenInstanceMissing(t *testing.T) {
	state := struct{}{}
	visual := &statusEffectVisualStub{state: &state}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Effect:      visual,
		DefaultType: "burning",
	})

	if visual.status != "" {
		t.Fatalf("expected visual status to remain empty when instance missing")
	}
}
