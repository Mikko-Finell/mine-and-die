package ai

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	combat "mine-and-die/server/internal/combat"
)

//go:embed configs/*.json
var embeddedConfigs embed.FS

// GlobalLibrary provides the default authoring configs bundled with the server.
var GlobalLibrary = MustLoadLibrary()

// Library stores compiled AI configurations indexed by NPC type and numeric ID.
type Library struct {
	configsByID   map[uint16]*CompiledConfig
	configsByType map[string]*CompiledConfig
	nextID        uint16
}

// CompiledConfig captures the runtime state machine produced from an authoring
// configuration.
type CompiledConfig struct {
	id           uint16
	initialState uint8
	states       []compiledState
	stateNames   []string
	defaults     blackboardDefaults

	moveTowardParams        []moveTowardParams
	useAbilityParams        []useAbilityParams
	faceParams              []faceParams
	setTimerParams          []setTimerParams
	setWaypointParams       []setWaypointParams
	randomDestinationParams []randomDestinationParams
	moveAwayParams          []moveAwayParams
	reachedWaypointParams   []reachedWaypointParams
	playerWithinParams      []playerWithinParams
	lostSightParams         []lostSightParams
	cooldownReadyParams     []cooldownReadyParams
	stuckParams             []stuckParams
	actorWithinParams       []actorWithinParams
}

type compiledState struct {
	cadence     uint16
	enterTimer  uint16
	actions     []compiledAction
	transitions []compiledTransition
}

type compiledAction struct {
	id         actionID
	paramIndex uint16
}

type compiledTransition struct {
	conditionID conditionID
	paramIndex  uint16
	toState     uint8
}

type blackboardDefaults struct {
	WaypointIndex int
	ArriveRadius  float64
	PauseTicks    uint64
	PatrolSpeed   float64
	StuckEpsilon  float64
}

type moveTowardParams struct {
	Target moveTarget
	Vector Vec2
}

type useAbilityParams struct {
	Ability AbilityID
}

type faceParams struct {
	Target moveTarget
}

type setTimerParams struct {
	Duration uint16
}

type setWaypointParams struct {
	Index   int
	Advance bool
}

type reachedWaypointParams struct {
	ArriveRadius float64
}

type playerWithinParams struct {
	Radius float64
}

type actorWithinParams struct {
	Radius float64
}

type lostSightParams struct {
	Distance float64
}

type cooldownReadyParams struct {
	Ability AbilityID
}

type stuckParams struct {
	Decisions uint8
	Epsilon   float64
}

type randomDestinationParams struct {
	Radius    float64
	MinRadius float64
}

type moveAwayParams struct {
	Distance    float64
	MinDistance float64
}

type moveTarget uint8

type actionID uint8

type conditionID uint8

// AbilityID enumerates the abilities referenced by the AI authoring configs.
type AbilityID uint8

const (
	// AbilityNone represents an action-free transition.
	AbilityNone AbilityID = iota
	// AbilityAttack maps to the melee attack command.
	AbilityAttack
	// AbilityFireball maps to the ranged fireball ability.
	AbilityFireball
)

const (
	moveTargetWaypoint moveTarget = iota
	moveTargetPlayer
	moveTargetVector
)

const (
	actionIDMoveToward actionID = iota
	actionIDStop
	actionIDUseAbility
	actionIDFace
	actionIDSetTimer
	actionIDSetWaypoint
	actionIDSetRandomDestination
	actionIDMoveAway
)

const (
	conditionReachedWaypoint conditionID = iota
	conditionTimerExpired
	conditionPlayerWithin
	conditionLostSight
	conditionCooldownReady
	conditionStuck
	conditionNonRatWithin
)

// MustLoadLibrary loads the embedded authoring configs or panics on failure.
func MustLoadLibrary() *Library {
	lib, err := LoadLibrary()
	if err != nil {
		panic(fmt.Errorf("ai: load library: %w", err))
	}
	return lib
}

// LoadLibrary loads the embedded authoring configs and compiles them into a
// runtime library instance.
func LoadLibrary() (*Library, error) {
	lib := &Library{
		configsByID:   make(map[uint16]*CompiledConfig),
		configsByType: make(map[string]*CompiledConfig),
	}

	entries, err := fs.ReadDir(embeddedConfigs, "configs")
	if err != nil {
		return nil, fmt.Errorf("ai: read configs: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(embeddedConfigs, "configs/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("ai: read %q: %w", entry.Name(), err)
		}
		var authoring authoringConfig
		if err := json.Unmarshal(data, &authoring); err != nil {
			return nil, fmt.Errorf("ai: decode %q: %w", entry.Name(), err)
		}
		compiled, err := compileConfig(lib.allocateID(), authoring)
		if err != nil {
			return nil, fmt.Errorf("ai: compile %q: %w", entry.Name(), err)
		}
		lib.configsByID[compiled.id] = compiled
		npcType := strings.TrimSpace(strings.ToLower(authoring.NPCType))
		lib.configsByType[npcType] = compiled
	}

	return lib, nil
}

