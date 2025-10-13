package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"mine-and-die/server/logging"
	loggingcombat "mine-and-die/server/logging/combat"
	loggingeconomy "mine-and-die/server/logging/economy"
)

// Effect represents a time-limited gameplay artifact (attack swing, projectile, etc.).
type Effect struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Owner    string             `json:"owner"`
	Start    int64              `json:"start"`
	Duration int64              `json:"duration"`
	X        float64            `json:"x,omitempty"`
	Y        float64            `json:"y,omitempty"`
	Width    float64            `json:"width,omitempty"`
	Height   float64            `json:"height,omitempty"`
	Params   map[string]float64 `json:"params,omitempty"`
}

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
}

type effectState struct {
	Effect
	expiresAt             time.Time
	Projectile            *ProjectileState
	FollowActorID         string
	StatusEffect          StatusEffectType
	version               uint64
	telemetryEnded        bool
	contractManaged       bool
	telemetrySource       string
	telemetrySpawnTick    Tick
	telemetryFirstHitTick Tick
	telemetryHitCount     int
	telemetryVictims      map[string]struct{}
	telemetryDamage       float64
}

const (
	telemetrySourceLegacy   = "legacy"
	telemetrySourceContract = "contract"
)

type ProjectileTemplate struct {
	Type           string
	Speed          float64
	MaxDistance    float64
	Lifetime       time.Duration
	SpawnRadius    float64
	SpawnOffset    float64
	CollisionShape CollisionShapeConfig
	TravelMode     TravelModeConfig
	ImpactRules    ImpactRuleConfig
	Params         map[string]float64
	Cooldown       time.Duration
}

type CollisionShapeConfig struct {
	RectWidth  float64
	RectHeight float64
	UseRect    bool
}

type TravelModeConfig struct {
	StraightLine bool
}

type ImpactRuleConfig struct {
	StopOnHit          bool
	MaxTargets         int
	AffectsOwner       bool
	ExplodeOnImpact    *ExplosionSpec
	ExplodeOnExpiry    *ExplosionSpec
	ExpiryOnlyIfNoHits bool
}

type ExplosionSpec struct {
	EffectType string
	Radius     float64
	Duration   time.Duration
	Params     map[string]float64
}

type ProjectileState struct {
	Template       *ProjectileTemplate
	VelocityUnitX  float64
	VelocityUnitY  float64
	RemainingRange float64
	HitCount       int
	ExpiryResolved bool
	HitActors      map[string]struct{}
}

type projectileStopOptions struct {
	triggerImpact bool
	triggerExpiry bool
}

func (p *ProjectileState) markHit(id string) bool {
	if p == nil || id == "" {
		return false
	}
	if p.HitActors == nil {
		p.HitActors = make(map[string]struct{})
	}
	if _, exists := p.HitActors[id]; exists {
		return false
	}
	p.HitActors[id] = struct{}{}
	p.HitCount++
	return true
}

func (w *World) resolveMeleeImpact(effect *effectState, owner *actorState, actorID string, tick uint64, now time.Time, area Obstacle) {
	if w == nil || effect == nil {
		return
	}

	for _, obs := range w.obstacles {
		if obs.Type != obstacleTypeGoldOre {
			continue
		}
		if !obstaclesOverlap(area, obs, 0) {
			continue
		}
		var addErr error
		if _, ok := w.players[actorID]; ok {
			addErr = w.MutateInventory(actorID, func(inv *Inventory) error {
				if inv == nil {
					return nil
				}
				_, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
				return err
			})
		} else if owner != nil {
			_, addErr = owner.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
		}
		if addErr != nil {
			loggingeconomy.ItemGrantFailed(
				context.Background(),
				w.publisher,
				tick,
				w.entityRef(actorID),
				loggingeconomy.ItemGrantFailedPayload{ItemType: string(ItemTypeGold), Quantity: 1, Reason: "mine_gold"},
				map[string]any{"error": addErr.Error(), "obstacle": obs.ID},
			)
		}
		break
	}

	hitPlayerIDs := make([]string, 0)
	for id, target := range w.players {
		if id == actorID {
			continue
		}
		if circleRectOverlap(target.X, target.Y, playerHalf, area) {
			hitPlayerIDs = append(hitPlayerIDs, id)
			w.applyEffectHitPlayer(effect, target, now)
		}
	}

	hitNPCIDs := make([]string, 0)
	for id, target := range w.npcs {
		if id == actorID {
			continue
		}
		if circleRectOverlap(target.X, target.Y, playerHalf, area) {
			hitNPCIDs = append(hitNPCIDs, id)
			w.applyEffectHitNPC(effect, target, now)
		}
	}

	if len(hitPlayerIDs) == 0 && len(hitNPCIDs) == 0 {
		return
	}

	targets := make([]logging.EntityRef, 0, len(hitPlayerIDs)+len(hitNPCIDs))
	for _, id := range hitPlayerIDs {
		targets = append(targets, w.entityRef(id))
	}
	for _, id := range hitNPCIDs {
		targets = append(targets, w.entityRef(id))
	}
	payload := loggingcombat.AttackOverlapPayload{Ability: effect.Type}
	if len(hitPlayerIDs) > 0 {
		payload.PlayerHits = append(payload.PlayerHits, hitPlayerIDs...)
	}
	if len(hitNPCIDs) > 0 {
		payload.NPCHits = append(payload.NPCHits, hitNPCIDs...)
	}
	loggingcombat.AttackOverlap(
		context.Background(),
		w.publisher,
		tick,
		w.entityRef(actorID),
		targets,
		payload,
		nil,
	)
}

