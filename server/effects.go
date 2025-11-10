package server

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	worldpkg "mine-and-die/server/internal/world"
	abilitiespkg "mine-and-die/server/internal/world/abilities"
	worldeffects "mine-and-die/server/internal/world/effects"
	worldstate "mine-and-die/server/internal/world/state"
	statuspkg "mine-and-die/server/internal/world/status"
	"mine-and-die/server/logging"
)

// EffectTrigger represents a one-shot visual instruction that the client may
// execute without additional server updates.
type EffectTrigger = internaleffects.Trigger

type (
	effectState          = internaleffects.State
	ProjectileTemplate   = internaleffects.ProjectileTemplate
	CollisionShapeConfig = internaleffects.CollisionShapeConfig
	TravelModeConfig     = internaleffects.TravelModeConfig
	ImpactRuleConfig     = internaleffects.ImpactRuleConfig
	ExplosionSpec        = internaleffects.ExplosionSpec
	ProjectileState      = internaleffects.ProjectileState
)

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

	fireballCooldown = abilitiespkg.FireballCooldown
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
	if w == nil {
		return
	}
	worldpkg.RecordEffectSpawnTelemetry(w.telemetry, effectType, producer)
}

func (w *World) recordEffectUpdate(eff *effectState, mutation string) {
	if w == nil || eff == nil {
		return
	}
	worldpkg.RecordEffectUpdateTelemetry(w.telemetry, eff.Type, mutation)
}

func (w *World) recordEffectEnd(eff *effectState, reason string) {
	if w == nil || eff == nil {
		return
	}
	worldpkg.RecordEffectEndTelemetry(
		w.telemetry,
		(*worldeffects.State)(eff),
		reason,
		effectcontract.Tick(int64(w.currentTick)),
	)
}

func (w *World) recordEffectTrigger(triggerType string) {
	if w == nil {
		return
	}
	worldpkg.RecordEffectTriggerTelemetry(w.telemetry, triggerType)
}

func (w *World) recordEffectHitTelemetry(eff *effectState, targetID string, delta float64) {
	if w == nil || eff == nil {
		return
	}
	worldpkg.RecordEffectHitTelemetry(
		(*worldeffects.State)(eff),
		targetID,
		delta,
		effectcontract.Tick(int64(w.currentTick)),
	)
}

func (w *World) flushEffectTelemetry(eff *effectState) {
	if w == nil || eff == nil {
		return
	}
	worldpkg.FlushEffectTelemetry(
		w.telemetry,
		(*worldeffects.State)(eff),
		effectcontract.Tick(int64(w.currentTick)),
	)
}

