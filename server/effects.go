package server

import (
	"fmt"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
)

// EffectTrigger represents a one-shot visual instruction that the client may
// execute without additional server updates.
type EffectTrigger struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Start    int64              `json:"start,omitempty"`
	Duration int64              `json:"duration,omitempty"`
	X        float64            `json:"x,omitempty"`
	Y        float64            `json:"y,omitempty"`
	Width    float64            `json:"width,omitempty"`
	Height   float64            `json:"height,omitempty"`
	Params   map[string]float64 `json:"params,omitempty"`
	Colors   []string           `json:"colors,omitempty"`
}

type (
	effectState          = internaleffects.State
	ProjectileTemplate   = internaleffects.ProjectileTemplate
	CollisionShapeConfig = internaleffects.CollisionShapeConfig
	TravelModeConfig     = internaleffects.TravelModeConfig
	ImpactRuleConfig     = internaleffects.ImpactRuleConfig
	ExplosionSpec        = internaleffects.ExplosionSpec
	ProjectileState      = internaleffects.ProjectileState
)

type projectileStopOptions struct {
	triggerImpact bool
	triggerExpiry bool
}

const (
	meleeAttackCooldown = combat.MeleeAttackCooldown
	meleeAttackDuration = combat.MeleeAttackDuration
	meleeAttackReach    = combat.MeleeAttackReach
	meleeAttackWidth    = combat.MeleeAttackWidth
	meleeAttackDamage   = combat.MeleeAttackDamage

	effectTypeAttack        = combat.EffectTypeAttack
	effectTypeFireball      = combat.EffectTypeFireball
	effectTypeBloodSplatter = combat.EffectTypeBloodSplatter
	effectTypeBurningTick   = combat.EffectTypeBurningTick
	effectTypeBurningVisual = combat.EffectTypeBurningVisual

	bloodSplatterDuration = 1200 * time.Millisecond

	fireballCooldown = 650 * time.Millisecond
	fireballSpeed    = 320.0
	fireballRange    = 5 * 40.0
	fireballSize     = 24.0
	fireballSpawnGap = 6.0
	fireballDamage   = 15.0
)

var fireballLifetime = time.Duration(fireballRange / fireballSpeed * float64(time.Second))

func newBloodSplatterParams() map[string]float64 {
	return internaleffects.NewBloodSplatterParams()
}

func bloodSplatterColors() []string {
	return internaleffects.BloodSplatterColors()
}

func newProjectileTemplates() map[string]*ProjectileTemplate {
	return map[string]*ProjectileTemplate{
		effectTypeFireball: {
			Type:        effectTypeFireball,
			Speed:       fireballSpeed,
			MaxDistance: fireballRange,
			Lifetime:    fireballLifetime,
			SpawnRadius: fireballSize / 2,
			SpawnOffset: playerHalf + fireballSpawnGap + fireballSize/2,
			CollisionShape: CollisionShapeConfig{
				UseRect: false,
			},
			TravelMode: TravelModeConfig{
				StraightLine: true,
			},
			ImpactRules: ImpactRuleConfig{
				StopOnHit:    true,
				MaxTargets:   1,
				AffectsOwner: false,
			},
			Params: map[string]float64{
				"radius":      fireballSize / 2,
				"speed":       fireballSpeed,
				"range":       fireballRange,
				"healthDelta": -fireballDamage,
			},
			Cooldown: fireballCooldown,
		},
	}
}

func (w *World) recordEffectSpawn(effectType, producer string) {
	if w == nil || w.telemetry == nil {
		return
	}
	w.telemetry.RecordEffectSpawned(effectType, producer)
}

func (w *World) recordEffectUpdate(eff *effectState, mutation string) {
	if w == nil || eff == nil || w.telemetry == nil {
		return
	}
	w.telemetry.RecordEffectUpdated(eff.Type, mutation)
}

func (w *World) recordEffectEnd(eff *effectState, reason string) {
	if w == nil || eff == nil {
		return
	}
	if !eff.TelemetryEnded {
		w.flushEffectTelemetry(eff)
		eff.TelemetryEnded = true
	}
	if w.telemetry != nil {
		w.telemetry.RecordEffectEnded(eff.Type, reason)
	}
}

func (w *World) recordEffectTrigger(triggerType string) {
	if w == nil || w.telemetry == nil {
		return
	}
	w.telemetry.RecordEffectTrigger(triggerType)
}

