package world

import (
	"testing"
	"time"
)

type abilityOwnerStub struct {
	id string
}

type abilityActorStub struct {
	id string
}

func TestNewAbilityOwnerStateLookup(t *testing.T) {
	playerState := &abilityOwnerStub{id: "player-1"}
	playerCooldowns := map[string]time.Time{}
	npcState := &abilityOwnerStub{id: "npc-1"}
	npcCooldowns := map[string]time.Time{}

	lookup := NewAbilityOwnerStateLookup(AbilityOwnerStateLookupConfig[*abilityOwnerStub]{
		FindPlayer: func(actorID string) (*abilityOwnerStub, *map[string]time.Time, bool) {
			if actorID != playerState.id {
				return nil, nil, false
			}
			return playerState, &playerCooldowns, true
		},
		FindNPC: func(actorID string) (*abilityOwnerStub, *map[string]time.Time, bool) {
			if actorID != npcState.id {
				return nil, nil, false
			}
			return npcState, &npcCooldowns, true
		},
	})

	if lookup == nil {
		t.Fatalf("expected lookup function")
	}

	state, cooldowns, ok := lookup("player-1")
	if !ok || state != playerState || cooldowns == nil || cooldowns != &playerCooldowns {
		t.Fatalf("expected player lookup to return state and cooldowns")
	}

	state, cooldowns, ok = lookup("npc-1")
	if !ok || state != npcState || cooldowns == nil || cooldowns != &npcCooldowns {
		t.Fatalf("expected npc lookup to return state and cooldowns")
	}

	if _, _, ok := lookup("missing"); ok {
		t.Fatalf("expected missing actor to return ok=false")
	}
}

func TestNewAbilityOwnerLookup(t *testing.T) {
	cooldowns := map[string]time.Time{}
	snapshotState := &abilityOwnerStub{id: "actor-1"}
	snapshotOwner := &abilityActorStub{id: "actor-1"}

	lookup := NewAbilityOwnerLookup(AbilityOwnerLookupConfig[*abilityOwnerStub, abilityActorStub]{
		LookupState: func(actorID string) (*abilityOwnerStub, *map[string]time.Time, bool) {
			if actorID != snapshotState.id {
				return nil, nil, false
			}
			return snapshotState, &cooldowns, true
		},
		Snapshot: func(state *abilityOwnerStub) *abilityActorStub {
			if state == nil {
				return nil
			}
			return snapshotOwner
		},
	})

	if lookup == nil {
		t.Fatalf("expected lookup function")
	}

	owner, cd, ok := lookup("actor-1")
	if !ok || owner != snapshotOwner || cd == nil || cd != &cooldowns {
		t.Fatalf("expected owner lookup to return snapshot and cooldowns")
	}

	if _, _, ok := lookup("missing"); ok {
		t.Fatalf("expected missing actor to return ok=false")
	}
}

func TestAbilityOwnerLookupSnapshotFailure(t *testing.T) {
	lookup := NewAbilityOwnerLookup(AbilityOwnerLookupConfig[*abilityOwnerStub, abilityActorStub]{
		LookupState: func(actorID string) (*abilityOwnerStub, *map[string]time.Time, bool) {
			return &abilityOwnerStub{id: actorID}, &map[string]time.Time{}, true
		},
		Snapshot: func(state *abilityOwnerStub) *abilityActorStub {
			if state == nil || state.id == "missing" {
				return nil
			}
			return &abilityActorStub{id: state.id}
		},
	})

	if lookup == nil {
		t.Fatalf("expected lookup function")
	}

	if owner, _, ok := lookup("actor-1"); !ok || owner == nil || owner.id != "actor-1" {
		t.Fatalf("expected snapshot to succeed for actor-1")
	}

	if owner, _, ok := lookup("missing"); ok || owner != nil {
		t.Fatalf("expected snapshot failure to propagate")
	}
}

func TestNewAbilityGateConfig(t *testing.T) {
	cooldowns := map[string]time.Time{"ability": time.Now()}
	owner := &abilityActorStub{id: "owner"}
	lookupCalls := 0

	opts := AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{
		AbilityID: "ability",
		Cooldown:  500 * time.Millisecond,
		Lookup: func(actorID string) (*abilityActorStub, *map[string]time.Time, bool) {
			lookupCalls++
			if actorID != owner.id {
				return nil, nil, false
			}
			return owner, &cooldowns, true
		},
	}

	cfg, ok := NewMeleeAbilityGateConfig(opts)
	if !ok {
		t.Fatalf("expected melee gate config")
	}
	if cfg.AbilityID != opts.AbilityID || cfg.Cooldown != opts.Cooldown {
		t.Fatalf("expected config to forward ability metadata")
	}

	resolved, cd, ok := cfg.LookupOwner("owner")
	if !ok || resolved != owner || cd == nil || cd != &cooldowns || lookupCalls != 1 {
		t.Fatalf("expected lookup to delegate to underlying adapter")
	}

	if _, _, ok := cfg.LookupOwner("missing"); ok {
		t.Fatalf("expected missing actor to return ok=false")
	}

	if _, ok := NewProjectileAbilityGateConfig(AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{AbilityID: "", Lookup: opts.Lookup}); ok {
		t.Fatalf("expected empty ability id to fail")
	}
}

func TestAbilityOwnerStateLookupMissingAdapters(t *testing.T) {
	lookup := NewAbilityOwnerStateLookup(AbilityOwnerStateLookupConfig[*abilityOwnerStub]{})
	if lookup == nil {
		t.Fatalf("expected lookup function")
	}
	if _, _, ok := lookup("any"); ok {
		t.Fatalf("expected lookup to fail when adapters are missing")
	}
}

func TestAbilityGateConfigMissingLookup(t *testing.T) {
	cfg, ok := NewMeleeAbilityGateConfig(AbilityGateOptions[*abilityOwnerStub, abilityActorStub]{
		AbilityID: "ability",
		Cooldown:  time.Second,
	})
	if ok {
		t.Fatalf("expected config creation to fail without a lookup")
	}
	if cfg.LookupOwner != nil {
		t.Fatalf("expected lookup to be nil when config creation fails")
	}
}
