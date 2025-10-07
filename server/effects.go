package main

import (
	"fmt"
	"log"
	"time"
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

type ProjectileTemplate struct {
	Type           string
	Cooldown       time.Duration
	Speed          float64
	MaxDistance    float64
	Lifetime       time.Duration
	SpawnRadius    float64
	SpawnOffset    float64
	CollisionShape CollisionShapeConfig
	TravelMode     TravelModeConfig
	ImpactRules    ImpactRuleConfig
	Params         map[string]float64
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
	StopOnHit       bool
	MaxTargets      int
	AffectsOwner    bool
	ExplodeOnImpact *ExplosionSpec
	ExplodeOnExpiry *ExplosionSpec
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
	Expired        bool
}

type effectState struct {
	Effect
	expiresAt  time.Time
	Projectile *ProjectileState
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

	effectTypeAttack   = "attack"
	effectTypeFireball = "fireball"

	fireballCooldown = 650 * time.Millisecond
	fireballSpeed    = 320.0
	fireballRange    = 5 * 40.0
	fireballSize     = 24.0
	fireballSpawnGap = 6.0
	fireballDamage   = 15.0
)

var fireballLifetime = time.Duration(fireballRange / fireballSpeed * float64(time.Second))

func newEffectBehaviors() map[string]effectBehavior {
	return map[string]effectBehavior{
		effectTypeAttack:   healthDeltaBehavior("healthDelta", 0),
		effectTypeFireball: healthDeltaBehavior("healthDelta", 0),
	}
}

func newProjectileTemplates() map[string]*ProjectileTemplate {
	return map[string]*ProjectileTemplate{
		effectTypeFireball: {
			Type:        effectTypeFireball,
			Cooldown:    fireballCooldown,
			Speed:       fireballSpeed,
			MaxDistance: fireballRange,
			Lifetime:    fireballLifetime,
			SpawnRadius: fireballSize / 2,
			SpawnOffset: playerHalf + fireballSpawnGap + fireballSize/2,
			TravelMode: TravelModeConfig{
				StraightLine: true,
			},
			ImpactRules: ImpactRuleConfig{
				StopOnHit:  true,
				MaxTargets: 1,
			},
			Params: map[string]float64{
				"healthDelta": -fireballDamage,
				"range":       fireballRange,
			},
		},
	}
}

func mergeParams(base map[string]float64, overrides map[string]float64) map[string]float64 {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]float64, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
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

func spawnSizeFromShape(tpl *ProjectileTemplate) (float64, float64) {
	if tpl == nil {
		return 0, 0
	}
	if tpl.CollisionShape.UseRect {
		return tpl.CollisionShape.RectWidth, tpl.CollisionShape.RectHeight
	}
	radius := tpl.SpawnRadius
	if radius <= 0 {
		return 0, 0
	}
	diameter := radius * 2
	return diameter, diameter
}

func healthDeltaBehavior(param string, fallback float64) effectBehavior {
	return effectBehaviorFunc(func(w *World, eff *effectState, target *actorState, now time.Time) {
		delta := fallback
		if eff != nil && eff.Params != nil {
			if value, ok := eff.Params[param]; ok {
				delta = value
			}
		}
		if delta == 0 {
			return
		}
		if target.applyHealthDelta(delta) {
			if delta < 0 && target.Health <= 0 {
				log.Printf("%s defeated %s with %s", eff.Owner, target.ID, eff.Type)
			}
		}
	})
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

func (w *World) cooldownReady(cooldowns *map[string]time.Time, ability string, cooldown time.Duration, now time.Time) bool {
	if cooldowns == nil {
		return false
	}
	if *cooldowns == nil {
		*cooldowns = make(map[string]time.Time)
	}
	if last, ok := (*cooldowns)[ability]; ok {
		if now.Sub(last) < cooldown {
			return false
		}
	}
	(*cooldowns)[ability] = now
	return true
}

// triggerMeleeAttack spawns a short-lived melee hitbox if the cooldown allows it.
func (w *World) triggerMeleeAttack(actorID string, now time.Time) bool {
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
		expiresAt: now.Add(meleeAttackDuration),
	}

	w.effects = append(w.effects, effect)

	area := Obstacle{X: rectX, Y: rectY, Width: rectW, Height: rectH}
	for _, obs := range w.obstacles {
		if obs.Type != obstacleTypeGoldOre {
			continue
		}
		if !obstaclesOverlap(area, obs, 0) {
			continue
		}
		if _, err := state.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1}); err != nil {
			log.Printf("failed to add mined gold for %s: %v", actorID, err)
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

	if len(hitPlayerIDs) > 0 {
		log.Printf("%s %s overlaps players %v", actorID, effectTypeAttack, hitPlayerIDs)
	}
	if len(hitNPCIDs) > 0 {
		log.Printf("%s %s overlaps NPCs %v", actorID, effectTypeAttack, hitNPCIDs)
	}

	return true
}

func (w *World) spawnProjectile(actorID, projectileType string, now time.Time) (*effectState, bool) {
	if w.projectileTemplates == nil {
		return nil, false
	}
	tpl, ok := w.projectileTemplates[projectileType]
	if !ok || tpl == nil {
		return nil, false
	}

	owner, cooldowns := w.abilityOwner(actorID)
	if owner == nil || cooldowns == nil {
		return nil, false
	}

	if !w.cooldownReady(cooldowns, tpl.Type, tpl.Cooldown, now) {
		return nil, false
	}

	facing := owner.Facing
	if facing == "" {
		facing = defaultFacing
	}

	dx, dy := facingToVector(facing)
	if dx == 0 && dy == 0 {
		dx, dy = 0, 1
	}

	width, height := spawnSizeFromShape(tpl)
	radius := tpl.SpawnRadius
	offset := tpl.SpawnOffset
	if offset == 0 {
		// default to placing the projectile just outside the owner's bounds
		offset = playerHalf + radius
	}

	centerX := owner.X + dx*offset
	centerY := owner.Y + dy*offset

	lifetime := effectLifetime(tpl)
	w.nextEffectID++
	effect := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", w.nextEffectID),
			Type:     tpl.Type,
			Owner:    actorID,
			Start:    now.UnixMilli(),
			Duration: lifetime.Milliseconds(),
			X:        centerX - width/2,
			Y:        centerY - height/2,
			Width:    width,
			Height:   height,
			Params: mergeParams(tpl.Params, map[string]float64{
				"radius": radius,
				"speed":  tpl.Speed,
				"range":  tpl.MaxDistance,
				"dx":     dx,
				"dy":     dy,
			}),
		},
		expiresAt: now.Add(lifetime),
		Projectile: &ProjectileState{
			Template:       tpl,
			VelocityUnitX:  dx,
			VelocityUnitY:  dy,
			RemainingRange: tpl.MaxDistance,
		},
	}

	w.effects = append(w.effects, effect)
	return effect, true
}