type effectBehavior interface {
	OnHit(w *World, eff *effectState, target *actorState, now time.Time)
}

type effectBehaviorFunc func(w *World, eff *effectState, target *actorState, now time.Time)

func (f effectBehaviorFunc) OnHit(w *World, eff *effectState, target *actorState, now time.Time) {
	f(w, eff, target, now)
}

const (
	meleeAttackCooldown = 400 * time.Millisecond
	meleeAttackDuration = 150 * time.Millisecond
	meleeAttackReach    = 56.0
	meleeAttackWidth    = 40.0
	meleeAttackDamage   = 10.0

	effectTypeAttack        = "attack"
	effectTypeFireball      = "fireball"
	effectTypeBloodSplatter = "blood-splatter"
	effectTypeBurningTick   = "burning-tick"
	effectTypeBurningVisual = "fire"

	bloodSplatterDuration = 1200 * time.Millisecond

	fireballCooldown = 650 * time.Millisecond
	fireballSpeed    = 320.0
	fireballRange    = 5 * 40.0
	fireballSize     = 24.0
	fireballSpawnGap = 6.0
	fireballDamage   = 15.0
)

var fireballLifetime = time.Duration(fireballRange / fireballSpeed * float64(time.Second))

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

func newEffectBehaviors() map[string]effectBehavior {
	return map[string]effectBehavior{
		effectTypeAttack:      healthDeltaBehavior("healthDelta", 0),
		effectTypeFireball:    damageAndStatusEffectBehavior("healthDelta", 0, StatusEffectBurning),
		effectTypeBurningTick: healthDeltaBehavior("healthDelta", 0),
	}
}

