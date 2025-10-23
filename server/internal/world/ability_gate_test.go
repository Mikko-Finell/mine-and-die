package world

import (
	"testing"
	"time"
)

func TestBindMeleeAbilityGate(t *testing.T) {
	cooldowns := map[string]time.Time{"ability": time.Now()}
	owner := &abilityActorStub{id: "owner"}

	opts := AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{
		AbilityID: "ability",
		Cooldown:  time.Second,
		Lookup: func(actorID string) (*abilityActorStub, *map[string]time.Time, bool) {
			if actorID != owner.id {
				return nil, nil, false
			}
			return owner, &cooldowns, true
		},
	}

	factoryCalls := 0
	gate, ok := BindMeleeAbilityGate(AbilityGateBindingOptions[*abilityOwnerStub, abilityActorStub, string]{
		AbilityGateOptions: opts,
		Factory: func(cfg AbilityGateConfig[abilityActorStub]) (string, bool) {
			factoryCalls++
			if cfg.AbilityID != opts.AbilityID || cfg.Cooldown != opts.Cooldown {
				t.Fatalf("expected ability metadata to match options")
			}

			resolved, cd, lookupOK := cfg.LookupOwner(owner.id)
			if !lookupOK || resolved != owner || cd == nil || cd != &cooldowns {
				t.Fatalf("expected lookup to forward owner and cooldowns")
			}

			if _, _, lookupOK = cfg.LookupOwner("missing"); lookupOK {
				t.Fatalf("expected missing owner to fail")
			}

			return "gate", true
		},
	})

	if !ok {
		t.Fatalf("expected gate construction to succeed")
	}
	if gate != "gate" {
		t.Fatalf("expected gate factory result")
	}
	if factoryCalls != 1 {
		t.Fatalf("expected factory to run exactly once")
	}
}

func TestBindMeleeAbilityGateFactoryFailure(t *testing.T) {
	_, ok := BindMeleeAbilityGate(AbilityGateBindingOptions[*abilityOwnerStub, abilityActorStub, string]{
		AbilityGateOptions: AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{
			AbilityID: "ability",
			Cooldown:  time.Second,
			Lookup: func(string) (*abilityActorStub, *map[string]time.Time, bool) {
				return nil, nil, false
			},
		},
		Factory: func(AbilityGateConfig[abilityActorStub]) (string, bool) {
			return "", false
		},
	})
	if ok {
		t.Fatalf("expected factory failure to propagate")
	}
}

func TestBindProjectileAbilityGateConfigFailure(t *testing.T) {
	factoryCalls := 0
	_, ok := BindProjectileAbilityGate(AbilityGateBindingOptions[*abilityOwnerStub, abilityActorStub, int]{
		AbilityGateOptions: AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{
			AbilityID: "",
			Cooldown:  time.Second,
			Lookup: func(actorID string) (*abilityActorStub, *map[string]time.Time, bool) {
				return nil, nil, true
			},
		},
		Factory: func(cfg AbilityGateConfig[abilityActorStub]) (int, bool) {
			factoryCalls++
			return 1, true
		},
	})
	if ok {
		t.Fatalf("expected config failure to propagate")
	}
	if factoryCalls != 0 {
		t.Fatalf("expected factory to be skipped on config failure")
	}
}

func TestBindProjectileAbilityGateMissingFactory(t *testing.T) {
	_, ok := BindProjectileAbilityGate(AbilityGateBindingOptions[*abilityOwnerStub, abilityActorStub, string]{
		AbilityGateOptions: AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{
			AbilityID: "ability",
			Cooldown:  time.Second,
			Lookup: func(string) (*abilityActorStub, *map[string]time.Time, bool) {
				return nil, nil, true
			},
		},
	})
	if ok {
		t.Fatalf("expected missing factory to fail")
	}
}