func (w *World) recordEffectHitTelemetry(eff *effectState, targetID string, delta float64) {
	if w == nil || eff == nil {
		return
	}
	if eff.TelemetrySpawnTick == 0 {
		eff.TelemetrySpawnTick = effectcontract.Tick(int64(w.currentTick))
	}
	if eff.TelemetryFirstHitTick == 0 {
		eff.TelemetryFirstHitTick = effectcontract.Tick(int64(w.currentTick))
	}
	eff.TelemetryHitCount++
	if eff.TelemetryVictims == nil {
		eff.TelemetryVictims = make(map[string]struct{})
	}
	if targetID != "" {
		eff.TelemetryVictims[targetID] = struct{}{}
	}
	if delta < 0 {
		eff.TelemetryDamage += -delta
	}
}

func (w *World) flushEffectTelemetry(eff *effectState) {
	if w == nil || eff == nil || w.telemetry == nil {
		return
	}
	victims := 0
	if len(eff.TelemetryVictims) > 0 {
		victims = len(eff.TelemetryVictims)
	}
	spawnTick := eff.TelemetrySpawnTick
	if spawnTick == 0 {
		spawnTick = effectcontract.Tick(int64(w.currentTick))
	}
	summary := effectParitySummary{
		EffectType:    eff.Type,
		Hits:          eff.TelemetryHitCount,
		UniqueVictims: victims,
		TotalDamage:   eff.TelemetryDamage,
		SpawnTick:     spawnTick,
		FirstHitTick:  eff.TelemetryFirstHitTick,
	}
	w.telemetry.RecordEffectParity(summary)
	eff.TelemetryHitCount = 0
	eff.TelemetryDamage = 0
	eff.TelemetryVictims = nil
	eff.TelemetryFirstHitTick = 0
}

// QueueEffectTrigger appends a fire-and-forget trigger for clients. The caller
// must hold the world mutex.
func (w *World) QueueEffectTrigger(trigger EffectTrigger, now time.Time) EffectTrigger {
	if w == nil {
		return EffectTrigger{}
	}
	if trigger.Type == "" {
		return EffectTrigger{}
	}
	if trigger.ID == "" {
		w.nextEffectID++
		trigger.ID = fmt.Sprintf("effect-%d", w.nextEffectID)
	}
	if trigger.Start == 0 {
		trigger.Start = now.UnixMilli()
	}
	w.effectTriggers = append(w.effectTriggers, trigger)
	w.recordEffectTrigger(trigger.Type)
	return trigger
}

func (w *World) abilityOwner(actorID string) (*combat.AbilityActor, *map[string]time.Time) {
	state, cooldowns := w.abilityOwnerState(actorID)
	if state == nil || cooldowns == nil {
		return nil, nil
	}
	return abilityActorSnapshot(state), cooldowns
}

func (w *World) abilityOwnerState(actorID string) (*actorState, *map[string]time.Time) {
	if player, ok := w.players[actorID]; ok {
		return &player.actorState, &player.cooldowns
	}
	if npc, ok := w.npcs[actorID]; ok {
		return &npc.actorState, &npc.cooldowns
	}
	return nil, nil
}

func (w *World) entityRef(actorID string) logging.EntityRef {
	if actorID == "" {
		return logging.EntityRef{}
	}
	if _, ok := w.players[actorID]; ok {
		return logging.EntityRef{ID: actorID, Kind: logging.EntityKind("player")}
	}
	if _, ok := w.npcs[actorID]; ok {
		return logging.EntityRef{ID: actorID, Kind: logging.EntityKind("npc")}
	}
	return logging.EntityRef{ID: actorID, Kind: logging.EntityKind("unknown")}
}

func contractSpawnProducer(definitionID string) string {
	switch definitionID {
	case effectTypeAttack:
		return "melee"
	default:
		return ""
	}
}

func (w *World) registerEffect(effect *effectState) bool {
	if w == nil {
		return false
	}
	return internaleffects.RegisterEffect(w.effectRegistry(), effect)
}

func (w *World) unregisterEffect(effect *effectState) {
	if w == nil {
		return
	}
	internaleffects.UnregisterEffect(w.effectRegistry(), effect)
}

// advanceEffects moves active projectiles and expires ones that collide or run out of range.
// LEGACY: advanceEffects updates the legacy projectile loop. When effectsgen
// definitions handle motion and expiry directly, this shim will be deleted.
func (w *World) advanceEffects(now time.Time, dt float64) {
	if len(w.effects) == 0 {
		return
	}
	w.advanceProjectiles(now, dt)
	w.advanceNonProjectiles(now, dt)
}