func healthDeltaBehavior(param string, fallback float64) effectBehavior {
	return effectBehaviorFunc(func(w *World, eff *effectState, target *actorState, now time.Time) {
		delta := fallback
		if eff != nil && eff.Params != nil {
			if value, ok := eff.Params[param]; ok {
				delta = value
			}
		}
		if delta == 0 || target == nil {
			return
		}

		var changed bool
		if w != nil {
			if player, ok := w.players[target.ID]; ok && player != nil {
				max := player.MaxHealth
				if max <= 0 {
					max = baselinePlayerMaxHealth
				}
				next := player.Health + delta
				if math.IsNaN(next) || math.IsInf(next, 0) {
					return
				}
				if next < 0 {
					next = 0
				} else if next > max {
					next = max
				}
				if math.Abs(next-player.Health) < healthEpsilon {
					return
				}
				actualDelta := next - player.Health
				w.SetHealth(player.ID, next)
				if actualDelta != 0 {
					w.recordEffectHitTelemetry(eff, player.ID, actualDelta)
				}
				changed = true
			} else if npc, ok := w.npcs[target.ID]; ok && npc != nil {
				max := npc.MaxHealth
				if max <= 0 {
					max = baselinePlayerMaxHealth
				}
				next := npc.Health + delta
				if math.IsNaN(next) || math.IsInf(next, 0) {
					return
				}
				if next < 0 {
					next = 0
				} else if next > max {
					next = max
				}
				if math.Abs(next-npc.Health) < healthEpsilon {
					return
				}
				actualDelta := next - npc.Health
				w.SetNPCHealth(npc.ID, next)
				if actualDelta != 0 {
					w.recordEffectHitTelemetry(eff, npc.ID, actualDelta)
				}
				changed = true
			} else {
				before := target.Health
				changed = target.applyHealthDelta(delta)
				if changed {
					w.recordEffectHitTelemetry(eff, target.ID, target.Health-before)
				}
			}
		} else {
			before := target.Health
			changed = target.applyHealthDelta(delta)
			if changed {
				w.recordEffectHitTelemetry(eff, target.ID, target.Health-before)
			}
		}

		if !changed {
			return
		}
		if w != nil && delta < 0 {
			ability := ""
			actorRef := logging.EntityRef{}
			conditionName := ""
			if eff != nil {
				ability = eff.Type
				actorRef = w.entityRef(eff.Owner)
				if eff.StatusEffect != "" {
					conditionName = string(eff.StatusEffect)
				}
			}
			targetRef := logging.EntityRef{}
			if target != nil {
				targetRef = w.entityRef(target.ID)
			}
			loggingcombat.Damage(
				context.Background(),
				w.publisher,
				w.currentTick,
				actorRef,
				targetRef,
				loggingcombat.DamagePayload{
					Ability:      ability,
					Amount:       -delta,
					TargetHealth: target.Health,
					StatusEffect: conditionName,
				},
				nil,
			)
			if target.Health <= 0 {
				loggingcombat.Defeat(
					context.Background(),
					w.publisher,
					w.currentTick,
					actorRef,
					targetRef,
					loggingcombat.DefeatPayload{Ability: ability, StatusEffect: conditionName},
					nil,
				)
				w.dropAllInventory(target, "death")
			}
		}
	})
}

func damageAndStatusEffectBehavior(param string, fallback float64, statusEffect StatusEffectType) effectBehavior {
	base := healthDeltaBehavior(param, fallback)
	return effectBehaviorFunc(func(w *World, eff *effectState, target *actorState, now time.Time) {
		if base != nil {
			base.OnHit(w, eff, target, now)
		}
		if w == nil || target == nil || statusEffect == "" {
			return
		}
		source := ""
		if eff != nil {
			source = eff.Owner
		}
		w.applyStatusEffect(target, statusEffect, source, now)
	})
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
	if !eff.telemetryEnded {
		w.flushEffectTelemetry(eff)
		eff.telemetryEnded = true
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

func (w *World) effectTelemetrySource(eff *effectState) string {
	if eff == nil {
		return telemetrySourceLegacy
	}
	if eff.telemetrySource != "" {
		return eff.telemetrySource
	}
	if eff.contractManaged {
		eff.telemetrySource = telemetrySourceContract
	} else {
		eff.telemetrySource = telemetrySourceLegacy
	}
	return eff.telemetrySource
}

func (w *World) recordEffectHitTelemetry(eff *effectState, targetID string, delta float64) {
	if w == nil || eff == nil {
		return
	}
	if eff.telemetrySpawnTick == 0 {
		eff.telemetrySpawnTick = Tick(int64(w.currentTick))
	}
	if eff.telemetryFirstHitTick == 0 {
		eff.telemetryFirstHitTick = Tick(int64(w.currentTick))
	}
	eff.telemetryHitCount++
	if eff.telemetryVictims == nil {
		eff.telemetryVictims = make(map[string]struct{})
	}
	if targetID != "" {
		eff.telemetryVictims[targetID] = struct{}{}
	}
	if delta < 0 {
		eff.telemetryDamage += -delta
	}
	w.effectTelemetrySource(eff)
}

func (w *World) flushEffectTelemetry(eff *effectState) {
	if w == nil || eff == nil || w.telemetry == nil {
		return
	}
	source := w.effectTelemetrySource(eff)
	victims := 0
	if len(eff.telemetryVictims) > 0 {
		victims = len(eff.telemetryVictims)
	}
	spawnTick := eff.telemetrySpawnTick
	if spawnTick == 0 {
		spawnTick = Tick(int64(w.currentTick))
	}
	summary := effectParitySummary{
		EffectType:    eff.Type,
		Source:        source,
		Hits:          eff.telemetryHitCount,
		UniqueVictims: victims,
		TotalDamage:   eff.telemetryDamage,
		SpawnTick:     spawnTick,
		FirstHitTick:  eff.telemetryFirstHitTick,
	}
	w.telemetry.RecordEffectParity(summary)
	eff.telemetryHitCount = 0
	eff.telemetryDamage = 0
	eff.telemetryVictims = nil
	eff.telemetryFirstHitTick = 0
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

	facing := state.Facing
	if facing == "" {
		facing = defaultFacing
	}

	rectX, rectY, rectW, rectH := meleeAttackRectangle(state.X, state.Y, facing)
	area := Obstacle{X: rectX, Y: rectY, Width: rectW, Height: rectH}

	w.pruneEffects(now)
	w.nextEffectID++
	effect := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", w.nextEffectID),
			Type:     effectTypeAttack,
			Owner:    actorID,
			Start:    now.UnixMilli(),
			Duration: meleeAttackDuration.Milliseconds(),
			X:        rectX,
			Y:        rectY,
			Width:    rectW,
			Height:   rectH,
			Params: map[string]float64{
				"healthDelta": -meleeAttackDamage,
				"reach":       meleeAttackReach,
				"width":       meleeAttackWidth,
			},
		},
		expiresAt:          now.Add(meleeAttackDuration),
		telemetrySource:    telemetrySourceLegacy,
		telemetrySpawnTick: Tick(int64(tick)),
	}

	w.effects = append(w.effects, effect)
	w.recordEffectSpawn(effectTypeAttack, "melee")

	useContract := enableContractMeleeDefinitions && enableContractEffectManager && w.effectManager != nil
	if !useContract {
		w.resolveMeleeImpact(effect, state, actorID, tick, now, area)
	}

	return true
}

