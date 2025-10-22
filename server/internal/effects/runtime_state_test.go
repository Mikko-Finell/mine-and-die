package effects

import "testing"

type runtimeStub struct {
	state    map[string]any
	registry Registry
}

func (s *runtimeStub) InstanceState(id string) any {
	if s == nil || id == "" {
		return nil
	}
	return s.state[id]
}

func (s *runtimeStub) SetInstanceState(id string, value any) {
	if s == nil || id == "" {
		return
	}
	if s.state == nil {
		s.state = make(map[string]any)
	}
	s.state[id] = value
}

func (s *runtimeStub) ClearInstanceState(id string) {
	if s == nil || id == "" || s.state == nil {
		return
	}
	delete(s.state, id)
}

func (s *runtimeStub) Registry() Registry {
	if s == nil {
		return Registry{}
	}
	return s.registry
}

func TestRegisterRuntimeEffectAppendsToRegistry(t *testing.T) {
	effects := make([]*State, 0)
	byID := make(map[string]*State)
	stub := &runtimeStub{
		registry: Registry{Effects: &effects, ByID: &byID},
	}
	effect := &State{ID: "effect-runtime-register", Type: "spark"}

	if !RegisterRuntimeEffect(stub, effect) {
		t.Fatalf("RegisterRuntimeEffect returned false")
	}
	if len(effects) != 1 || effects[0] != effect {
		t.Fatalf("expected effect appended, got %#v", effects)
	}
	if got := byID[effect.ID]; got != effect {
		t.Fatalf("expected map entry for %q, got %#v", effect.ID, got)
	}
}

func TestUnregisterRuntimeEffectRemovesMapEntry(t *testing.T) {
	effects := make([]*State, 0)
	byID := make(map[string]*State)
	stub := &runtimeStub{
		registry: Registry{Effects: &effects, ByID: &byID},
	}
	effect := &State{ID: "effect-runtime-unregister", Type: "spark"}
	if !RegisterEffect(stub.registry, effect) {
		t.Fatalf("expected registration to succeed")
	}

	UnregisterRuntimeEffect(stub, effect)
	if _, exists := byID[effect.ID]; exists {
		t.Fatalf("expected map entry removed")
	}
}

func TestStoreRuntimeEffectCachesAndClearsState(t *testing.T) {
	stub := &runtimeStub{}
	effect := &State{ID: "effect-runtime-store"}

	StoreRuntimeEffect(stub, effect.ID, effect)
	if got := stub.state[effect.ID]; got != effect {
		t.Fatalf("expected effect cached, got %#v", got)
	}

	StoreRuntimeEffect(stub, effect.ID, nil)
	if _, exists := stub.state[effect.ID]; exists {
		t.Fatalf("expected cache entry cleared")
	}

	// Ensure nil runtime does not panic.
	StoreRuntimeEffect(nil, effect.ID, effect)
}

func TestLoadRuntimeEffectPrefersRuntimeCache(t *testing.T) {
	stub := &runtimeStub{state: map[string]any{}}
	cached := &State{ID: "effect-runtime-cache"}
	stub.state[cached.ID] = cached

	if got := LoadRuntimeEffect(stub, cached.ID); got != cached {
		t.Fatalf("expected cached effect, got %#v", got)
	}
}

func TestLoadRuntimeEffectFallsBackToRegistryAndCaches(t *testing.T) {
	effects := make([]*State, 0)
	byID := make(map[string]*State)
	stub := &runtimeStub{
		registry: Registry{Effects: &effects, ByID: &byID},
	}
	effect := &State{ID: "effect-runtime-fallback", Type: "spark"}
	if !RegisterEffect(stub.registry, effect) {
		t.Fatalf("expected registration to succeed")
	}

	if got := LoadRuntimeEffect(stub, effect.ID); got != effect {
		t.Fatalf("expected registry effect, got %#v", got)
	}
	if cached := stub.state[effect.ID]; cached != effect {
		t.Fatalf("expected registry effect cached, got %#v", cached)
	}
}