// LEGACY: advanceProjectiles exists solely for the legacy projectile runtime.
// Contract-managed instances bypass most of this code today and will replace it
// entirely after the effectsgen rollout.
func (w *World) advanceProjectiles(now time.Time, dt float64) {
	if len(w.effects) == 0 {
		return
	}
	for _, eff := range w.effects {
		if eff == nil {
			continue
		}
		if eff.ContractManaged {
			continue
		}
		p := eff.Projectile
		if p == nil {
			continue
		}
		if !now.Before(eff.ExpiresAt) {
			w.stopProjectile(eff, now, projectileStopOptions{triggerExpiry: true})
			continue
		}
		w.advanceProjectile(eff, now, dt)
	}
}

// LEGACY: advanceProjectile is the legacy physics step. Contract lifecycle
// hooks call it for parity, but it will be removed once the effectsgen engine
// owns projectile motion.
func (w *World) advanceProjectile(eff *effectState, now time.Time, dt float64) bool {
	if eff == nil || eff.Projectile == nil {
		return false
	}

	worldW, worldH := w.dimensions()
	tpl := eff.Projectile.Template

	var overlapMetadata map[string]any
	if w.recordAttackOverlap != nil {
		overlapMetadata = map[string]any{"projectile": eff.Type}
	}

	impactRules := combat.ProjectileImpactRules{}
	if tpl != nil {
		impactRules.StopOnHit = tpl.ImpactRules.StopOnHit
		impactRules.MaxTargets = tpl.ImpactRules.MaxTargets
		impactRules.AffectsOwner = tpl.ImpactRules.AffectsOwner
	}

	result := combat.AdvanceProjectile(combat.ProjectileAdvanceConfig{
		Effect:      eff,
		Delta:       dt,
		WorldWidth:  worldW,
		WorldHeight: worldH,
		ComputeArea: func() worldpkg.Obstacle {
			return effectAABB(eff)
		},
		AnyObstacleOverlap: func(area worldpkg.Obstacle) bool {
			return w.anyObstacleOverlap(area)
		},
		SetPosition: func(x, y float64) {
			w.SetEffectPosition(eff, x, y)
		},
		SetRemainingRange: func(remaining float64) {
			w.SetEffectParam(eff, "remainingRange", remaining)
		},
		Stop: func(triggerImpact, triggerExpiry bool) {
			w.stopProjectile(eff, now, projectileStopOptions{
				triggerImpact: triggerImpact,
				triggerExpiry: triggerExpiry,
			})
		},
		OverlapConfig: combat.ProjectileOverlapResolutionConfig{
			Impact:              impactRules,
			OwnerID:             eff.Owner,
			Ability:             eff.Type,
			Tick:                w.currentTick,
			Metadata:            overlapMetadata,
			RecordAttackOverlap: w.recordAttackOverlap,
			VisitPlayers: func(visitor combat.ProjectileOverlapVisitor) {
				for id, player := range w.players {
					if player == nil {
						continue
					}
					if !visitor(combat.ProjectileOverlapTarget{
						ID:     id,
						X:      player.X,
						Y:      player.Y,
						Radius: playerHalf,
						Raw:    player,
					}) {
						break
					}
				}
			},
			VisitNPCs: func(visitor combat.ProjectileOverlapVisitor) {
				for id, npc := range w.npcs {
					if npc == nil {
						continue
					}
					if !visitor(combat.ProjectileOverlapTarget{
						ID:     id,
						X:      npc.X,
						Y:      npc.Y,
						Radius: playerHalf,
						Raw:    npc,
					}) {
						break
					}
				}
			},
			OnPlayerHit: func(target combat.ProjectileOverlapTarget) {
				state, _ := target.Raw.(*playerState)
				if state == nil {
					return
				}
				w.invokePlayerHitCallback(eff, state, now)
			},
			OnNPCHit: func(target combat.ProjectileOverlapTarget) {
				state, _ := target.Raw.(*npcState)
				if state == nil {
					return
				}
				w.invokeNPCHitCallback(eff, state, now)
			},
		},
	})

	if tpl != nil && result.OverlapResult.HitsApplied > 0 {
		if tpl.ImpactRules.ExplodeOnImpact != nil {
			w.spawnAreaEffectAt(eff, now, tpl.ImpactRules.ExplodeOnImpact)
		}
	}

	return result.Stopped
}