func (l *Library) allocateID() uint16 {
	l.nextID++
	if l.nextID == 0 {
		l.nextID++
	}
	return l.nextID
}

// ConfigForType retrieves the compiled configuration for the provided NPC type.
func (l *Library) ConfigForType(t string) *CompiledConfig {
	if l == nil {
		return nil
	}
	key := strings.TrimSpace(strings.ToLower(t))
	return l.configsByType[key]
}

// ConfigByID retrieves the compiled configuration for the provided numeric ID.
func (l *Library) ConfigByID(id uint16) *CompiledConfig {
	if l == nil {
		return nil
	}
	return l.configsByID[id]
}

// ID exposes the numeric identifier assigned to the compiled configuration.
func (cfg *CompiledConfig) ID() uint16 {
	if cfg == nil {
		return 0
	}
	return cfg.id
}

// InitialState returns the default state index for the compiled configuration.
func (cfg *CompiledConfig) InitialState() uint8 {
	if cfg == nil {
		return 0
	}
	return cfg.initialState
}

// ApplyDefaults applies authoring blackboard defaults to the provided instance.
func (cfg *CompiledConfig) ApplyDefaults(bb *Blackboard) {
	if bb == nil || cfg == nil {
		return
	}
	bb.WaypointIndex = cfg.defaults.WaypointIndex
	if cfg.defaults.ArriveRadius > 0 {
		bb.ArriveRadius = cfg.defaults.ArriveRadius
	}
	if cfg.defaults.PauseTicks > 0 {
		bb.PauseTicks = cfg.defaults.PauseTicks
	}
	if cfg.defaults.PatrolSpeed > 0 {
		bb.PatrolSpeed = cfg.defaults.PatrolSpeed
	}
	if cfg.defaults.StuckEpsilon > 0 {
		bb.StuckEpsilon = cfg.defaults.StuckEpsilon
	} else if bb.StuckEpsilon <= 0 {
		bb.StuckEpsilon = 0.5
	}
}

// StateName returns the human-readable name for the provided state index.
func (cfg *CompiledConfig) StateName(id uint8) string {
	if cfg == nil {
		return ""
	}
	if int(id) >= len(cfg.stateNames) {
		return ""
	}
	return cfg.stateNames[id]
}

// StateNames returns a copy of the compiled state's human-readable names.
func (cfg *CompiledConfig) StateNames() []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, len(cfg.stateNames))
	copy(out, cfg.stateNames)
	return out
}