// triggerFireball launches a projectile effect when the player is ready.
func (w *World) triggerFireball(actorID string, now time.Time) bool {
	w.pruneEffects(now)
	_, spawned := w.spawnProjectile(actorID, effectTypeFireball, now)
	return spawned
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

	for _, eff := range w.effects {
		projectile := eff.Projectile
		if projectile == nil {
			continue
		}
		if !now.Before(eff.expiresAt) {
			if !projectile.Expired {
				w.maybeExplodeOnExpiry(eff, now)
				projectile.Expired = true
			}
			continue
		}
		w.advanceProjectile(eff, now, dt)
	}
}

func (w *World) advanceProjectile(eff *effectState, now time.Time, dt float64) {
	projectile := eff.Projectile
	if projectile == nil || projectile.Expired || dt <= 0 {
		return
	}

	tpl := projectile.Template
	if tpl == nil {
		projectile.Expired = true
		eff.expiresAt = now
		return
	}

	distance := tpl.Speed * dt
	if distance <= 0 {
		return
	}
	if distance > projectile.RemainingRange {
		distance = projectile.RemainingRange
	}

	eff.Effect.X += projectile.VelocityUnitX * distance
	eff.Effect.Y += projectile.VelocityUnitY * distance
	projectile.RemainingRange -= distance
	if eff.Params == nil {
		eff.Params = make(map[string]float64)
	}
	eff.Params["remainingRange"] = projectile.RemainingRange

	if projectile.RemainingRange <= 0 {
		projectile.Expired = true
		eff.expiresAt = now
		w.maybeExplodeOnExpiry(eff, now)
		return
	}

	if eff.Effect.X < 0 || eff.Effect.Y < 0 || eff.Effect.X+eff.Effect.Width > worldWidth || eff.Effect.Y+eff.Effect.Height > worldHeight {
		projectile.Expired = true
		eff.expiresAt = now
		w.maybeExplodeOnExpiry(eff, now)
		return
	}

	w.resolveProjectileImpacts(eff, now)
}