// triggerFireball launches a projectile effect when the player is ready.
func (w *World) triggerFireball(actorID string, now time.Time) bool {
	useContract := enableContractEffectManager && enableContractProjectileDefinitions && w.effectManager != nil
	if !useContract {
		_, spawned := w.spawnProjectile(actorID, effectTypeFireball, now)
		return spawned
	}

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

	return true
}

func (w *World) spawnProjectile(actorID, projectileType string, now time.Time) (*effectState, bool) {
	tpl := w.projectileTemplates[projectileType]
	if tpl == nil {
		return nil, false
	}

	owner, cooldowns := w.abilityOwner(actorID)
	if owner == nil || cooldowns == nil {
		return nil, false
	}

	if !w.cooldownReady(cooldowns, tpl.Type, tpl.Cooldown, now) {
		return nil, false
	}

	w.pruneEffects(now)
	w.nextEffectID++
	effectID := fmt.Sprintf("effect-%d", w.nextEffectID)
	effect := w.buildProjectileEffect(owner, actorID, tpl, now, effectID)
	if effect == nil {
		return nil, false
	}

	w.effects = append(w.effects, effect)
	w.recordEffectSpawn(tpl.Type, "projectile")
	return effect, true
}

func (w *World) buildProjectileEffect(owner *actorState, actorID string, tpl *ProjectileTemplate, now time.Time, effectID string) *effectState {
	if owner == nil || tpl == nil {
		return nil
	}

	facing := owner.Facing
	if facing == "" {
		facing = defaultFacing
	}
	dirX, dirY := facingToVector(facing)
	if dirX == 0 && dirY == 0 {
		dirX, dirY = 0, 1
	}

	spawnRadius := sanitizedSpawnRadius(tpl.SpawnRadius)
	spawnOffset := tpl.SpawnOffset
	if spawnOffset == 0 {
		spawnOffset = ownerHalfExtent(owner) + spawnRadius
	}

	centerX := owner.X + dirX*spawnOffset
	centerY := owner.Y + dirY*spawnOffset
	width, height := spawnSizeFromShape(tpl)

	lifetime := effectLifetime(tpl)
	params := mergeParams(tpl.Params, map[string]float64{
		"speed":  tpl.Speed,
		"radius": spawnRadius,
		"dx":     dirX,
		"dy":     dirY,
	})

	effect := &effectState{
		Effect: Effect{
			ID:       effectID,
			Type:     tpl.Type,
			Owner:    actorID,
			Start:    now.UnixMilli(),
			Duration: lifetime.Milliseconds(),
			X:        centerX - width/2,
			Y:        centerY - height/2,
			Width:    width,
			Height:   height,
			Params:   params,
		},
		expiresAt: now.Add(lifetime),
		Projectile: &ProjectileState{
			Template:       tpl,
			VelocityUnitX:  dirX,
			VelocityUnitY:  dirY,
			RemainingRange: tpl.MaxDistance,
		},
		telemetrySource:    telemetrySourceLegacy,
		telemetrySpawnTick: Tick(int64(w.currentTick)),
	}

	return effect
}