func (w *World) advanceNonProjectiles(now time.Time, dt float64) {
	_ = dt
	if len(w.effects) == 0 {
		return
	}
	for _, eff := range w.effects {
		w.updateFollowEffect(eff, now)
	}
}

func (w *World) updateFollowEffect(eff *effectState, now time.Time) {
	if eff == nil {
		return
	}
	if eff.FollowActorID == "" {
		return
	}
	actor := w.actorByID(eff.FollowActorID)
	if actor == nil {
		w.expireAttachedEffect(eff, now)
		eff.FollowActorID = ""
		return
	}
	width := eff.Width
	height := eff.Height
	if width <= 0 {
		width = playerHalf * 2
	}
	if height <= 0 {
		height = playerHalf * 2
	}
	w.SetEffectPosition(eff, actor.X-width/2, actor.Y-height/2)
}

func (w *World) actorByID(id string) *actorState {
	if w == nil || id == "" {
		return nil
	}
	if player, ok := w.players[id]; ok && player != nil {
		return &player.actorState
	}
	if npc, ok := w.npcs[id]; ok && npc != nil {
		return &npc.actorState
	}
	return nil
}

func (w *World) stopProjectile(eff *effectState, now time.Time, opts projectileStopOptions) {
	if eff == nil || eff.Projectile == nil {
		return
	}

	p := eff.Projectile
	if p.RemainingRange != 0 {
		p.RemainingRange = 0
		w.SetEffectParam(eff, "remainingRange", 0)
	}

	if p.ExpiryResolved {
		if eff.ExpiresAt.After(now) {
			eff.ExpiresAt = now
		}
		return
	}

	tpl := p.Template
	if opts.triggerImpact && tpl != nil && tpl.ImpactRules.ExplodeOnImpact != nil {
		w.spawnAreaEffectAt(eff, now, tpl.ImpactRules.ExplodeOnImpact)
	}
	if opts.triggerExpiry {
		w.triggerExpiryExplosion(eff, now)
	}

	reason := "stopped"
	if opts.triggerImpact {
		reason = "impact"
	} else if opts.triggerExpiry {
		reason = "expiry"
	}

	p.ExpiryResolved = true
	eff.ExpiresAt = now
	w.recordEffectEnd(eff, reason)
}

func (w *World) triggerExpiryExplosion(eff *effectState, now time.Time) {
	if eff == nil || eff.Projectile == nil {
		return
	}
	tpl := eff.Projectile.Template
	if tpl == nil {
		return
	}
	spec := tpl.ImpactRules.ExplodeOnExpiry
	if spec == nil {
		return
	}
	if tpl.ImpactRules.ExpiryOnlyIfNoHits && eff.Projectile.HitCount > 0 {
		return
	}
	w.spawnAreaEffectAt(eff, now, spec)
}

func (w *World) spawnAreaEffectAt(eff *effectState, now time.Time, spec *ExplosionSpec) {
	if eff == nil || spec == nil {
		return
	}
	radius := spec.Radius
	size := radius * 2
	if size <= 0 {
		size = eff.Width
		if size <= 0 {
			size = 1
		}
	}
	params := internaleffects.MergeParams(spec.Params, map[string]float64{
		"radius": radius,
	})
	if spec.Duration > 0 {
		if params == nil {
			params = make(map[string]float64)
		}
		params["duration_ms"] = float64(spec.Duration.Milliseconds())
	}

	w.nextEffectID++
	instanceID := fmt.Sprintf("effect-%d", w.nextEffectID)
	area := &effectState{
		ID:       instanceID,
		Type:     spec.EffectType,
		Owner:    eff.Owner,
		Start:    now.UnixMilli(),
		Duration: spec.Duration.Milliseconds(),
		X:        centerX(eff) - size/2,
		Y:        centerY(eff) - size/2,
		Width:    size,
		Height:   size,
		Params:   params,
		Instance: effectcontract.EffectInstance{
			ID:           instanceID,
			DefinitionID: spec.EffectType,
			OwnerActorID: eff.Owner,
			StartTick:    effectcontract.Tick(int64(w.currentTick)),
		},
		ExpiresAt:          now.Add(spec.Duration),
		TelemetrySpawnTick: effectcontract.Tick(int64(w.currentTick)),
	}
	if !w.registerEffect(area) {
		return
	}
	w.recordEffectSpawn(spec.EffectType, "explosion")
}

func (w *World) maybeExplodeOnExpiry(eff *effectState, now time.Time) {
	w.stopProjectile(eff, now, projectileStopOptions{triggerExpiry: true})
}