func (w *World) configureAbilityOwnerAdapters() {
	if w == nil {
		return
	}

	stateLookup := worldpkg.NewAbilityOwnerStateLookup(worldpkg.AbilityOwnerStateLookupConfig[*actorState]{
		FindPlayer: func(actorID string) (*actorState, *map[string]time.Time, bool) {
			if w == nil || actorID == "" {
				return nil, nil, false
			}
			player, ok := w.players[actorID]
			if !ok || player == nil {
				return nil, nil, false
			}
			return &player.ActorState, &player.Cooldowns, true
		},
		FindNPC: func(actorID string) (*actorState, *map[string]time.Time, bool) {
			if w == nil || actorID == "" {
				return nil, nil, false
			}
			npc, ok := w.npcs[actorID]
			if !ok || npc == nil {
				return nil, nil, false
			}
			return &npc.ActorState, &npc.Cooldowns, true
		},
	})

	w.abilityOwnerStateLookup = stateLookup
	w.abilityOwnerLookup = worldpkg.NewAbilityOwnerLookup(worldpkg.AbilityOwnerLookupConfig[*actorState, worldpkg.AbilityActorSnapshot]{
		LookupState: stateLookup,
		Snapshot:    worldAbilityActorSnapshot,
	})
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
	if w.internalWorld != nil {
		queued := w.internalWorld.QueueEffectTrigger(trigger, now)
		w.nextEffectID = w.internalWorld.NextEffectID()
		return queued
	}
	if trigger.ID == "" {
		trigger.ID = w.allocateEffectID()
	}
	if trigger.Start == 0 {
		trigger.Start = now.UnixMilli()
	}
	w.recordEffectTrigger(trigger.Type)
	return trigger
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
	worldpkg.AdvanceLegacyProjectiles(worldpkg.LegacyProjectileAdvanceConfig{
		Now:   now,
		Delta: dt,
		ForEachEffect: func(visitor func(effect any)) {
			for _, eff := range w.effects {
				visitor(eff)
			}
		},
		Inspect: func(effect any) worldpkg.LegacyProjectileState {
			eff, _ := effect.(*effectState)
			if eff == nil {
				return worldpkg.LegacyProjectileState{}
			}
			return worldpkg.LegacyProjectileState{
				ContractManaged: eff.ContractManaged,
				HasProjectile:   eff.Projectile != nil,
				ExpiresAt:       eff.ExpiresAt,
			}
		},
		Advance: func(effect any, now time.Time, delta float64) {
			eff, _ := effect.(*effectState)
			if eff == nil {
				return
			}
			w.advanceProjectile(eff, now, delta)
		},
		StopAdapter:    w.projectileStopAdapter,
		StopProjectile: w.stopLegacyProjectile,
	})
	worldpkg.AdvanceLegacyFollowEffects(worldpkg.LegacyFollowEffectAdvanceConfig{
		Now: now,
		ForEachEffect: func(visitor func(effect any)) {
			for _, eff := range w.effects {
				visitor(eff)
			}
		},
		Inspect: func(effect any) worldpkg.LegacyFollowEffect {
			state, _ := effect.(*effectState)
			if state == nil {
				return worldpkg.LegacyFollowEffect{}
			}
			return worldpkg.LegacyFollowEffect{
				FollowActorID: state.FollowActorID,
				Width:         state.Width,
				Height:        state.Height,
			}
		},
		ActorByID: func(id string) *itemspkg.Actor {
			actor := w.actorByID(id)
			if actor == nil {
				return nil
			}
			return &itemspkg.Actor{ID: id, X: actor.X, Y: actor.Y}
		},
		Expire: func(effect any, at time.Time) {
			state, _ := effect.(*effectState)
			if state == nil {
				return
			}
			if statuspkg.ExpireStatusEffectAttachment(statusEffectAttachmentFields(state), at) {
				w.recordEffectEnd(state, "status-effect-expire")
			}
		},
		ClearFollow: func(effect any) {
			state, _ := effect.(*effectState)
			if state == nil {
				return
			}
			state.FollowActorID = ""
		},
		SetPosition: func(effect any, x, y float64) {
			state, _ := effect.(*effectState)
			if state == nil {
				return
			}
			w.SetEffectPosition(state, x, y)
		},
	})
}

func (w *World) projectileStopConfig(eff *effectState, now time.Time) combat.ProjectileStopConfig {
	if w == nil {
		return combat.ProjectileStopConfig{}
	}

	bindings := w.projectileStopAdapter.StopConfig(eff, now)
	if bindings.Effect == nil {
		bindings.Effect = eff
	}
	if bindings.Now.IsZero() {
		bindings.Now = now
	}
	return w.bindProjectileStopConfig(bindings, eff, now)
}

func (w *World) stopLegacyProjectile(bindings worldpkg.ProjectileStopConfig, opts worldpkg.ProjectileStopOptions) {
	if w == nil {
		return
	}

	effect, _ := bindings.Effect.(*effectState)
	combatCfg := w.bindProjectileStopConfig(bindings, effect, bindings.Now)
	combatCfg.Options = combat.ProjectileStopOptions{
		TriggerImpact: opts.TriggerImpact,
		TriggerExpiry: opts.TriggerExpiry,
	}
	combat.StopProjectile(combatCfg)
}