func compileConfig(id uint16, authoring authoringConfig) (*CompiledConfig, error) {
	compiled := &CompiledConfig{
		id:                      id,
		stateNames:              make([]string, 0, len(authoring.States)),
		states:                  make([]compiledState, 0, len(authoring.States)),
		moveTowardParams:        make([]moveTowardParams, 0),
		useAbilityParams:        make([]useAbilityParams, 0),
		faceParams:              make([]faceParams, 0),
		setTimerParams:          make([]setTimerParams, 0),
		setWaypointParams:       make([]setWaypointParams, 0),
		randomDestinationParams: make([]randomDestinationParams, 0),
		moveAwayParams:          make([]moveAwayParams, 0),
		reachedWaypointParams:   make([]reachedWaypointParams, 0),
		playerWithinParams:      make([]playerWithinParams, 0),
		lostSightParams:         make([]lostSightParams, 0),
		cooldownReadyParams:     make([]cooldownReadyParams, 0),
		stuckParams:             make([]stuckParams, 0),
		actorWithinParams:       make([]actorWithinParams, 0),
	}

	compiled.defaults = blackboardDefaults{
		WaypointIndex: authoring.BlackboardDefaults.WaypointIndex,
		ArriveRadius:  authoring.BlackboardDefaults.ArriveRadius,
		PauseTicks:    authoring.BlackboardDefaults.PauseTicks,
		PatrolSpeed:   authoring.BlackboardDefaults.PatrolSpeed,
		StuckEpsilon:  authoring.BlackboardDefaults.StuckEpsilon,
	}

	stateIndex := make(map[string]int)
	for idx, state := range authoring.States {
		name := strings.TrimSpace(state.ID)
		if name == "" {
			return nil, fmt.Errorf("state %d missing id", idx)
		}
		lower := strings.ToLower(name)
		if _, exists := stateIndex[lower]; exists {
			return nil, fmt.Errorf("duplicate state %q", name)
		}
		stateIndex[lower] = idx
		compiled.stateNames = append(compiled.stateNames, name)
	}

	if len(compiled.stateNames) == 0 {
		return nil, fmt.Errorf("no states defined")
	}

	compiled.initialState = 0

	for _, state := range authoring.States {
		compiledState := compiledState{
			cadence:     state.TickEvery,
			enterTimer:  state.DurationTicks,
			actions:     make([]compiledAction, 0, len(state.Actions)),
			transitions: make([]compiledTransition, 0, len(state.Transitions)),
		}

		for _, action := range state.Actions {
			id, err := parseActionID(action.Name)
			if err != nil {
				return nil, fmt.Errorf("state %q action: %w", state.ID, err)
			}
			compiledAction := compiledAction{id: id}
			switch id {
			case actionIDMoveToward:
				target := parseMoveTarget(action.Target)
				params := moveTowardParams{Target: target}
				if action.Vector != nil {
					params.Vector = Vec2{X: action.Vector.X, Y: action.Vector.Y}
				}
				compiled.moveTowardParams = append(compiled.moveTowardParams, params)
				compiledAction.paramIndex = uint16(len(compiled.moveTowardParams) - 1)
			case actionIDUseAbility:
				ability, err := parseAbilityID(action.Ability)
				if err != nil {
					return nil, fmt.Errorf("state %q ability: %w", state.ID, err)
				}
				compiled.useAbilityParams = append(compiled.useAbilityParams, useAbilityParams{Ability: ability})
				compiledAction.paramIndex = uint16(len(compiled.useAbilityParams) - 1)
			case actionIDFace:
				compiled.faceParams = append(compiled.faceParams, faceParams{Target: parseMoveTarget(action.Target)})
				compiledAction.paramIndex = uint16(len(compiled.faceParams) - 1)
			case actionIDSetTimer:
				compiled.setTimerParams = append(compiled.setTimerParams, setTimerParams{Duration: action.DurationTicks})
				compiledAction.paramIndex = uint16(len(compiled.setTimerParams) - 1)
			case actionIDSetWaypoint:
				compiled.setWaypointParams = append(compiled.setWaypointParams, setWaypointParams{Index: action.Waypoint, Advance: action.Advance})
				compiledAction.paramIndex = uint16(len(compiled.setWaypointParams) - 1)
			case actionIDSetRandomDestination:
				compiled.randomDestinationParams = append(compiled.randomDestinationParams, randomDestinationParams{Radius: action.Radius, MinRadius: action.MinRadius})
				compiledAction.paramIndex = uint16(len(compiled.randomDestinationParams) - 1)
			case actionIDMoveAway:
				compiled.moveAwayParams = append(compiled.moveAwayParams, moveAwayParams{Distance: action.Distance, MinDistance: action.MinDistance})
				compiledAction.paramIndex = uint16(len(compiled.moveAwayParams) - 1)
			}
			compiledState.actions = append(compiledState.actions, compiledAction)
		}

		for _, transition := range state.Transitions {
			cond, err := parseConditionID(transition.Condition)
			if err != nil {
				return nil, fmt.Errorf("state %q transition: %w", state.ID, err)
			}
			compiledTransition := compiledTransition{conditionID: cond}
			switch cond {
			case conditionReachedWaypoint:
				compiled.reachedWaypointParams = append(compiled.reachedWaypointParams, reachedWaypointParams{ArriveRadius: transition.Radius})
				compiledTransition.paramIndex = uint16(len(compiled.reachedWaypointParams) - 1)
			case conditionPlayerWithin:
				compiled.playerWithinParams = append(compiled.playerWithinParams, playerWithinParams{Radius: transition.Radius})
				compiledTransition.paramIndex = uint16(len(compiled.playerWithinParams) - 1)
			case conditionLostSight:
				compiled.lostSightParams = append(compiled.lostSightParams, lostSightParams{Distance: transition.Distance})
				compiledTransition.paramIndex = uint16(len(compiled.lostSightParams) - 1)
			case conditionCooldownReady:
				ability, err := parseAbilityID(transition.Ability)
				if err != nil {
					return nil, fmt.Errorf("state %q transition ability: %w", state.ID, err)
				}
				compiled.cooldownReadyParams = append(compiled.cooldownReadyParams, cooldownReadyParams{Ability: ability})
				compiledTransition.paramIndex = uint16(len(compiled.cooldownReadyParams) - 1)
			case conditionStuck:
				compiled.stuckParams = append(compiled.stuckParams, stuckParams{Decisions: transition.Decisions, Epsilon: transition.Epsilon})
				compiledTransition.paramIndex = uint16(len(compiled.stuckParams) - 1)
			case conditionNonRatWithin:
				compiled.actorWithinParams = append(compiled.actorWithinParams, actorWithinParams{Radius: transition.Radius})
				compiledTransition.paramIndex = uint16(len(compiled.actorWithinParams) - 1)
			}

			target := strings.TrimSpace(transition.ToState)
			if target == "" {
				return nil, fmt.Errorf("state %q transition missing target", state.ID)
			}
			lower := strings.ToLower(target)
			idx, ok := stateIndex[lower]
			if !ok {
				return nil, fmt.Errorf("state %q transition references unknown state %q", state.ID, target)
			}
			compiledTransition.toState = uint8(idx)
			compiledState.transitions = append(compiledState.transitions, compiledTransition)
		}

		compiled.states = append(compiled.states, compiledState)
	}

	return compiled, nil
}