func effectAABB(eff *effectState) Obstacle {
	if eff == nil {
		return Obstacle{}
	}
	return Obstacle{X: eff.X, Y: eff.Y, Width: eff.Width, Height: eff.Height}
}

func (w *World) findEffectByID(id string) *effectState {
	if w == nil {
		return nil
	}
	return internaleffects.FindByID(w.effectRegistry(), id)
}

func (w *World) anyObstacleOverlap(area Obstacle) bool {
	for _, obs := range w.obstacles {
		if obs.Type == obstacleTypeLava {
			continue
		}
		if obstaclesOverlap(area, obs, 0) {
			return true
		}
	}
	return false
}

func centerX(eff *effectState) float64 {
	return eff.X + eff.Width/2
}

func centerY(eff *effectState) float64 {
	return eff.Y + eff.Height/2
}

// pruneEffects drops expired effects from the in-memory list.
func (w *World) pruneEffects(now time.Time) {
	if w == nil {
		return
	}
	expired := internaleffects.PruneExpired(w.effectRegistry(), now)
	for _, eff := range expired {
		if eff == nil {
			continue
		}
		if eff.Projectile != nil && !eff.Projectile.ExpiryResolved {
			w.maybeExplodeOnExpiry(eff, now)
		}
		w.recordEffectEnd(eff, "expired")
		w.purgeEntityPatches(eff.ID)
	}
}

func (w *World) invokePlayerHitCallback(eff *effectState, target *playerState, now time.Time) {
	if w == nil || w.playerHitCallback == nil {
		return
	}
	w.playerHitCallback(eff, target, now)
}

func (w *World) invokeNPCHitCallback(eff *effectState, target *npcState, now time.Time) {
	if w == nil || w.npcHitCallback == nil {
		return
	}
	w.npcHitCallback(eff, target, now)
}

func (w *World) maybeSpawnBloodSplatter(eff *effectState, target *npcState, now time.Time) {
	if eff == nil || target == nil {
		return
	}
	if eff.Type != effectTypeAttack {
		return
	}
	if target.Type != NPCTypeGoblin && target.Type != NPCTypeRat {
		return
	}

	w.pruneEffects(now)

	if w.effectManager != nil {
		intent, ok := internaleffects.NewBloodSplatterIntent(internaleffects.BloodSplatterIntentConfig{
			SourceActorID: eff.Owner,
			TargetActorID: target.ID,
			Target: &internaleffects.ActorPosition{
				X: target.X,
				Y: target.Y,
			},
			TileSize:  tileSize,
			Footprint: playerHalf * 2,
			Duration:  bloodSplatterDuration,
			TickRate:  tickRate,
		})
		if ok {
			w.effectManager.EnqueueIntent(intent)
		}
	}
}

func (w *World) configureMeleeAbilityGate() {
	if w == nil {
		return
	}

	w.meleeAbilityGate = combat.NewMeleeAbilityGate(combat.MeleeAbilityGateConfig{
		AbilityID: effectTypeAttack,
		Cooldown:  meleeAttackCooldown,
		LookupOwner: func(actorID string) (*combat.AbilityActor, *map[string]time.Time, bool) {
			owner, cooldowns := w.abilityOwner(actorID)
			if owner == nil || cooldowns == nil {
				return nil, nil, false
			}

			return owner, cooldowns, true
		},
	})
}

func (w *World) configureProjectileAbilityGate() {
	if w == nil {
		return
	}

	tpl := w.projectileTemplates[effectTypeFireball]
	if tpl == nil {
		w.projectileAbilityGate = nil
		return
	}

	w.projectileAbilityGate = combat.NewProjectileAbilityGate(combat.ProjectileAbilityGateConfig{
		AbilityID: tpl.Type,
		Cooldown:  tpl.Cooldown,
		LookupOwner: func(actorID string) (*combat.AbilityActor, *map[string]time.Time, bool) {
			owner, cooldowns := w.abilityOwner(actorID)
			if owner == nil || cooldowns == nil {
				return nil, nil, false
			}

			return owner, cooldowns, true
		},
	})
}

