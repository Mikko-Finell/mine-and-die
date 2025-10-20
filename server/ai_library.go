package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed ai_configs/*.json
var embeddedAIConfigs embed.FS

var globalAILibrary = mustLoadAILibrary()

type aiLibrary struct {
	configsByID   map[uint16]*aiCompiledConfig
	configsByType map[NPCType]*aiCompiledConfig
	nextID        uint16
}

type aiCompiledConfig struct {
	id           uint16
	initialState uint8
	states       []aiCompiledState
	stateNames   []string
	defaults     aiBlackboardDefaults

	moveTowardParams        []aiMoveTowardParams
	useAbilityParams        []aiUseAbilityParams
	faceParams              []aiFaceParams
	setTimerParams          []aiSetTimerParams
	setWaypointParams       []aiSetWaypointParams
	randomDestinationParams []aiRandomDestinationParams
	moveAwayParams          []aiMoveAwayParams
	reachedWaypointParams   []aiReachedWaypointParams
	playerWithinParams      []aiPlayerWithinParams
	lostSightParams         []aiLostSightParams
	cooldownReadyParams     []aiCooldownReadyParams
	stuckParams             []aiStuckParams
	nonRatWithinParams      []aiActorWithinParams
}

type aiCompiledState struct {
	cadence     uint16
	enterTimer  uint16
	actions     []aiCompiledAction
	transitions []aiCompiledTransition
}

type aiCompiledAction struct {
	id         aiActionID
	paramIndex uint16
}

type aiCompiledTransition struct {
	conditionID aiConditionID
	paramIndex  uint16
	toState     uint8
}

type aiBlackboardDefaults struct {
	WaypointIndex int
	ArriveRadius  float64
	PauseTicks    uint64
	PatrolSpeed   float64
	StuckEpsilon  float64
}

type aiMoveTowardParams struct {
	Target aiMoveTarget
	Vector vec2
}

type aiUseAbilityParams struct {
	Ability aiAbilityID
}

type aiFaceParams struct {
	Target aiMoveTarget
}

type aiSetTimerParams struct {
	Duration uint16
}

type aiSetWaypointParams struct {
	Index   int
	Advance bool
}

type aiReachedWaypointParams struct {
	ArriveRadius float64
}

type aiPlayerWithinParams struct {
	Radius float64
}

type aiActorWithinParams struct {
	Radius float64
}

type aiLostSightParams struct {
	Distance float64
}

type aiCooldownReadyParams struct {
	Ability aiAbilityID
}

type aiStuckParams struct {
	Decisions uint8
	Epsilon   float64
}

type aiRandomDestinationParams struct {
	Radius    float64
	MinRadius float64
}

type aiMoveAwayParams struct {
	Distance    float64
	MinDistance float64
}

type aiMoveTarget uint8

type aiActionID uint8

type aiConditionID uint8

type aiAbilityID uint8

const (
	aiMoveTargetWaypoint aiMoveTarget = iota
	aiMoveTargetPlayer
	aiMoveTargetVector
)

const (
	aiActionMoveToward aiActionID = iota
	aiActionStop
	aiActionUseAbility
	aiActionFace
	aiActionSetTimer
	aiActionSetWaypoint
	aiActionSetRandomDestination
	aiActionMoveAway
)

const (
	aiConditionReachedWaypoint aiConditionID = iota
	aiConditionTimerExpired
	aiConditionPlayerWithin
	aiConditionLostSight
	aiConditionCooldownReady
	aiConditionStuck
	aiConditionNonRatWithin
)

const (
	aiAbilityNone aiAbilityID = iota
	aiAbilityAttack
	aiAbilityFireball
)

type aiAuthoringConfig struct {
	NPCType            string              `json:"npc_type"`
	States             []aiAuthoringState  `json:"states"`
	BlackboardDefaults aiAuthoringDefaults `json:"blackboard_defaults"`
}

type aiAuthoringDefaults struct {
	WaypointIndex int     `json:"waypoint_index"`
	ArriveRadius  float64 `json:"arrive_radius"`
	PauseTicks    uint64  `json:"pause_ticks"`
	PatrolSpeed   float64 `json:"patrol_speed"`
	StuckEpsilon  float64 `json:"stuck_epsilon"`
}

type aiAuthoringState struct {
	ID            string                  `json:"id"`
	TickEvery     uint16                  `json:"tick_every"`
	DurationTicks uint16                  `json:"duration_ticks"`
	Actions       []aiAuthoringAction     `json:"actions"`
	Transitions   []aiAuthoringTransition `json:"transitions"`
}

type aiAuthoringAction struct {
	Name          string             `json:"name"`
	Target        string             `json:"target,omitempty"`
	DurationTicks uint16             `json:"duration_ticks,omitempty"`
	Waypoint      int                `json:"waypoint,omitempty"`
	Advance       bool               `json:"advance,omitempty"`
	Ability       string             `json:"ability,omitempty"`
	Vector        *aiAuthoringVector `json:"vector,omitempty"`
	Radius        float64            `json:"radius,omitempty"`
	MinRadius     float64            `json:"min_radius,omitempty"`
	Distance      float64            `json:"distance,omitempty"`
	MinDistance   float64            `json:"min_distance,omitempty"`
}

type aiAuthoringVector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type aiAuthoringTransition struct {
	Condition string  `json:"if"`
	To        string  `json:"to"`
	Radius    float64 `json:"radius,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
	Ability   string  `json:"ability,omitempty"`
	Decisions uint8   `json:"decisions,omitempty"`
	Epsilon   float64 `json:"epsilon,omitempty"`
}

func mustLoadAILibrary() *aiLibrary {
	lib, err := loadAILibrary()
	if err != nil {
		panic(err)
	}
	return lib
}

func loadAILibrary() (*aiLibrary, error) {
	entries, err := fs.ReadDir(embeddedAIConfigs, "ai_configs")
	if err != nil {
		return nil, fmt.Errorf("read ai configs: %w", err)
	}
	lib := &aiLibrary{
		configsByID:   make(map[uint16]*aiCompiledConfig),
		configsByType: make(map[NPCType]*aiCompiledConfig),
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := embeddedAIConfigs.ReadFile("ai_configs/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		var authoring aiAuthoringConfig
		if err := json.Unmarshal(data, &authoring); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		cfg, err := compileAIConfig(authoring)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", entry.Name(), err)
		}
		cfg.id = lib.allocateID()
		npcType := NPCType(authoring.NPCType)
		lib.configsByID[cfg.id] = cfg
		lib.configsByType[npcType] = cfg
	}
	return lib, nil
}

func (l *aiLibrary) allocateID() uint16 {
	l.nextID++
	return l.nextID
}

func (l *aiLibrary) ConfigForType(t NPCType) *aiCompiledConfig {
	if l == nil {
		return nil
	}
	return l.configsByType[t]
}

func (l *aiLibrary) ConfigByID(id uint16) *aiCompiledConfig {
	if l == nil {
		return nil
	}
	return l.configsByID[id]
}

func compileAIConfig(authoring aiAuthoringConfig) (*aiCompiledConfig, error) {
	if len(authoring.States) == 0 {
		return nil, fmt.Errorf("ai config must define at least one state")
	}
	cfg := &aiCompiledConfig{
		states:     make([]aiCompiledState, len(authoring.States)),
		stateNames: make([]string, len(authoring.States)),
	}
	cfg.defaults = aiBlackboardDefaults{
		WaypointIndex: authoring.BlackboardDefaults.WaypointIndex,
		ArriveRadius:  authoring.BlackboardDefaults.ArriveRadius,
		PauseTicks:    authoring.BlackboardDefaults.PauseTicks,
		PatrolSpeed:   authoring.BlackboardDefaults.PatrolSpeed,
		StuckEpsilon:  authoring.BlackboardDefaults.StuckEpsilon,
	}

	stateID := make(map[string]uint8, len(authoring.States))
	for idx, state := range authoring.States {
		if state.ID == "" {
			return nil, fmt.Errorf("state %d missing id", idx)
		}
		if len(authoring.States) > 255 {
			return nil, fmt.Errorf("ai config exceeds 255 states")
		}
		stateID[state.ID] = uint8(idx)
		cfg.stateNames[idx] = state.ID
		cfg.states[idx].cadence = state.TickEvery
		cfg.states[idx].enterTimer = state.DurationTicks
	}
	cfg.initialState = 0

	for idx, state := range authoring.States {
		compiled := &cfg.states[idx]
		compiled.actions = make([]aiCompiledAction, 0, len(state.Actions))
		compiled.transitions = make([]aiCompiledTransition, 0, len(state.Transitions))

		for _, action := range state.Actions {
			actionID, err := parseAIActionID(action.Name)
			if err != nil {
				return nil, fmt.Errorf("state %s: %w", state.ID, err)
			}
			compiledAction := aiCompiledAction{id: actionID}
			switch actionID {
			case aiActionMoveToward:
				params := aiMoveTowardParams{Target: parseMoveTarget(action.Target)}
				if params.Target == aiMoveTargetVector {
					if action.Vector == nil {
						return nil, fmt.Errorf("state %s action moveToward(vector) requires vector", state.ID)
					}
					params.Vector = vec2{X: action.Vector.X, Y: action.Vector.Y}
				}
				cfg.moveTowardParams = append(cfg.moveTowardParams, params)
				compiledAction.paramIndex = uint16(len(cfg.moveTowardParams) - 1)
			case aiActionSetRandomDestination:
				params := aiRandomDestinationParams{Radius: action.Radius, MinRadius: action.MinRadius}
				cfg.randomDestinationParams = append(cfg.randomDestinationParams, params)
				compiledAction.paramIndex = uint16(len(cfg.randomDestinationParams) - 1)
			case aiActionMoveAway:
				params := aiMoveAwayParams{Distance: action.Distance, MinDistance: action.MinDistance}
				cfg.moveAwayParams = append(cfg.moveAwayParams, params)
				compiledAction.paramIndex = uint16(len(cfg.moveAwayParams) - 1)
			case aiActionUseAbility:
				ability, err := parseAbilityID(action.Ability)
				if err != nil {
					return nil, fmt.Errorf("state %s action useAbility: %w", state.ID, err)
				}
				cfg.useAbilityParams = append(cfg.useAbilityParams, aiUseAbilityParams{Ability: ability})
				compiledAction.paramIndex = uint16(len(cfg.useAbilityParams) - 1)
			case aiActionFace:
				params := aiFaceParams{Target: parseMoveTarget(action.Target)}
				cfg.faceParams = append(cfg.faceParams, params)
				compiledAction.paramIndex = uint16(len(cfg.faceParams) - 1)
			case aiActionSetTimer:
				cfg.setTimerParams = append(cfg.setTimerParams, aiSetTimerParams{Duration: action.DurationTicks})
				compiledAction.paramIndex = uint16(len(cfg.setTimerParams) - 1)
			case aiActionSetWaypoint:
				params := aiSetWaypointParams{Advance: true}
				if action.Advance {
					params.Advance = true
				}
				if action.Waypoint > 0 || (action.Waypoint == 0 && !action.Advance) {
					params.Advance = false
					params.Index = action.Waypoint
				}
				cfg.setWaypointParams = append(cfg.setWaypointParams, params)
				compiledAction.paramIndex = uint16(len(cfg.setWaypointParams) - 1)
			}
			compiled.actions = append(compiled.actions, compiledAction)
		}

		for _, transition := range state.Transitions {
			condID, err := parseAIConditionID(transition.Condition)
			if err != nil {
				return nil, fmt.Errorf("state %s: %w", state.ID, err)
			}
			targetID, ok := stateID[transition.To]
			if !ok {
				return nil, fmt.Errorf("state %s transition references unknown state %q", state.ID, transition.To)
			}
			compiledTransition := aiCompiledTransition{conditionID: condID, toState: targetID}
			switch condID {
			case aiConditionReachedWaypoint:
				cfg.reachedWaypointParams = append(cfg.reachedWaypointParams, aiReachedWaypointParams{ArriveRadius: transition.Radius})
				compiledTransition.paramIndex = uint16(len(cfg.reachedWaypointParams) - 1)
			case aiConditionPlayerWithin:
				cfg.playerWithinParams = append(cfg.playerWithinParams, aiPlayerWithinParams{Radius: transition.Radius})
				compiledTransition.paramIndex = uint16(len(cfg.playerWithinParams) - 1)
			case aiConditionNonRatWithin:
				cfg.nonRatWithinParams = append(cfg.nonRatWithinParams, aiActorWithinParams{Radius: transition.Radius})
				compiledTransition.paramIndex = uint16(len(cfg.nonRatWithinParams) - 1)
			case aiConditionLostSight:
				cfg.lostSightParams = append(cfg.lostSightParams, aiLostSightParams{Distance: transition.Distance})
				compiledTransition.paramIndex = uint16(len(cfg.lostSightParams) - 1)
			case aiConditionCooldownReady:
				ability, err := parseAbilityID(transition.Ability)
				if err != nil {
					return nil, fmt.Errorf("state %s transition cooldownReady: %w", state.ID, err)
				}
				cfg.cooldownReadyParams = append(cfg.cooldownReadyParams, aiCooldownReadyParams{Ability: ability})
				compiledTransition.paramIndex = uint16(len(cfg.cooldownReadyParams) - 1)
			case aiConditionStuck:
				cfg.stuckParams = append(cfg.stuckParams, aiStuckParams{Decisions: transition.Decisions, Epsilon: transition.Epsilon})
				compiledTransition.paramIndex = uint16(len(cfg.stuckParams) - 1)
			}
			compiled.transitions = append(compiled.transitions, compiledTransition)
		}
	}

	return cfg, nil
}

func parseAIActionID(name string) (aiActionID, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "movetoward":
		return aiActionMoveToward, nil
	case "stop":
		return aiActionStop, nil
	case "useability":
		return aiActionUseAbility, nil
	case "face":
		return aiActionFace, nil
	case "settimer":
		return aiActionSetTimer, nil
	case "setwaypoint":
		return aiActionSetWaypoint, nil
	case "setrandomdestination":
		return aiActionSetRandomDestination, nil
	case "moveaway":
		return aiActionMoveAway, nil
	default:
		return 0, fmt.Errorf("unknown action %q", name)
	}
}

func parseAIConditionID(name string) (aiConditionID, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "reachedwaypoint":
		return aiConditionReachedWaypoint, nil
	case "timerexpired":
		return aiConditionTimerExpired, nil
	case "playerwithin":
		return aiConditionPlayerWithin, nil
	case "lostsight":
		return aiConditionLostSight, nil
	case "cooldownready":
		return aiConditionCooldownReady, nil
	case "stuck":
		return aiConditionStuck, nil
	case "nonratwithin":
		return aiConditionNonRatWithin, nil
	default:
		return 0, fmt.Errorf("unknown condition %q", name)
	}
}

func parseMoveTarget(target string) aiMoveTarget {
	key := strings.ToLower(strings.TrimSpace(target))
	switch key {
	case "player":
		return aiMoveTargetPlayer
	case "vector":
		return aiMoveTargetVector
	default:
		return aiMoveTargetWaypoint
	}
}

func parseAbilityID(name string) (aiAbilityID, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "", "none":
		return aiAbilityNone, nil
	case effectTypeAttack:
		return aiAbilityAttack, nil
	case effectTypeFireball:
		return aiAbilityFireball, nil
	default:
		return 0, fmt.Errorf("unknown ability %q", name)
	}
}

func (cfg *aiCompiledConfig) applyDefaults(bb *npcBlackboard) {
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

func (cfg *aiCompiledConfig) stateName(id uint8) string {
	if cfg == nil {
		return ""
	}
	if int(id) >= len(cfg.stateNames) {
		return ""
	}
	return cfg.stateNames[id]
}