func (w *World) spawnContractProjectileFromInstance(instance *EffectInstance, owner *actorState, tpl *ProjectileTemplate, now time.Time) *effectState {
	if w == nil || instance == nil || owner == nil || tpl == nil {
		return nil
	}

	params := intMapToFloat64(instance.BehaviorState.Extra)
	if params == nil {
		params = make(map[string]float64)
	}

	dirX := params["dx"]
	dirY := params["dy"]
	if dirX == 0 && dirY == 0 {
		facing := owner.Facing
		if facing == "" {
			facing = defaultFacing
		}
		dirX, dirY = facingToVector(facing)
		if dirX == 0 && dirY == 0 {
			dirX, dirY = 0, 1
		}
	}

	geometry := instance.DeliveryState.Geometry
	offsetX := dequantizeWorldCoord(geometry.OffsetX)
	offsetY := dequantizeWorldCoord(geometry.OffsetY)
	centerX := owner.X + offsetX
	centerY := owner.Y + offsetY

	width, height := spawnSizeFromShape(tpl)
	if geometry.Width != 0 {
		width = dequantizeWorldCoord(geometry.Width)
	}
	if geometry.Height != 0 {
		height = dequantizeWorldCoord(geometry.Height)
	}

	radius := sanitizedSpawnRadius(tpl.SpawnRadius)
	if geometry.Radius != 0 {
		radius = dequantizeWorldCoord(geometry.Radius)
	} else if val, ok := params["radius"]; ok && val > 0 {
		radius = val
	}

	lifetime := effectLifetime(tpl)
	params = mergeParams(params, map[string]float64{
		"speed":  tpl.Speed,
		"radius": radius,
		"dx":     dirX,
		"dy":     dirY,
	})
	if _, ok := params["range"]; !ok && tpl.MaxDistance > 0 {
		params["range"] = tpl.MaxDistance
	}

	effect := &effectState{
		Effect: Effect{
			ID:       instance.ID,
			Type:     tpl.Type,
			Owner:    instance.OwnerActorID,
			Start:    now.UnixMilli(),
			Duration: lifetime.Milliseconds(),
			X:        centerX - width/2,
			Y:        centerY - height/2,
			Width:    width,
			Height:   height,
			Params:   params,
		},
		expiresAt: now.Add(lifetime),
		Projectile: &ProjectileState{
			Template:       tpl,
			VelocityUnitX:  dirX,
			VelocityUnitY:  dirY,
			RemainingRange: tpl.MaxDistance,
		},
		contractManaged:    true,
		telemetrySource:    telemetrySourceContract,
		telemetrySpawnTick: instance.StartTick,
	}

	if remaining, ok := params["range"]; ok && remaining >= 0 {
		effect.Projectile.RemainingRange = remaining
	}

	w.pruneEffects(now)
	w.effects = append(w.effects, effect)
	w.recordEffectSpawn(tpl.Type, "projectile")
	return effect
}

func (w *World) spawnContractBloodDecalFromInstance(instance *EffectInstance, now time.Time) *effectState {
	if w == nil || instance == nil {
		return nil
	}
	params := instance.BehaviorState.Extra
	if len(params) == 0 {
		return nil
	}
	centerXVal, okX := params["centerX"]
	centerYVal, okY := params["centerY"]
	if !okX || !okY {
		return nil
	}
	centerX := dequantizeWorldCoord(centerXVal)
	centerY := dequantizeWorldCoord(centerYVal)
	width := dequantizeWorldCoord(instance.DeliveryState.Geometry.Width)
	if width <= 0 {
		width = playerHalf * 2
	}
	height := dequantizeWorldCoord(instance.DeliveryState.Geometry.Height)
	if height <= 0 {
		height = playerHalf * 2
	}
	lifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
	if lifetime <= 0 {
		lifetime = bloodSplatterDuration
	}
	if lifetime <= 0 {
		lifetime = time.Millisecond
	}
	effectType := instance.DefinitionID
	if effectType == "" {
		effectType = effectTypeBloodSplatter
	}
	w.pruneEffects(now)
	effect := &effectState{
		Effect: Effect{
			ID:       instance.ID,
			Type:     effectType,
			Owner:    instance.OwnerActorID,
			Start:    now.UnixMilli(),
			Duration: lifetime.Milliseconds(),
			X:        centerX - width/2,
			Y:        centerY - height/2,
			Width:    width,
			Height:   height,
		},
		expiresAt:          now.Add(lifetime),
		contractManaged:    true,
		telemetrySource:    telemetrySourceContract,
		telemetrySpawnTick: instance.StartTick,
	}
	w.effects = append(w.effects, effect)
	w.recordEffectSpawn(effectType, "blood-decal")
	return effect
}