func (w *World) configureEffectHitAdapter() {
	if w == nil {
		return
	}

	w.recordAttackOverlap = combat.NewAttackOverlapTelemetryRecorder(combat.AttackOverlapTelemetryRecorderConfig{
		Publisher: w.publisher,
		LookupEntity: func(id string) logging.EntityRef {
			return w.entityRef(id)
		},
	})

	w.effectHitAdapter = combat.NewLegacyWorldEffectHitAdapter(combat.LegacyWorldEffectHitAdapterConfig{
		HealthEpsilon:           worldpkg.HealthEpsilon,
		BaselinePlayerMaxHealth: baselinePlayerMaxHealth,
		ExtractEffect: func(effect any) (*internaleffects.State, bool) {
			eff, _ := effect.(*effectState)
			if eff == nil {
				return nil, false
			}
			return eff, true
		},
		ExtractActor: func(target any) (combat.WorldActorAdapter, bool) {
			if target == nil {
				return combat.WorldActorAdapter{}, false
			}

			var actor *actorState
			kind := combat.ActorKindGeneric

			switch typed := target.(type) {
			case *actorState:
				actor = typed
			case *playerState:
				if typed == nil {
					return combat.WorldActorAdapter{}, false
				}
				actor = &typed.actorState
				kind = combat.ActorKindPlayer
			case *npcState:
				if typed == nil {
					return combat.WorldActorAdapter{}, false
				}
				actor = &typed.actorState
				kind = combat.ActorKindNPC
			default:
				return combat.WorldActorAdapter{}, false
			}

			if actor == nil {
				return combat.WorldActorAdapter{}, false
			}

			return combat.WorldActorAdapter{
				ID:        actor.ID,
				Health:    actor.Health,
				MaxHealth: actor.MaxHealth,
				KindHint:  kind,
				Raw:       actor,
			}, true
		},
		IsPlayer: func(id string) bool {
			_, ok := w.players[id]
			return ok
		},
		IsNPC: func(id string) bool {
			_, ok := w.npcs[id]
			return ok
		},
		SetPlayerHealth: func(id string, next float64) {
			if id == "" {
				return
			}
			w.SetHealth(id, next)
		},
		SetNPCHealth: func(id string, next float64) {
			if id == "" {
				return
			}
			w.SetNPCHealth(id, next)
		},
		ApplyGenericHealthDelta: func(actor combat.WorldActorAdapter, delta float64) (bool, float64, float64) {
			state, _ := actor.Raw.(*actorState)
			if state == nil {
				return false, 0, actor.Health
			}
			before := state.Health
			if !state.applyHealthDelta(delta) {
				return false, 0, before
			}
			return true, state.Health - before, state.Health
		},
		RecordEffectHitTelemetry: func(effect *internaleffects.State, targetID string, actualDelta float64) {
			if effect == nil || targetID == "" {
				return
			}
			w.recordEffectHitTelemetry(effect, targetID, actualDelta)
		},
		RecordDamageTelemetry: combat.NewDamageTelemetryRecorder(combat.DamageTelemetryRecorderConfig{
			Publisher: w.publisher,
			LookupEntity: func(id string) logging.EntityRef {
				return w.entityRef(id)
			},
			CurrentTick: func() uint64 {
				return w.currentTick
			},
		}),
		RecordDefeatTelemetry: combat.NewDefeatTelemetryRecorder(combat.DefeatTelemetryRecorderConfig{
			Publisher: w.publisher,
			LookupEntity: func(id string) logging.EntityRef {
				return w.entityRef(id)
			},
			CurrentTick: func() uint64 {
				return w.currentTick
			},
		}),
		DropAllInventory: func(actor combat.WorldActorAdapter, reason string) {
			state, _ := actor.Raw.(*actorState)
			if state == nil {
				return
			}
			w.dropAllInventory(state, reason)
		},
		ApplyStatusEffect: func(effect *internaleffects.State, actor combat.WorldActorAdapter, statusEffect string, now time.Time) {
			if statusEffect == "" {
				return
			}
			state, _ := actor.Raw.(*actorState)
			if state == nil {
				return
			}
			ownerID := ""
			if effect != nil {
				ownerID = effect.Owner
			}
			w.applyStatusEffect(state, StatusEffectType(statusEffect), ownerID, now)
		},
	})
}

// applyEnvironmentalStatusEffects applies persistent effects triggered by hazards.
func (w *World) applyEnvironmentalStatusEffects(states []*actorState, now time.Time) {
	if len(states) == 0 {
		return
	}
	for _, state := range states {
		if state == nil {
			continue
		}
		for _, obs := range w.obstacles {
			if obs.Type != obstacleTypeLava {
				continue
			}
			if circleRectOverlap(state.X, state.Y, playerHalf, obs) {
				w.applyStatusEffect(state, StatusEffectBurning, obs.ID, now)
				break
			}
		}
	}
}
