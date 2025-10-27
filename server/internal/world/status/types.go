package status

import runtime "mine-and-die/server/internal/effects/runtime"

// StatusEffectType aliases the runtime status effect identifier so world status
// helpers can share the same contract without importing legacy façade types.
type StatusEffectType = runtime.StatusEffectType