func parseActionID(name string) (actionID, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "movetoward":
		return actionIDMoveToward, nil
	case "stop":
		return actionIDStop, nil
	case "useability":
		return actionIDUseAbility, nil
	case "face":
		return actionIDFace, nil
	case "settimer":
		return actionIDSetTimer, nil
	case "setwaypoint":
		return actionIDSetWaypoint, nil
	case "setrandomdestination":
		return actionIDSetRandomDestination, nil
	case "moveaway":
		return actionIDMoveAway, nil
	default:
		return 0, fmt.Errorf("unknown action %q", name)
	}
}

func parseConditionID(name string) (conditionID, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "reachedwaypoint":
		return conditionReachedWaypoint, nil
	case "timerexpired":
		return conditionTimerExpired, nil
	case "playerwithin":
		return conditionPlayerWithin, nil
	case "lostsight":
		return conditionLostSight, nil
	case "cooldownready":
		return conditionCooldownReady, nil
	case "stuck":
		return conditionStuck, nil
	case "nonratwithin":
		return conditionNonRatWithin, nil
	default:
		return 0, fmt.Errorf("unknown condition %q", name)
	}
}

func parseMoveTarget(target string) moveTarget {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "player":
		return moveTargetPlayer
	case "vector":
		return moveTargetVector
	default:
		return moveTargetWaypoint
	}
}

func parseAbilityID(name string) (AbilityID, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "none":
		return AbilityNone, nil
	case strings.ToLower(combat.EffectTypeAttack):
		return AbilityAttack, nil
	case strings.ToLower(combat.EffectTypeFireball):
		return AbilityFireball, nil
	default:
		return 0, fmt.Errorf("unknown ability %q", name)
	}
}

type authoringConfig struct {
	NPCType            string            `json:"npc_type"`
	States             []authoringState  `json:"states"`
	BlackboardDefaults authoringDefaults `json:"blackboard_defaults"`
}

type authoringDefaults struct {
	WaypointIndex int     `json:"waypoint_index"`
	ArriveRadius  float64 `json:"arrive_radius"`
	PauseTicks    uint64  `json:"pause_ticks"`
	PatrolSpeed   float64 `json:"patrol_speed"`
	StuckEpsilon  float64 `json:"stuck_epsilon"`
}

type authoringState struct {
	ID            string                `json:"id"`
	TickEvery     uint16                `json:"tick_every"`
	DurationTicks uint16                `json:"duration_ticks"`
	Actions       []authoringAction     `json:"actions"`
	Transitions   []authoringTransition `json:"transitions"`
}

type authoringAction struct {
	Name          string           `json:"name"`
	Target        string           `json:"target,omitempty"`
	DurationTicks uint16           `json:"duration_ticks,omitempty"`
	Waypoint      int              `json:"waypoint,omitempty"`
	Advance       bool             `json:"advance,omitempty"`
	Ability       string           `json:"ability,omitempty"`
	Vector        *authoringVector `json:"vector,omitempty"`
	Radius        float64          `json:"radius,omitempty"`
	MinRadius     float64          `json:"min_radius,omitempty"`
	Distance      float64          `json:"distance,omitempty"`
	MinDistance   float64          `json:"min_distance,omitempty"`
}

type authoringTransition struct {
	Condition string  `json:"if"`
	ToState   string  `json:"to"`
	Radius    float64 `json:"radius,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
	Duration  uint16  `json:"duration_ticks,omitempty"`
	Ability   string  `json:"ability,omitempty"`
	Decisions uint8   `json:"decisions,omitempty"`
	Epsilon   float64 `json:"epsilon,omitempty"`
}

type authoringVector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
