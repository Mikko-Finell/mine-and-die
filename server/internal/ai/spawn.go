package ai

// SpawnBootstrapConfig captures the data required to initialise an NPC's AI
// state from a compiled library configuration.
type SpawnBootstrapConfig struct {
	Library       *Library
	Type          string
	ConfigID      *uint16
	State         *uint8
	Blackboard    *Blackboard
	WaypointCount int
}

type spawnDefaults struct {
	arriveRadius float64
	pauseTicks   uint64
	stuckEpsilon float64
	waypointMode waypointMode
}

type waypointMode uint8

const (
	waypointModeNone waypointMode = iota
	waypointModeClamp
	waypointModeZero
)

var spawnDefaultsByType = map[string]spawnDefaults{
	"goblin": {
		arriveRadius: 16,
		pauseTicks:   30,
		stuckEpsilon: 0.5,
		waypointMode: waypointModeClamp,
	},
	"rat": {
		arriveRadius: 10,
		pauseTicks:   20,
		stuckEpsilon: 0.5,
		waypointMode: waypointModeZero,
	},
}

// BootstrapNPC applies the compiled configuration and fallback defaults for the
// provided NPC type. When the library is nil or missing a configuration for the
// type, the helper still ensures blackboard defaults mirror the legacy spawn
// behaviour.
func BootstrapNPC(cfg SpawnBootstrapConfig) {
	if cfg.Blackboard == nil {
		return
	}

	if cfg.Library != nil {
		if compiled := cfg.Library.ConfigForType(cfg.Type); compiled != nil {
			if cfg.ConfigID != nil {
				*cfg.ConfigID = compiled.ID()
			}
			if cfg.State != nil {
				*cfg.State = compiled.InitialState()
			}
			compiled.ApplyDefaults(cfg.Blackboard)
		}
	}

	defaults := spawnDefaultsByType[cfg.Type]

	if cfg.Blackboard.ArriveRadius <= 0 && defaults.arriveRadius > 0 {
		cfg.Blackboard.ArriveRadius = defaults.arriveRadius
	}
	if cfg.Blackboard.PauseTicks == 0 && defaults.pauseTicks > 0 {
		cfg.Blackboard.PauseTicks = defaults.pauseTicks
	}
	if cfg.Blackboard.StuckEpsilon <= 0 && defaults.stuckEpsilon > 0 {
		cfg.Blackboard.StuckEpsilon = defaults.stuckEpsilon
	}

	switch defaults.waypointMode {
	case waypointModeZero:
		cfg.Blackboard.WaypointIndex = 0
	case waypointModeClamp:
		if cfg.WaypointCount <= 0 {
			cfg.Blackboard.WaypointIndex = 0
			break
		}
		if cfg.Blackboard.WaypointIndex < 0 || cfg.Blackboard.WaypointIndex >= cfg.WaypointCount {
			cfg.Blackboard.WaypointIndex = 0
		}
	}

	cfg.Blackboard.NextDecisionAt = 0
	cfg.Blackboard.LastWaypointIndex = -1
}
