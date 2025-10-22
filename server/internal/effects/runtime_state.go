package effects

// runtime_state.go centralizes helpers that coordinate runtime-managed effect
// state between contract hooks and the legacy world wrappers.

// runtimeRegistry returns the runtime's registry or an empty registry when the
// runtime is nil so callers can operate without additional guard clauses.
func runtimeRegistry(rt Runtime) Registry {
	if rt == nil {
		return Registry{}
	}
	return rt.Registry()
}

// RegisterRuntimeEffect records the provided effect in the runtime registry so
// legacy world lookups and spatial queries reuse the shared bookkeeping.
func RegisterRuntimeEffect(rt Runtime, effect *State) bool {
	if effect == nil {
		return false
	}
	return RegisterEffect(runtimeRegistry(rt), effect)
}

// UnregisterRuntimeEffect removes the provided effect from the runtime
// registry when it is no longer active.
func UnregisterRuntimeEffect(rt Runtime, effect *State) {
	if effect == nil {
		return
	}
	UnregisterEffect(runtimeRegistry(rt), effect)
}

// StoreRuntimeEffect caches the effect pointer on the runtime so subsequent
// hook invocations can reuse the same instance without a registry lookup.
func StoreRuntimeEffect(rt Runtime, id string, effect *State) {
	if rt == nil || id == "" {
		return
	}
	if effect == nil {
		rt.ClearInstanceState(id)
		return
	}
	rt.SetInstanceState(id, effect)
}

// LoadRuntimeEffect returns the effect associated with the provided runtime
// instance identifier. It first consults the runtime state cache and falls
// back to the registry when the cache is empty, caching the result for future
// callers.
func LoadRuntimeEffect(rt Runtime, id string) *State {
	if id == "" {
		return nil
	}
	if rt != nil {
		if value := rt.InstanceState(id); value != nil {
			if effect, ok := value.(*State); ok {
				return effect
			}
		}
	}
	effect := FindByID(runtimeRegistry(rt), id)
	if effect != nil && rt != nil {
		rt.SetInstanceState(id, effect)
	}
	return effect
}
