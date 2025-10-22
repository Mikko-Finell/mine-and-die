package world

import "time"

// MeleeActorVisitor iterates over actors participating in melee collision
// resolution. Callers must provide a function that invokes the visitor for each
// candidate target. The callback receives the actor identifier, world
// coordinates, and an opaque reference passed through to the hit callbacks.
type MeleeActorVisitor func(func(id string, x, y float64, reference any))

// ResolveMeleeImpactConfig bundles the inputs required to resolve a melee swing
// against world state without the helper depending on the legacy server types.
// Callers supply iterators for players and NPCs along with callback hooks for
// inventory updates, hit application, and telemetry so the helper can mirror the
// legacy behaviour while running inside the world package.
type ResolveMeleeImpactConfig struct {
	EffectType string
	Effect     any
	Owner      any
	ActorID    string
	Tick       uint64
	Now        time.Time
	Area       Obstacle
	Obstacles  []Obstacle

	ForEachPlayer MeleeActorVisitor
	ForEachNPC    MeleeActorVisitor

	GivePlayerGold func(actorID string) (bool, error)
	GiveNPCGold    func(actorID string) (bool, error)
	GiveOwnerGold  func(owner any) error

	ApplyPlayerHit func(effect any, target any, now time.Time)
	ApplyNPCHit    func(effect any, target any, now time.Time)

	RecordGoldGrantFailure func(actorID string, obstacleID string, err error)
	RecordAttackOverlap    func(actorID string, tick uint64, effectType string, playerHits []string, npcHits []string)
}

// ResolveMeleeImpact reproduces the legacy melee collision handling by
// inspecting the provided world state, awarding gold for ore deposits, applying
// hit callbacks for overlapping actors, and emitting telemetry through the
// supplied hooks. Behaviour matches the legacy implementation: the helper exits
// early when the effect reference is nil, scans for at most one ore deposit, and
// only records telemetry when at least one target is hit.
func ResolveMeleeImpact(cfg ResolveMeleeImpactConfig) {
	if cfg.Effect == nil {
		return
	}

	for _, obs := range cfg.Obstacles {
		if obs.Type != ObstacleTypeGoldOre {
			continue
		}
		if !ObstaclesOverlap(cfg.Area, obs, 0) {
			continue
		}

		handled := false
		var addErr error

		if cfg.GivePlayerGold != nil {
			if ok, err := cfg.GivePlayerGold(cfg.ActorID); ok {
				handled = true
				addErr = err
			}
		}

		if !handled && cfg.GiveNPCGold != nil {
			if ok, err := cfg.GiveNPCGold(cfg.ActorID); ok {
				handled = true
				addErr = err
			}
		}

		if !handled && cfg.Owner != nil && cfg.GiveOwnerGold != nil {
			handled = true
			addErr = cfg.GiveOwnerGold(cfg.Owner)
		}

		if addErr != nil && cfg.RecordGoldGrantFailure != nil {
			cfg.RecordGoldGrantFailure(cfg.ActorID, obs.ID, addErr)
		}

		break
	}

	var hitPlayers []string
	if cfg.ForEachPlayer != nil {
		cfg.ForEachPlayer(func(id string, x, y float64, reference any) {
			if id == cfg.ActorID {
				return
			}
			if !CircleRectOverlap(x, y, PlayerHalf, cfg.Area) {
				return
			}

			hitPlayers = append(hitPlayers, id)
			if cfg.ApplyPlayerHit != nil {
				cfg.ApplyPlayerHit(cfg.Effect, reference, cfg.Now)
			}
		})
	}

	var hitNPCs []string
	if cfg.ForEachNPC != nil {
		cfg.ForEachNPC(func(id string, x, y float64, reference any) {
			if id == cfg.ActorID {
				return
			}
			if !CircleRectOverlap(x, y, PlayerHalf, cfg.Area) {
				return
			}

			hitNPCs = append(hitNPCs, id)
			if cfg.ApplyNPCHit != nil {
				cfg.ApplyNPCHit(cfg.Effect, reference, cfg.Now)
			}
		})
	}

	if len(hitPlayers) == 0 && len(hitNPCs) == 0 {
		return
	}

	if cfg.RecordAttackOverlap != nil {
		cfg.RecordAttackOverlap(cfg.ActorID, cfg.Tick, cfg.EffectType, hitPlayers, hitNPCs)
	}
}
