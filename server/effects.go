package server

import (
	"context"
	"fmt"
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
	loggingcombat "mine-and-die/server/logging/combat"
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
	meleeAttackCooldown = 400 * time.Millisecond
	meleeAttackDuration = 150 * time.Millisecond
	meleeAttackReach    = 56.0
	meleeAttackWidth    = 40.0
	meleeAttackDamage   = 10.0

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

func (w *World) abilityOwner(actorID string) (*actorState, *map[string]time.Time) {
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

func (w *World) cooldownReady(cooldowns *map[string]time.Time, ability string, cooldown time.Duration, now time.Time) bool {
	if cooldowns == nil {
		return false
	}
	if *cooldowns == nil {
		*cooldowns = make(map[string]time.Time)
	}
	if cooldown > 0 {
		if last, ok := (*cooldowns)[ability]; ok {
			if now.Sub(last) < cooldown {
				return false
			}
		}
	}
	(*cooldowns)[ability] = now
	return true
}

// triggerMeleeAttack spawns a short-lived melee hitbox if the cooldown allows it.
func (w *World) triggerMeleeAttack(actorID string, tick uint64, now time.Time) bool {
	state, cooldowns := w.abilityOwner(actorID)
	if state == nil || cooldowns == nil {
		return false
	}

	if !w.cooldownReady(cooldowns, effectTypeAttack, meleeAttackCooldown, now) {
		return false
	}

	return w.effectManager != nil
}

func contractSpawnProducer(definitionID string) string {
	switch definitionID {
	case effectTypeAttack:
		return "melee"
	default:
		return ""
	}
}

// triggerFireball launches a projectile effect when the player is ready.
func (w *World) triggerFireball(actorID string, now time.Time) bool {
	tpl := w.projectileTemplates[effectTypeFireball]
	if tpl == nil {
		return false
	}

	owner, cooldowns := w.abilityOwner(actorID)
	if owner == nil || cooldowns == nil {
		return false
	}

	if !w.cooldownReady(cooldowns, tpl.Type, tpl.Cooldown, now) {
		return false
	}

	return w.effectManager != nil
}

func ownerHalfExtent(owner *actorState) float64 {
	if owner == nil {
		return playerHalf
	}
	return playerHalf
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

// meleeAttackRectangle builds the hitbox in front of a player for a melee swing.
func meleeAttackRectangle(x, y float64, facing FacingDirection) (float64, float64, float64, float64) {
	reach := meleeAttackReach
	thickness := meleeAttackWidth

	switch facing {
	case FacingUp:
		return x - thickness/2, y - playerHalf - reach, thickness, reach
	case FacingDown:
		return x - thickness/2, y + playerHalf, thickness, reach
	case FacingLeft:
		return x - playerHalf - reach, y - thickness/2, reach, thickness
	case FacingRight:
		return x + playerHalf, y - thickness/2, reach, thickness
	default:
		return x - thickness/2, y + playerHalf, thickness, reach
	}
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
	p := eff.Projectile
	tpl := p.Template
	if tpl == nil {
		w.stopProjectile(eff, now, projectileStopOptions{})
		return true
	}

	if tpl.TravelMode.StraightLine && tpl.Speed > 0 && dt > 0 {
		distance := tpl.Speed * dt
		if p.RemainingRange > 0 && distance > p.RemainingRange {
			distance = p.RemainingRange
		}
		if distance > 0 {
			newX := eff.X + p.VelocityUnitX*distance
			newY := eff.Y + p.VelocityUnitY*distance
			w.SetEffectPosition(eff, newX, newY)
			if p.RemainingRange > 0 {
				previous := p.RemainingRange
				p.RemainingRange -= distance
				if p.RemainingRange < 0 {
					p.RemainingRange = 0
				}
				if math.Abs(previous-p.RemainingRange) > 1e-9 {
					w.SetEffectParam(eff, "remainingRange", p.RemainingRange)
				}
			}
		}
	}

	if tpl.MaxDistance > 0 && p.RemainingRange <= 0 {
		w.stopProjectile(eff, now, projectileStopOptions{triggerExpiry: true})
		return true
	}

	worldW, worldH := w.dimensions()
	if eff.X < 0 || eff.Y < 0 || eff.X+eff.Width > worldW || eff.Y+eff.Height > worldH {
		w.stopProjectile(eff, now, projectileStopOptions{triggerExpiry: true})
		return true
	}

	area := effectAABB(eff)
	if w.anyObstacleOverlap(area) {
		w.stopProjectile(eff, now, projectileStopOptions{triggerImpact: true})
		return true
	}

	hitPlayers := make([]string, 0)
	hitNPCs := make([]string, 0)
	maxTargets := tpl.ImpactRules.MaxTargets
	shouldStop := false
	hitCountAtStart := p.HitCount

	for id, target := range w.players {
		if target == nil {
			continue
		}
		if id == eff.Owner && !tpl.ImpactRules.AffectsOwner {
			continue
		}
		if !circleRectOverlap(target.X, target.Y, playerHalf, area) {
			continue
		}
		if !p.MarkHit(id) {
			continue
		}
		hitPlayers = append(hitPlayers, id)
		w.invokePlayerHitCallback(eff, target, now)
		if tpl.ImpactRules.StopOnHit || (maxTargets > 0 && p.HitCount >= maxTargets) {
			shouldStop = true
		}
		if shouldStop {
			break
		}
	}

	if !shouldStop {
		for id, target := range w.npcs {
			if target == nil {
				continue
			}
			if id == eff.Owner && !tpl.ImpactRules.AffectsOwner {
				continue
			}
			if !circleRectOverlap(target.X, target.Y, playerHalf, area) {
				continue
			}
			if !p.MarkHit(id) {
				continue
			}
			hitNPCs = append(hitNPCs, id)
			w.invokeNPCHitCallback(eff, target, now)
			if tpl.ImpactRules.StopOnHit || (maxTargets > 0 && p.HitCount >= maxTargets) {
				shouldStop = true
			}
			if shouldStop {
				break
			}
		}
	}

	hitsApplied := p.HitCount - hitCountAtStart
	if hitsApplied > 0 {
		if tpl.ImpactRules.ExplodeOnImpact != nil {
			w.spawnAreaEffectAt(eff, now, tpl.ImpactRules.ExplodeOnImpact)
		}
		if len(hitPlayers) > 0 || len(hitNPCs) > 0 {
			targets := make([]logging.EntityRef, 0, len(hitPlayers)+len(hitNPCs))
			for _, id := range hitPlayers {
				targets = append(targets, w.entityRef(id))
			}
			for _, id := range hitNPCs {
				targets = append(targets, w.entityRef(id))
			}
			payload := loggingcombat.AttackOverlapPayload{Ability: eff.Type}
			if len(hitPlayers) > 0 {
				payload.PlayerHits = append(payload.PlayerHits, hitPlayers...)
			}
			if len(hitNPCs) > 0 {
				payload.NPCHits = append(payload.NPCHits, hitNPCs...)
			}
			loggingcombat.AttackOverlap(
				context.Background(),
				w.publisher,
				w.currentTick,
				w.entityRef(eff.Owner),
				targets,
				payload,
				map[string]any{"projectile": eff.Type},
			)
		}
	}

	if shouldStop {
		w.stopProjectile(eff, now, projectileStopOptions{})
		return true
	}
	return false
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

func (w *World) configureEffectHitAdapter() {
	if w == nil {
		return
	}

	w.effectHitAdapter = combat.NewEffectHitDispatcher(combat.EffectHitDispatcherConfig{
		ExtractEffect: func(effect any) (combat.EffectRef, bool) {
			eff, _ := effect.(*effectState)
			if eff == nil {
				return combat.EffectRef{}, false
			}
			status := ""
			if eff.StatusEffect != "" {
				status = string(eff.StatusEffect)
			}
			return combat.EffectRef{
				Effect: combat.Effect{
					Type:         eff.Type,
					OwnerID:      eff.Owner,
					Params:       eff.Params,
					StatusEffect: status,
				},
				Raw: eff,
			}, true
		},
		ExtractActor: func(target any) (combat.ActorRef, bool) {
			actor, _ := target.(*actorState)
			if actor == nil {
				return combat.ActorRef{}, false
			}
			kind := combat.ActorKindGeneric
			if _, ok := w.players[actor.ID]; ok {
				kind = combat.ActorKindPlayer
			} else if _, ok := w.npcs[actor.ID]; ok {
				kind = combat.ActorKindNPC
			}
			return combat.ActorRef{
				Actor: combat.Actor{
					ID:        actor.ID,
					Health:    actor.Health,
					MaxHealth: actor.MaxHealth,
					Kind:      kind,
				},
				Raw: actor,
			}, true
		},
		HealthEpsilon:           worldpkg.HealthEpsilon,
		BaselinePlayerMaxHealth: baselinePlayerMaxHealth,
		SetPlayerHealth: func(target combat.ActorRef, next float64) {
			if target.Actor.ID == "" {
				return
			}
			w.SetHealth(target.Actor.ID, next)
		},
		SetNPCHealth: func(target combat.ActorRef, next float64) {
			if target.Actor.ID == "" {
				return
			}
			w.SetNPCHealth(target.Actor.ID, next)
		},
		ApplyGenericHealthDelta: func(target combat.ActorRef, delta float64) (bool, float64, float64) {
			actor, _ := target.Raw.(*actorState)
			if actor == nil {
				return false, 0, target.Actor.Health
			}
			before := actor.Health
			if !actor.applyHealthDelta(delta) {
				return false, 0, before
			}
			return true, actor.Health - before, actor.Health
		},
		RecordEffectHitTelemetry: func(effect combat.EffectRef, target combat.ActorRef, actualDelta float64) {
			eff, _ := effect.Raw.(*effectState)
			if eff == nil || target.Actor.ID == "" {
				return
			}
			w.recordEffectHitTelemetry(eff, target.Actor.ID, actualDelta)
		},
		RecordDamageTelemetry: func(effect combat.EffectRef, target combat.ActorRef, damage float64, targetHealth float64, statusEffect string) {
			if w == nil || damage <= 0 {
				return
			}
			actorRef := logging.EntityRef{}
			targetRef := logging.EntityRef{}
			if effect.Effect.OwnerID != "" {
				actorRef = w.entityRef(effect.Effect.OwnerID)
			}
			if target.Actor.ID != "" {
				targetRef = w.entityRef(target.Actor.ID)
			}
			loggingcombat.Damage(
				context.Background(),
				w.publisher,
				w.currentTick,
				actorRef,
				targetRef,
				loggingcombat.DamagePayload{
					Ability:      effect.Effect.Type,
					Amount:       damage,
					TargetHealth: targetHealth,
					StatusEffect: statusEffect,
				},
				nil,
			)
		},
		RecordDefeatTelemetry: func(effect combat.EffectRef, target combat.ActorRef, statusEffect string) {
			if w == nil {
				return
			}
			actorRef := logging.EntityRef{}
			targetRef := logging.EntityRef{}
			if effect.Effect.OwnerID != "" {
				actorRef = w.entityRef(effect.Effect.OwnerID)
			}
			if target.Actor.ID != "" {
				targetRef = w.entityRef(target.Actor.ID)
			}
			loggingcombat.Defeat(
				context.Background(),
				w.publisher,
				w.currentTick,
				actorRef,
				targetRef,
				loggingcombat.DefeatPayload{Ability: effect.Effect.Type, StatusEffect: statusEffect},
				nil,
			)
		},
		DropAllInventory: func(target combat.ActorRef, reason string) {
			actor, _ := target.Raw.(*actorState)
			if actor == nil {
				return
			}
			w.dropAllInventory(actor, reason)
		},
		ApplyStatusEffect: func(effect combat.EffectRef, target combat.ActorRef, statusEffect string, now time.Time) {
			if statusEffect == "" {
				return
			}
			actor, _ := target.Raw.(*actorState)
			if actor == nil {
				return
			}
			w.applyStatusEffect(actor, StatusEffectType(statusEffect), effect.Effect.OwnerID, now)
		},
	})
}

func (w *World) applyEffectHitActor(eff *effectState, target *actorState, now time.Time) {
	if w == nil || w.effectHitAdapter == nil || eff == nil || target == nil {
		return
	}
	w.effectHitAdapter(eff, target, now)
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