func (w *World) bindProjectileStopConfig(bindings worldpkg.ProjectileStopConfig, fallbackEffect *effectState, fallbackNow time.Time) combat.ProjectileStopConfig {
	stopCfg := combat.ProjectileStopConfig{
		Effect: fallbackEffect,
		Now:    fallbackNow,
	}

	if effectValue := bindings.Effect; effectValue != nil {
		if bound, ok := effectValue.(*effectState); ok && bound != nil {
			stopCfg.Effect = bound
		}
	}
	if stopCfg.Effect == nil {
		stopCfg.Effect = fallbackEffect
	}

	if !bindings.Now.IsZero() {
		stopCfg.Now = bindings.Now
	} else if stopCfg.Now.IsZero() {
		stopCfg.Now = fallbackNow
	}

	effectRef := stopCfg.Effect

	if bindings.SetRemainingRange != nil {
		stopCfg.SetRemainingRange = bindings.SetRemainingRange
	} else if effectRef != nil {
		stopCfg.SetRemainingRange = func(remaining float64) {
			w.SetEffectParam(effectRef, "remainingRange", remaining)
		}
	}

	if bindings.RecordEffectEnd != nil {
		stopCfg.RecordEffectEnd = bindings.RecordEffectEnd
	} else {
		stopCfg.RecordEffectEnd = func(reason string) {
			w.recordEffectEnd(effectRef, reason)
		}
	}

	areaBindings := bindings.AreaEffectSpawn
	spawnCfg := internaleffects.AreaEffectSpawnConfig{
		Now:         areaBindings.Now,
		CurrentTick: areaBindings.CurrentTick,
		AllocateID:  areaBindings.AllocateID,
		RecordSpawn: areaBindings.RecordSpawn,
	}
	if sourceValue := areaBindings.Source; sourceValue != nil {
		if source, ok := sourceValue.(*effectState); ok && source != nil {
			spawnCfg.Source = source
		}
	}
	if spawnCfg.Source == nil {
		spawnCfg.Source = effectRef
	}
	if spawnCfg.Now.IsZero() {
		if !stopCfg.Now.IsZero() {
			spawnCfg.Now = stopCfg.Now
		} else {
			spawnCfg.Now = fallbackNow
		}
	}
	if spawnCfg.CurrentTick == 0 {
		if areaBindings.CurrentTick != 0 {
			spawnCfg.CurrentTick = areaBindings.CurrentTick
		} else {
			spawnCfg.CurrentTick = effectcontract.Tick(int64(w.currentTick))
		}
	}
	if spawnCfg.AllocateID == nil {
		spawnCfg.AllocateID = w.allocateEffectID
	}
	if areaBindings.Register != nil {
		spawnCfg.Register = func(effect *internaleffects.State) bool {
			return areaBindings.Register(effect)
		}
	}
	if spawnCfg.Register == nil {
		spawnCfg.Register = func(effect *internaleffects.State) bool {
			return w.registerEffect(effect)
		}
	}
	if spawnCfg.RecordSpawn == nil {
		spawnCfg.RecordSpawn = w.recordEffectSpawn
	}

	stopCfg.AreaEffectSpawn = &spawnCfg
	return stopCfg
}

