package effects

import runtime "mine-and-die/server/internal/effects/runtime"

type SpatialIndex = runtime.SpatialIndex

var (
	DefaultSpatialCellSize   = runtime.DefaultSpatialCellSize
	DefaultSpatialMaxPerCell = runtime.DefaultSpatialMaxPerCell
	NewSpatialIndex          = runtime.NewSpatialIndex
	RegisterEffect           = runtime.RegisterEffect
)

func runtimeRegistry(rt Runtime) runtime.Registry {
	if rt == nil {
		return runtime.Registry{}
	}
	return rt.Registry()
}

func RegisterRuntimeEffect(rt Runtime, effect *State) bool {
	if effect == nil {
		return false
	}
	return runtime.RegisterEffect(runtimeRegistry(rt), effect)
}

func UnregisterRuntimeEffect(rt Runtime, effect *State) {
	if effect == nil {
		return
	}
	runtime.UnregisterEffect(runtimeRegistry(rt), effect)
}

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
	effect := runtime.FindByID(runtimeRegistry(rt), id)
	if effect != nil && rt != nil {
		rt.SetInstanceState(id, effect)
	}
	return effect
}
