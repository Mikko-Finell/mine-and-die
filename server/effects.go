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
)

var fireballLifetime = time.Duration(fireballRange / fireballSpeed * float64(time.Second))

// HandleAction routes an action string to the appropriate ability helper.
func (h *Hub) HandleAction(playerID, action string) bool {
	switch action {
	case effectTypeAttack:
		h.triggerMeleeAttack(playerID)
		return true
	case effectTypeFireball:
		h.triggerFireball(playerID)
		return true
	default:
		return false
	}
}

// triggerMeleeAttack spawns a short-lived melee hitbox if the cooldown allows it.
func (h *Hub) triggerMeleeAttack(playerID string) bool {
	now := time.Now()

	h.mu.Lock()

	state, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return false
	}

	if state.cooldowns == nil {
		state.cooldowns = make(map[string]time.Time)
	}

	if last, ok := state.cooldowns[effectTypeAttack]; ok {
		if now.Sub(last) < meleeAttackCooldown {
			h.mu.Unlock()
			return false
		}
	}

	state.cooldowns[effectTypeAttack] = now

	facing := state.Facing
	if facing == "" {
		facing = defaultFacing
	}

	rectX, rectY, rectW, rectH := meleeAttackRectangle(state.X, state.Y, facing)

	effect := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", h.nextEffect.Add(1)),
			Type:     effectTypeAttack,
			Owner:    playerID,
			Start:    now.UnixMilli(),
			Duration: meleeAttackDuration.Milliseconds(),
			X:        rectX,
			Y:        rectY,
			Width:    rectW,
			Height:   rectH,
			Params: map[string]float64{
				"damage": meleeAttackDamage,
				"reach":  meleeAttackReach,
				"width":  meleeAttackWidth,
			},
		},
		expiresAt: now.Add(meleeAttackDuration),
	}

	h.pruneEffectsLocked(now)
	h.effects = append(h.effects, effect)

	area := Obstacle{X: rectX, Y: rectY, Width: rectW, Height: rectH}
	hitIDs := make([]string, 0)
	for id, target := range h.players {
		if id == playerID {
			continue
		}
		if circleRectOverlap(target.X, target.Y, playerHalf, area) {
			hitIDs = append(hitIDs, id)
		}
	}

	h.mu.Unlock()

	if len(hitIDs) > 0 {
		log.Printf("%s %s overlaps players %v", playerID, effectTypeAttack, hitIDs)
	}

	return true
}

// triggerFireball launches a projectile effect when the player is ready.
func (h *Hub) triggerFireball(playerID string) bool {
	now := time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return false
	}

	if state.cooldowns == nil {
		state.cooldowns = make(map[string]time.Time)
	}

	if last, ok := state.cooldowns[effectTypeFireball]; ok {
		if now.Sub(last) < fireballCooldown {
			return false
		}
	}

	state.cooldowns[effectTypeFireball] = now

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

	effect := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", h.nextEffect.Add(1)),
			Type:     effectTypeFireball,
			Owner:    playerID,
			Start:    now.UnixMilli(),
			Duration: fireballLifetime.Milliseconds(),
			X:        centerX - radius,
			Y:        centerY - radius,
			Width:    fireballSize,
			Height:   fireballSize,
			Params: map[string]float64{
				"radius": radius,
				"speed":  fireballSpeed,
				"range":  fireballRange,
				"dx":     dirX,
				"dy":     dirY,
			},
		},
		expiresAt:      now.Add(fireballLifetime),
		velocityX:      dirX,
		velocityY:      dirY,
		remainingRange: fireballRange,
		projectile:     true,
	}

	h.pruneEffectsLocked(now)
	h.effects = append(h.effects, effect)
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

// advanceEffectsLocked moves active projectiles and expires ones that collide or run out of range.
func (h *Hub) advanceEffectsLocked(now time.Time, dt float64) {
	if len(h.effects) == 0 {
		return
	}

	for _, eff := range h.effects {
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
		for _, obs := range h.obstacles {
			if obstaclesOverlap(area, obs, 0) {
				collided = true
				break
			}
		}

		if collided {
			eff.expiresAt = now
			continue
		}

		for id, target := range h.players {
			if id == eff.Owner {
				continue
			}
			if circleRectOverlap(target.X, target.Y, playerHalf, area) {
				collided = true
				break
			}
		}

		if collided {
			eff.expiresAt = now
		}
	}
}

// pruneEffectsLocked drops expired effects from the in-memory list.
func (h *Hub) pruneEffectsLocked(now time.Time) {
	if len(h.effects) == 0 {
		return
	}
	filtered := h.effects[:0]
	for _, eff := range h.effects {
		if now.Before(eff.expiresAt) {
			filtered = append(filtered, eff)
		}
	}
	h.effects = filtered
}