// LEGACY: advanceProjectile is the legacy physics step. Contract lifecycle
// hooks call it for parity, but it will be removed once the effectsgen engine
// owns projectile motion.
func (w *World) advanceProjectile(eff *effectState, now time.Time, dt float64) bool {
	if eff == nil {
		return false
	}

	result := worldpkg.AdvanceLegacyProjectile(worldpkg.LegacyProjectileStepConfig{
		Effect: eff,
		Now:    now,
		Delta:  dt,
		HasProjectile: func(effect any) bool {
			state, _ := effect.(*effectState)
			return state != nil && state.Projectile != nil
		},
		Dimensions: w.dimensions,
		ComputeArea: func(effect any) worldpkg.Obstacle {
			state, _ := effect.(*effectState)
			if state == nil {
				return worldpkg.Obstacle{}
			}
			return effectAABB(state)
		},
		AnyObstacleOverlap: func(obstacle worldpkg.Obstacle) bool {
			return w.anyObstacleOverlap(obstacle)
		},
		SetPosition: func(effect any, x, y float64) {
			state, _ := effect.(*effectState)
			if state == nil {
				return
			}
			w.SetEffectPosition(state, x, y)
		},
		StopAdapter: w.projectileStopAdapter,
		BindStopConfig: func(bindings worldpkg.ProjectileStopConfig, effect any, at time.Time) any {
			state, _ := effect.(*effectState)
			return w.bindProjectileStopConfig(bindings, state, at)
		},
		RecordAttackOverlap: w.recordAttackOverlap,
		CurrentTick: func() uint64 {
			return w.currentTick
		},
		VisitPlayers: func(visitor worldpkg.LegacyProjectileOverlapVisitor) {
			for id, player := range w.players {
				if player == nil {
					continue
				}
				if !visitor(worldpkg.LegacyProjectileOverlapTarget{
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
		VisitNPCs: func(visitor worldpkg.LegacyProjectileOverlapVisitor) {
			for id, npc := range w.npcs {
				if npc == nil {
					continue
				}
				if !visitor(worldpkg.LegacyProjectileOverlapTarget{
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
		OnPlayerHit: func(target worldpkg.LegacyProjectileOverlapTarget) {
			state, _ := target.Raw.(*playerState)
			if state == nil {
				return
			}
			w.invokePlayerHitCallback(eff, state, now)
		},
		OnNPCHit: func(target worldpkg.LegacyProjectileOverlapTarget) {
			state, _ := target.Raw.(*npcState)
			if state == nil {
				return
			}
			w.invokeNPCHitCallback(eff, state, now)
		},
		Advance: func(stepCfg worldpkg.LegacyProjectileStepAdvanceConfig) worldpkg.LegacyProjectileStepAdvanceResult {
			state, _ := stepCfg.Effect.(*effectState)
			if state == nil || state.Projectile == nil {
				return worldpkg.LegacyProjectileStepAdvanceResult{}
			}

			combatStop := combat.ProjectileStopConfig{}
			if stepCfg.BindStopConfig != nil {
				if bound, ok := stepCfg.BindStopConfig(stepCfg.StopBindings, state, stepCfg.Now).(combat.ProjectileStopConfig); ok {
					combatStop = bound
				}
			}

			setRemainingRange := combatStop.SetRemainingRange
			if setRemainingRange == nil {
				setRemainingRange = func(remaining float64) {
					w.SetEffectParam(state, "remainingRange", remaining)
				}
			}

			tpl := state.Projectile.Template
			impactRules := combat.ProjectileImpactRules{}
			if tpl != nil {
				impactRules.StopOnHit = tpl.ImpactRules.StopOnHit
				impactRules.MaxTargets = tpl.ImpactRules.MaxTargets
				impactRules.AffectsOwner = tpl.ImpactRules.AffectsOwner
			}

			var overlapMetadata map[string]any
			if stepCfg.RecordAttackOverlap != nil {
				overlapMetadata = map[string]any{"projectile": state.Type}
			}

			combatCfg := combat.ProjectileAdvanceConfig{
				Effect:      state,
				Delta:       stepCfg.Delta,
				WorldWidth:  stepCfg.WorldWidth,
				WorldHeight: stepCfg.WorldHeight,
				ComputeArea: func() combat.Rectangle {
					if stepCfg.ComputeArea == nil {
						return combat.Rectangle{}
					}
					area := stepCfg.ComputeArea()
					return combat.Rectangle{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height}
				},
				AnyObstacleOverlap: func(rect combat.Rectangle) bool {
					if stepCfg.AnyObstacleOverlap == nil {
						return false
					}
					return stepCfg.AnyObstacleOverlap(worldpkg.Obstacle{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height})
				},
				SetPosition:       stepCfg.SetPosition,
				SetRemainingRange: setRemainingRange,
				Stop:              combatStop,
				AreaEffectSpawn:   combatStop.AreaEffectSpawn,
				OverlapConfig: combat.ProjectileOverlapResolutionConfig{
					Impact:              impactRules,
					OwnerID:             state.Owner,
					Ability:             state.Type,
					Tick:                stepCfg.CurrentTick,
					Metadata:            overlapMetadata,
					RecordAttackOverlap: stepCfg.RecordAttackOverlap,
					VisitPlayers: func(visitor combat.ProjectileOverlapVisitor) {
						if stepCfg.VisitPlayers == nil {
							return
						}
						stepCfg.VisitPlayers(func(target worldpkg.LegacyProjectileOverlapTarget) bool {
							return visitor(combat.ProjectileOverlapTarget{
								ID:     target.ID,
								X:      target.X,
								Y:      target.Y,
								Radius: target.Radius,
								Raw:    target.Raw,
							})
						})
					},
					VisitNPCs: func(visitor combat.ProjectileOverlapVisitor) {
						if stepCfg.VisitNPCs == nil {
							return
						}
						stepCfg.VisitNPCs(func(target worldpkg.LegacyProjectileOverlapTarget) bool {
							return visitor(combat.ProjectileOverlapTarget{
								ID:     target.ID,
								X:      target.X,
								Y:      target.Y,
								Radius: target.Radius,
								Raw:    target.Raw,
							})
						})
					},
					OnPlayerHit: func(target combat.ProjectileOverlapTarget) {
						if stepCfg.OnPlayerHit == nil {
							return
						}
						stepCfg.OnPlayerHit(worldpkg.LegacyProjectileOverlapTarget{
							ID:     target.ID,
							X:      target.X,
							Y:      target.Y,
							Radius: target.Radius,
							Raw:    target.Raw,
						})
					},
					OnNPCHit: func(target combat.ProjectileOverlapTarget) {
						if stepCfg.OnNPCHit == nil {
							return
						}
						stepCfg.OnNPCHit(worldpkg.LegacyProjectileOverlapTarget{
							ID:     target.ID,
							X:      target.X,
							Y:      target.Y,
							Radius: target.Radius,
							Raw:    target.Raw,
						})
					},
				},
			}

			advanceResult := combat.AdvanceProjectile(combatCfg)
			return worldpkg.LegacyProjectileStepAdvanceResult{Stopped: advanceResult.Stopped, Raw: advanceResult}
		},
	})

	return result.Stopped
}

func (w *World) actorByID(id string) *actorState {
	if w == nil || id == "" {
		return nil
	}
	if player, ok := w.players[id]; ok && player != nil {
		return &player.ActorState
	}
	if npc, ok := w.npcs[id]; ok && npc != nil {
		return &npc.ActorState
	}
	return nil
}

func (w *World) maybeExplodeOnExpiry(eff *effectState, now time.Time) {
	stopCfg := w.projectileStopConfig(eff, now)
	stopCfg.Options = combat.ProjectileStopOptions{TriggerExpiry: true}
	combat.StopProjectile(stopCfg)
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
			worldpkg.StopLegacyProjectileOnExpiry(w.projectileStopAdapter, eff, now, w.stopLegacyProjectile)
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

func bindEffectHitAdapters(w *World) {
	if w == nil {
		return
	}
	if w.internalWorld == nil {
		w.effectHitAdapter = nil
		w.playerHitCallback = nil
		w.npcHitCallback = nil
		return
	}

	w.recordAttackOverlap = combat.NewAttackOverlapTelemetryRecorder(combat.AttackOverlapTelemetryRecorderConfig{
		Publisher: w.publisher,
		LookupEntity: func(id string) logging.EntityRef {
			return w.entityRef(id)
		},
	})

	combatCfg := worldpkg.EffectHitCombatDispatcherConfig{
		HealthEpsilon:           worldpkg.HealthEpsilon,
		BaselinePlayerMaxHealth: baselinePlayerMaxHealth,
		Publisher:               w.publisher,
		LookupEntity: func(id string) logging.EntityRef {
			return w.entityRef(id)
		},
		CurrentTick: func() uint64 {
			return w.currentTick
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
		ApplyGenericHealthDelta: func(actor *worldstate.ActorState, delta float64) (bool, float64, float64) {
			if actor == nil {
				return false, 0, 0
			}
			state := (*actorState)(actor)
			before := state.Health
			if !state.ApplyHealthDelta(delta) {
				return false, 0, before
			}
			return true, state.Health - before, state.Health
		},
		RecordEffectHitTelemetry: func(effect *worldeffects.State, targetID string, actualDelta float64) {
			if effect == nil || targetID == "" {
				return
			}
			w.recordEffectHitTelemetry((*effectState)(effect), targetID, actualDelta)
		},
		DropAllInventory: func(actor *worldstate.ActorState, reason string) {
			if actor == nil {
				return
			}
			w.dropAllInventory((*actorState)(actor), reason)
		},
		ApplyStatusEffect: func(effect *worldeffects.State, actor *worldstate.ActorState, status statuspkg.StatusEffectType, now time.Time) {
			if actor == nil || status == "" {
				return
			}
			ownerID := ""
			if effect != nil {
				ownerID = effect.Owner
			}
			w.applyStatusEffect((*actorState)(actor), StatusEffectType(status), ownerID, now)
		},
		IsPlayer: func(id string) bool {
			_, ok := w.players[id]
			return ok
		},
		IsNPC: func(id string) bool {
			_, ok := w.npcs[id]
			return ok
		},
	}

	npcCfg := worldpkg.EffectHitNPCConfig{
		SpawnBlood: func(effect any, target any, now time.Time) {
			eff, _ := effect.(*effectState)
			npc, _ := target.(*npcState)
			if eff == nil || npc == nil {
				return
			}
			w.maybeSpawnBloodSplatter(eff, npc, now)
		},
		IsAlive: func(target any) bool {
			npc, _ := target.(*npcState)
			if npc == nil {
				return false
			}
			return npc.Health > 0
		},
		HandleDefeat: func(target any) {
			npc, _ := target.(*npcState)
			if npc == nil {
				return
			}
			w.dropAllGold(&npc.ActorState, "death")
			w.handleNPCDefeat(npc)
		},
	}

	if ok := w.internalWorld.ConfigureEffectHitAdapters(worldpkg.EffectHitAdaptersConfig{
		Combat: combatCfg,
		NPC:    npcCfg,
	}); !ok {
		w.effectHitAdapter = nil
		w.playerHitCallback = nil
		w.npcHitCallback = nil
		return
	}

	w.effectHitAdapter = w.internalWorld.EffectHitDispatcher()
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