func (w *World) resolveProjectileImpacts(eff *effectState, now time.Time) {
	projectile := eff.Projectile
	if projectile == nil || projectile.Expired {
		return
	}
	tpl := projectile.Template
	if tpl == nil {
		return
	}

	area := effectAABB(eff)
	hitObstacle := w.projectileHitsObstacle(area)

	initialHits := projectile.HitCount
	hitPlayers := make([]string, 0)
	hitNPCs := make([]string, 0)

	allowOwnerHit := tpl.ImpactRules.AffectsOwner
	maxTargets := tpl.ImpactRules.MaxTargets

	for id, target := range w.players {
		if target == nil {
			continue
		}
		if !allowOwnerHit && id == eff.Owner {
			continue
		}
		if circleRectOverlap(target.X, target.Y, playerHalf, area) {
			if maxTargets > 0 && projectile.HitCount >= maxTargets {
				break
			}
			hitPlayers = append(hitPlayers, id)
			w.applyEffectHitPlayer(eff, target, now)
			projectile.HitCount++
			if maxTargets > 0 && projectile.HitCount >= maxTargets {
				break
			}
		}
	}

	if maxTargets == 0 || projectile.HitCount < maxTargets {
		for id, target := range w.npcs {
			if target == nil {
				continue
			}
			if !allowOwnerHit && id == eff.Owner {
				continue
			}
			if circleRectOverlap(target.X, target.Y, playerHalf, area) {
				if maxTargets > 0 && projectile.HitCount >= maxTargets {
					break
				}
				hitNPCs = append(hitNPCs, id)
				w.applyEffectHitNPC(eff, target, now)
				projectile.HitCount++
				if maxTargets > 0 && projectile.HitCount >= maxTargets {
					break
				}
			}
		}
	}

	hitsApplied := projectile.HitCount - initialHits
	if hitsApplied > 0 {
		if len(hitPlayers) > 0 {
			log.Printf("%s %s hit players %v", eff.Owner, eff.Type, hitPlayers)
		}
		if len(hitNPCs) > 0 {
			log.Printf("%s %s hit NPCs %v", eff.Owner, eff.Type, hitNPCs)
		}
	}

	exploded := false
	if hitsApplied > 0 && tpl.ImpactRules.ExplodeOnImpact != nil {
		w.spawnAreaEffectAt(eff, now, tpl.ImpactRules.ExplodeOnImpact)
		exploded = true
	}

	stopAfterTargets := tpl.ImpactRules.StopOnHit || (maxTargets > 0 && projectile.HitCount >= maxTargets)
	if hitsApplied > 0 && (stopAfterTargets || hitObstacle) {
		projectile.RemainingRange = 0
		projectile.Expired = true
		eff.expiresAt = now
		return
	}

	if hitObstacle {
		if tpl.ImpactRules.ExplodeOnImpact != nil && !exploded {
			w.spawnAreaEffectAt(eff, now, tpl.ImpactRules.ExplodeOnImpact)
		}
		projectile.RemainingRange = 0
		projectile.Expired = true
		eff.expiresAt = now
	}
}

func (w *World) projectileHitsObstacle(area Obstacle) bool {
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

func (w *World) spawnAreaEffectAt(eff *effectState, now time.Time, spec *ExplosionSpec) {
	if eff == nil || spec == nil {
		return
	}

	radius := spec.Radius
	size := radius * 2
	cx, cy := effectCenter(eff)

	duration := spec.Duration
	w.nextEffectID++
	areaEffect := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", w.nextEffectID),
			Type:     spec.EffectType,
			Owner:    eff.Owner,
			Start:    now.UnixMilli(),
			Duration: duration.Milliseconds(),
			X:        cx - radius,
			Y:        cy - radius,
			Width:    size,
			Height:   size,
			Params: mergeParams(spec.Params, map[string]float64{
				"radius":      radius,
				"duration_ms": float64(duration.Milliseconds()),
			}),
		},
		expiresAt: now.Add(duration),
	}

	w.effects = append(w.effects, areaEffect)
}

func (w *World) maybeExplodeOnExpiry(eff *effectState, now time.Time) {
	if eff == nil || eff.Projectile == nil {
		return
	}
	tpl := eff.Projectile.Template
	if tpl == nil || eff.Projectile.HitCount > 0 {
		return
	}
	if spec := tpl.ImpactRules.ExplodeOnExpiry; spec != nil {
		w.spawnAreaEffectAt(eff, now, spec)
	}
}

func effectAABB(eff *effectState) Obstacle {
	return Obstacle{X: eff.Effect.X, Y: eff.Effect.Y, Width: eff.Effect.Width, Height: eff.Effect.Height}
}

func effectCenter(eff *effectState) (float64, float64) {
	if eff == nil {
		return 0, 0
	}
	return eff.Effect.X + eff.Effect.Width/2, eff.Effect.Y + eff.Effect.Height/2
}

// pruneEffects drops expired effects from the in-memory list.
func (w *World) pruneEffects(now time.Time) {
	if len(w.effects) == 0 {
		return
	}
	filtered := w.effects[:0]
	for _, eff := range w.effects {
		if now.Before(eff.expiresAt) {
			filtered = append(filtered, eff)
		}
	}
	w.effects = filtered
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
	wasAlive := target.Health > 0
	w.applyEffectHitActor(eff, &target.actorState, now)
	if wasAlive && target.Health <= 0 {
		w.handleNPCDefeat(target)
	}
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

// applyEnvironmentalDamage processes hazard areas that deal damage over time.
func (w *World) applyEnvironmentalDamage(states []*actorState, dt float64) {
	if dt <= 0 || len(states) == 0 {
		return
	}
	damage := lavaDamagePerSecond * dt
	if damage <= 0 {
		return
	}
	for _, state := range states {
		for _, obs := range w.obstacles {
			if obs.Type != obstacleTypeLava {
				continue
			}
			if circleRectOverlap(state.X, state.Y, playerHalf, obs) {
				if state.applyHealthDelta(-damage) {
				}
				break
			}
		}
	}
}