func spawnSizeFromShape(tpl *ProjectileTemplate) (float64, float64) {
	if tpl == nil {
		return 0, 0
	}
	if tpl.CollisionShape.UseRect {
		spawnDiameter := sanitizedSpawnRadius(tpl.SpawnRadius) * 2
		width := math.Max(tpl.CollisionShape.RectWidth, spawnDiameter)
		height := math.Max(tpl.CollisionShape.RectHeight, spawnDiameter)
		width = math.Max(width, 1)
		height = math.Max(height, 1)
		return width, height
	}
	radius := sanitizedSpawnRadius(tpl.SpawnRadius)
	diameter := radius * 2
	return diameter, diameter
}

func sanitizedSpawnRadius(value float64) float64 {
	if value < 1 {
		return 1
	}
	return value
}

func ownerHalfExtent(owner *actorState) float64 {
	if owner == nil {
		return playerHalf
	}
	return playerHalf
}

func effectLifetime(tpl *ProjectileTemplate) time.Duration {
	if tpl == nil {
		return 0
	}
	if tpl.Lifetime > 0 {
		return tpl.Lifetime
	}
	if tpl.Speed <= 0 || tpl.MaxDistance <= 0 {
		return 0
	}
	seconds := tpl.MaxDistance / tpl.Speed
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func mergeParams(base map[string]float64, overrides map[string]float64) map[string]float64 {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]float64)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
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
func (w *World) advanceEffects(now time.Time, dt float64) {
	if len(w.effects) == 0 {
		return
	}
	w.advanceProjectiles(now, dt)
	w.advanceNonProjectiles(now, dt)
}

func (w *World) advanceProjectiles(now time.Time, dt float64) {
	if len(w.effects) == 0 {
		return
	}
	for _, eff := range w.effects {
		if eff == nil {
			continue
		}
		if eff.contractManaged {
			continue
		}
		p := eff.Projectile
		if p == nil {
			continue
		}
		if !now.Before(eff.expiresAt) {
			w.stopProjectile(eff, now, projectileStopOptions{triggerExpiry: true})
			continue
		}
		w.advanceProjectile(eff, now, dt)
	}
}

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
	if eff.Effect.X < 0 || eff.Effect.Y < 0 || eff.Effect.X+eff.Effect.Width > worldW || eff.Effect.Y+eff.Effect.Height > worldH {
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
		if !p.markHit(id) {
			continue
		}
		hitPlayers = append(hitPlayers, id)
		w.applyEffectHitPlayer(eff, target, now)
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
			if !p.markHit(id) {
				continue
			}
			hitNPCs = append(hitNPCs, id)
			w.applyEffectHitNPC(eff, target, now)
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
				map[string]any{"projectile": eff.Effect.Type},
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
	width := eff.Effect.Width
	height := eff.Effect.Height
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
		if eff.expiresAt.After(now) {
			eff.expiresAt = now
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
	eff.expiresAt = now
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
	source := w.effectTelemetrySource(eff)
	radius := spec.Radius
	size := radius * 2
	if size <= 0 {
		size = eff.Effect.Width
		if size <= 0 {
			size = 1
		}
	}
	params := mergeParams(spec.Params, map[string]float64{
		"radius": radius,
	})
	if spec.Duration > 0 {
		if params == nil {
			params = make(map[string]float64)
		}
		params["duration_ms"] = float64(spec.Duration.Milliseconds())
	}

	w.nextEffectID++
	area := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", w.nextEffectID),
			Type:     spec.EffectType,
			Owner:    eff.Owner,
			Start:    now.UnixMilli(),
			Duration: spec.Duration.Milliseconds(),
			X:        centerX(eff) - size/2,
			Y:        centerY(eff) - size/2,
			Width:    size,
			Height:   size,
			Params:   params,
		},
		expiresAt:          now.Add(spec.Duration),
		telemetrySource:    source,
		telemetrySpawnTick: Tick(int64(w.currentTick)),
	}
	w.effects = append(w.effects, area)
	w.recordEffectSpawn(spec.EffectType, "explosion")
	w.mirrorLegacyAreaEffect(area)
}

