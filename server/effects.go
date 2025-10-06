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

type effectState struct {
	Effect
	expiresAt      time.Time
	velocityX      float64
	velocityY      float64
	remainingRange float64
	projectile     bool
}

type effectBehavior interface {
	OnHit(w *World, eff *effectState, target *actorState, now time.Time, tick uint64, output *StepOutput)
}

type effectBehaviorFunc func(w *World, eff *effectState, target *actorState, now time.Time, tick uint64, output *StepOutput)

func (f effectBehaviorFunc) OnHit(w *World, eff *effectState, target *actorState, now time.Time, tick uint64, output *StepOutput) {
	f(w, eff, target, now, tick, output)
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

func healthDeltaBehavior(param string, fallback float64) effectBehavior {
	return effectBehaviorFunc(func(w *World, eff *effectState, target *actorState, now time.Time, tick uint64, output *StepOutput) {
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
			output.Events = append(output.Events, Event{
				Tick:     tick,
				EntityID: target.ID,
				Type:     EventHealthChanged,
				Payload: map[string]any{
					"delta":  delta,
					"health": target.Health,
				},
			})
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

// triggerMeleeAttack spawns a short-lived melee hitbox if the cooldown allows it.
func (w *World) triggerMeleeAttack(actorID string, now time.Time, tick uint64, output *StepOutput) bool {
	state, cooldowns := w.abilityOwner(actorID)
	if state == nil || cooldowns == nil {
		return false
	}

	if *cooldowns == nil {
		*cooldowns = make(map[string]time.Time)
	}

	if last, ok := (*cooldowns)[effectTypeAttack]; ok {
		if now.Sub(last) < meleeAttackCooldown {
			return false
		}
	}

	(*cooldowns)[effectTypeAttack] = now

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
	output.Events = append(output.Events, Event{
		Tick:     tick,
		EntityID: effect.ID,
		Type:     EventEffectSpawned,
		Payload: map[string]any{
			"owner": actorID,
			"kind":  effectTypeAttack,
		},
	})

	area := Obstacle{X: rectX, Y: rectY, Width: rectW, Height: rectH}
	for _, obs := range w.obstacles {
		if obs.Type != obstacleTypeGoldOre {
			continue
		}
		if !obstaclesOverlap(area, obs, 0) {
			continue
		}
		if slot, err := state.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1}); err != nil {
			log.Printf("failed to add mined gold for %s: %v", actorID, err)
		} else {
			output.Events = append(output.Events, Event{
				Tick:     tick,
				EntityID: actorID,
				Type:     EventItemAdded,
				Payload: map[string]any{
					"item":     ItemTypeGold,
					"quantity": 1,
					"slot":     slot,
				},
			})
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
			w.applyEffectHitPlayer(effect, target, now, tick, output)
		}
	}

	hitNPCIDs := make([]string, 0)
	for id, target := range w.npcs {
		if id == actorID {
			continue
		}
		if circleRectOverlap(target.X, target.Y, playerHalf, area) {
			hitNPCIDs = append(hitNPCIDs, id)
			w.applyEffectHitNPC(effect, target, now, tick, output)
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

// triggerFireball launches a projectile effect when the player is ready.
func (w *World) triggerFireball(actorID string, now time.Time, tick uint64, output *StepOutput) bool {
	state, cooldowns := w.abilityOwner(actorID)
	if state == nil || cooldowns == nil {
		return false
	}

	if *cooldowns == nil {
		*cooldowns = make(map[string]time.Time)
	}

	if last, ok := (*cooldowns)[effectTypeFireball]; ok {
		if now.Sub(last) < fireballCooldown {
			return false
		}
	}

	(*cooldowns)[effectTypeFireball] = now

	facing := state.Facing
	if facing == "" {
		facing = defaultFacing
	}

	dirX, dirY := facingToVector(facing)
	if dirX == 0 && dirY == 0 {
		dirX, dirY = 0, 1
	}

	radius := fireballSize / 2
	spawnOffset := playerHalf + fireballSpawnGap + radius
	centerX := state.X + dirX*spawnOffset
	centerY := state.Y + dirY*spawnOffset

	w.pruneEffects(now)
	w.nextEffectID++
	effect := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", w.nextEffectID),
			Type:     effectTypeFireball,
			Owner:    actorID,
			Start:    now.UnixMilli(),
			Duration: fireballLifetime.Milliseconds(),
			X:        centerX - radius,
			Y:        centerY - radius,
			Width:    fireballSize,
			Height:   fireballSize,
			Params: map[string]float64{
				"radius":      radius,
				"speed":       fireballSpeed,
				"range":       fireballRange,
				"dx":          dirX,
				"dy":          dirY,
				"healthDelta": -fireballDamage,
			},
		},
		expiresAt:      now.Add(fireballLifetime),
		velocityX:      dirX,
		velocityY:      dirY,
		remainingRange: fireballRange,
		projectile:     true,
	}

	w.effects = append(w.effects, effect)
	output.Events = append(output.Events, Event{
		Tick:     tick,
		EntityID: effect.ID,
		Type:     EventEffectSpawned,
		Payload: map[string]any{
			"owner": actorID,
			"kind":  effectTypeFireball,
		},
	})
	return true
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
func (w *World) advanceEffects(now time.Time, dt float64, tick uint64, output *StepOutput) {
	if len(w.effects) == 0 {
		return
	}

	for _, eff := range w.effects {
		if !eff.projectile || !now.Before(eff.expiresAt) {
			continue
		}

		distance := fireballSpeed * dt
		if distance <= 0 {
			continue
		}
		if distance > eff.remainingRange {
			distance = eff.remainingRange
		}

		eff.Effect.X += eff.velocityX * distance
		eff.Effect.Y += eff.velocityY * distance
		eff.remainingRange -= distance
		if eff.Params == nil {
			eff.Params = make(map[string]float64)
		}
		eff.Params["remainingRange"] = eff.remainingRange

		if eff.remainingRange <= 0 {
			eff.expiresAt = now
			continue
		}

		if eff.Effect.X < 0 || eff.Effect.Y < 0 || eff.Effect.X+eff.Effect.Width > worldWidth || eff.Effect.Y+eff.Effect.Height > worldHeight {
			eff.expiresAt = now
			continue
		}

		area := Obstacle{X: eff.Effect.X, Y: eff.Effect.Y, Width: eff.Effect.Width, Height: eff.Effect.Height}

		collided := false
		for _, obs := range w.obstacles {
			if obs.Type == obstacleTypeLava {
				continue
			}
			if obstaclesOverlap(area, obs, 0) {
				collided = true
				break
			}
		}

		if collided {
			eff.expiresAt = now
			continue
		}

		hitPlayers := make([]string, 0)
		for id, target := range w.players {
			if id == eff.Owner {
				continue
			}
			if circleRectOverlap(target.X, target.Y, playerHalf, area) {
				collided = true
				hitPlayers = append(hitPlayers, id)
				w.applyEffectHitPlayer(eff, target, now, tick, output)
			}
		}

		hitNPCs := make([]string, 0)
		for id, target := range w.npcs {
			if id == eff.Owner {
				continue
			}
			if circleRectOverlap(target.X, target.Y, playerHalf, area) {
				collided = true
				hitNPCs = append(hitNPCs, id)
				w.applyEffectHitNPC(eff, target, now, tick, output)
			}
		}

		if collided {
			eff.expiresAt = now
			if len(hitPlayers) > 0 {
				log.Printf("%s %s hit players %v", eff.Owner, eff.Type, hitPlayers)
			}
			if len(hitNPCs) > 0 {
				log.Printf("%s %s hit NPCs %v", eff.Owner, eff.Type, hitNPCs)
			}
		}
	}
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

func (w *World) applyEffectHitPlayer(eff *effectState, target *playerState, now time.Time, tick uint64, output *StepOutput) {
	if target == nil {
		return
	}
	w.applyEffectHitActor(eff, &target.actorState, now, tick, output)
}

func (w *World) applyEffectHitNPC(eff *effectState, target *npcState, now time.Time, tick uint64, output *StepOutput) {
	if target == nil {
		return
	}
	wasAlive := target.Health > 0
	w.applyEffectHitActor(eff, &target.actorState, now, tick, output)
	if wasAlive && target.Health <= 0 {
		w.handleNPCDefeat(target, tick, output)
	}
}

func (w *World) applyEffectHitActor(eff *effectState, target *actorState, now time.Time, tick uint64, output *StepOutput) {
	if eff == nil || target == nil {
		return
	}
	behavior, ok := w.effectBehaviors[eff.Type]
	if !ok || behavior == nil {
		return
	}
	behavior.OnHit(w, eff, target, now, tick, output)
}

// applyEnvironmentalDamage processes hazard areas that deal damage over time.
func (w *World) applyEnvironmentalDamage(states []*actorState, dt float64, tick uint64, output *StepOutput) {
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
					output.Events = append(output.Events, Event{
						Tick:     tick,
						EntityID: state.ID,
						Type:     EventHealthChanged,
						Payload: map[string]any{
							"delta":  -damage,
							"health": state.Health,
						},
					})
				}
				break
			}
		}
	}
}