func (w *World) mirrorLegacyAreaEffect(effect *effectState) {
	if w == nil || effect == nil || w.legacyCompat == nil {
		return
	}
	w.legacyCompat.mirrorAreaEffect(effect)
}

func (w *World) mirrorLegacyEffectEnd(effect *effectState, reason string) {
	if w == nil || effect == nil || w.legacyCompat == nil {
		return
	}
	w.legacyCompat.noteLegacyEffectEnd(effect, legacyEndReason(reason))
}

func legacyEndReason(reason string) EndReason {
	switch reason {
	case "impact":
		return EndReasonExpired
	case "stopped":
		return EndReasonCancelled
	case "expiry", "expired":
		return EndReasonExpired
	default:
		return EndReasonExpired
	}
}

func (w *World) maybeExplodeOnExpiry(eff *effectState, now time.Time) {
	w.stopProjectile(eff, now, projectileStopOptions{triggerExpiry: true})
}

func effectAABB(eff *effectState) Obstacle {
	if eff == nil {
		return Obstacle{}
	}
	return Obstacle{X: eff.Effect.X, Y: eff.Effect.Y, Width: eff.Effect.Width, Height: eff.Effect.Height}
}

func (w *World) findEffectByID(id string) *effectState {
	if w == nil || id == "" {
		return nil
	}
	for _, eff := range w.effects {
		if eff == nil {
			continue
		}
		if eff.ID == id {
			return eff
		}
	}
	return nil
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
	return eff.Effect.X + eff.Effect.Width/2
}

func centerY(eff *effectState) float64 {
	return eff.Effect.Y + eff.Effect.Height/2
}

// pruneEffects drops expired effects from the in-memory list.
func (w *World) pruneEffects(now time.Time) {
	if len(w.effects) == 0 {
		return
	}
	current := w.effects
	w.effects = w.effects[:0]
	for _, eff := range current {
		if now.Before(eff.expiresAt) {
			w.effects = append(w.effects, eff)
			continue
		}
		if eff.Projectile != nil && !eff.Projectile.ExpiryResolved {
			w.maybeExplodeOnExpiry(eff, now)
		}
		w.mirrorLegacyEffectEnd(eff, "expired")
		w.recordEffectEnd(eff, "expired")
		w.purgeEntityPatches(eff.ID)
	}
}

func (w *World) applyEffectHitPlayer(eff *effectState, target *playerState, now time.Time) {
	if target == nil {
		return
	}
	w.applyEffectHitActor(eff, &target.actorState, now)
}

func (w *World) applyEffectHitNPC(eff *effectState, target *npcState, now time.Time) {
	if target == nil {
		return
	}
	w.maybeSpawnBloodSplatter(eff, target, now)
	wasAlive := target.Health > 0
	w.applyEffectHitActor(eff, &target.actorState, now)
	if wasAlive && target.Health <= 0 {
		w.handleNPCDefeat(target)
	}
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

	if enableContractEffectManager && w.effectManager != nil {
		if intent, ok := NewBloodSplatterIntent(eff.Owner, &target.actorState); ok {
			w.effectManager.EnqueueIntent(intent)
		}
	}

	if w.contractBloodDecalsEnabled() {
		return
	}

	trigger := EffectTrigger{
		Type:     effectTypeBloodSplatter,
		Start:    now.UnixMilli(),
		Duration: bloodSplatterDuration.Milliseconds(),
		X:        target.X - playerHalf,
		Y:        target.Y - playerHalf,
		Width:    playerHalf * 2,
		Height:   playerHalf * 2,
	}

	w.QueueEffectTrigger(trigger, now)
}

func (w *World) contractBloodDecalsEnabled() bool {
	if w == nil {
		return false
	}
	if !enableContractEffectManager || !enableContractBloodDecalDefinitions {
		return false
	}
	return w.effectManager != nil
}

func (w *World) applyEffectHitActor(eff *effectState, target *actorState, now time.Time) {
	if eff == nil || target == nil {
		return
	}
	behavior, ok := w.effectBehaviors[eff.Type]
	if !ok || behavior == nil {
		return
	}
	behavior.OnHit(w, eff, target, now)
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
