package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait             = 10 * time.Second
	tickRate              = 15    // ticks per second (10–20 Hz)
	moveSpeed             = 160.0 // pixels per second
	worldWidth            = 800.0
	worldHeight           = 600.0
	playerHalf            = 14.0
	heartbeatInterval     = 2 * time.Second
	disconnectAfter       = 3 * heartbeatInterval
	obstacleCount         = 6
	obstacleMinWidth      = 60.0
	obstacleMaxWidth      = 140.0
	obstacleMinHeight     = 60.0
	obstacleMaxHeight     = 140.0
	obstacleSpawnMargin   = 100.0
	playerSpawnSafeRadius = 120.0
	goldOreCount          = 4
	goldOreMinSize        = 56.0
	goldOreMaxSize        = 96.0
)

type Player struct {
	ID     string          `json:"id"`
	X      float64         `json:"x"`
	Y      float64         `json:"y"`
	Facing FacingDirection `json:"facing"`
}

type FacingDirection string

const (
	FacingUp    FacingDirection = "up"
	FacingDown  FacingDirection = "down"
	FacingLeft  FacingDirection = "left"
	FacingRight FacingDirection = "right"

	defaultFacing FacingDirection = FacingDown
)

// parseFacing validates a facing string received from the client.
func parseFacing(value string) (FacingDirection, bool) {
	switch FacingDirection(value) {
	case FacingUp, FacingDown, FacingLeft, FacingRight:
		return FacingDirection(value), true
	default:
		return "", false
	}
}

// deriveFacing picks the facing direction that best matches the movement
// vector, falling back to the last known facing when idle.
func deriveFacing(dx, dy float64, fallback FacingDirection) FacingDirection {
	if fallback == "" {
		fallback = defaultFacing
	}

	const epsilon = 1e-6

	if math.Abs(dx) < epsilon {
		dx = 0
	}
	if math.Abs(dy) < epsilon {
		dy = 0
	}

	if dx == 0 && dy == 0 {
		return fallback
	}

	absX := math.Abs(dx)
	absY := math.Abs(dy)

	if absY >= absX && dy != 0 {
		if dy > 0 {
			return FacingDown
		}
		return FacingUp
	}

	if dx > 0 {
		return FacingRight
	}
	return FacingLeft
}

// facingToVector returns a unit vector for the given facing.
func facingToVector(facing FacingDirection) (float64, float64) {
	switch facing {
	case FacingUp:
		return 0, -1
	case FacingDown:
		return 0, 1
	case FacingLeft:
		return -1, 0
	case FacingRight:
		return 1, 0
	default:
		return 0, 1
	}
}

type Obstacle struct {
	ID     string  `json:"id"`
	Type   string  `json:"type,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type joinResponse struct {
	ID        string     `json:"id"`
	Players   []Player   `json:"players"`
	Obstacles []Obstacle `json:"obstacles"`
	Effects   []Effect   `json:"effects"`
}

type stateMessage struct {
	Type       string     `json:"type"`
	Players    []Player   `json:"players"`
	Obstacles  []Obstacle `json:"obstacles"`
	Effects    []Effect   `json:"effects"`
	ServerTime int64      `json:"serverTime"`
}

type clientMessage struct {
	Type   string  `json:"type"`
	DX     float64 `json:"dx"`
	DY     float64 `json:"dy"`
	Facing string  `json:"facing"`
	SentAt int64   `json:"sentAt"`
	Action string  `json:"action"`
}

// Effect represents a time-bound gameplay artifact (an attack swing, a buff,
// an environmental hazard, etc.). It captures the minimum data the simulation
// needs to reason about the effect while staying extendable via Params.
// Effect describes a time-limited gameplay artifact (attack swing, projectile,
// etc.) that clients can render.
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

// effectState stores runtime bookkeeping for an effect while keeping the
// serialized payload ready to share.
type effectState struct {
	Effect
	expiresAt      time.Time
	velocityX      float64
	velocityY      float64
	remainingRange float64
	projectile     bool
}

type heartbeatMessage struct {
	Type       string `json:"type"`
	ServerTime int64  `json:"serverTime"`
	ClientTime int64  `json:"clientTime"`
	RTTMillis  int64  `json:"rtt"`
}

type subscriber struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type playerState struct {
	Player
	intentX       float64
	intentY       float64
	lastInput     time.Time
	lastHeartbeat time.Time
	lastRTT       time.Duration
	cooldowns     map[string]time.Time
}

type diagnosticsPlayer struct {
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
}

// Hub owns all live players, subscribers, obstacles, and active effects.
type Hub struct {
	mu          sync.Mutex
	players     map[string]*playerState
	subscribers map[string]*subscriber
	nextID      atomic.Uint64
	obstacles   []Obstacle
	effects     []*effectState
	nextEffect  atomic.Uint64
}

// newHub creates a hub with empty maps and a freshly generated obstacle set.
func newHub() *Hub {
	hub := &Hub{
		players:     make(map[string]*playerState),
		subscribers: make(map[string]*subscriber),
		effects:     make([]*effectState, 0),
	}
	hub.obstacles = hub.generateObstacles(obstacleCount)
	return hub
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

// generateObstacles scatters blocking rectangles and ore deposits around the map.
func (h *Hub) generateObstacles(count int) []Obstacle {
	if count <= 0 {
		return h.generateGoldOreNodes(goldOreCount, nil, rand.New(rand.NewSource(time.Now().UnixNano())))
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	obstacles := make([]Obstacle, 0, count)
	attempts := 0
	maxAttempts := count * 20

	for len(obstacles) < count && attempts < maxAttempts {
		attempts++

		width := obstacleMinWidth + rng.Float64()*(obstacleMaxWidth-obstacleMinWidth)
		height := obstacleMinHeight + rng.Float64()*(obstacleMaxHeight-obstacleMinHeight)

		maxX := worldWidth - obstacleSpawnMargin - width
		maxY := worldHeight - obstacleSpawnMargin - height
		if maxX <= obstacleSpawnMargin || maxY <= obstacleSpawnMargin {
			break
		}

		x := obstacleSpawnMargin + rng.Float64()*(maxX-obstacleSpawnMargin)
		y := obstacleSpawnMargin + rng.Float64()*(maxY-obstacleSpawnMargin)

		candidate := Obstacle{
			ID:     fmt.Sprintf("obstacle-%d", len(obstacles)+1),
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}

		if circleRectOverlap(80, 80, playerSpawnSafeRadius, candidate) {
			continue
		}

		overlapsExisting := false
		for _, obs := range obstacles {
			if obstaclesOverlap(candidate, obs, playerHalf) {
				overlapsExisting = true
				break
			}
		}

		if overlapsExisting {
			continue
		}

		obstacles = append(obstacles, candidate)
	}

	goldOre := h.generateGoldOreNodes(goldOreCount, obstacles, rng)
	return append(obstacles, goldOre...)
}

// generateGoldOreNodes places ore obstacles while avoiding overlaps.
func (h *Hub) generateGoldOreNodes(count int, existing []Obstacle, rng *rand.Rand) []Obstacle {
	if count <= 0 || rng == nil {
		return nil
	}

	ores := make([]Obstacle, 0, count)
	attempts := 0
	maxAttempts := count * 30

	for len(ores) < count && attempts < maxAttempts {
		attempts++

		width := goldOreMinSize + rng.Float64()*(goldOreMaxSize-goldOreMinSize)
		height := goldOreMinSize + rng.Float64()*(goldOreMaxSize-goldOreMinSize)

		maxX := worldWidth - obstacleSpawnMargin - width
		maxY := worldHeight - obstacleSpawnMargin - height
		if maxX <= obstacleSpawnMargin || maxY <= obstacleSpawnMargin {
			break
		}

		x := obstacleSpawnMargin + rng.Float64()*(maxX-obstacleSpawnMargin)
		y := obstacleSpawnMargin + rng.Float64()*(maxY-obstacleSpawnMargin)

		candidate := Obstacle{
			ID:     fmt.Sprintf("gold-ore-%d", len(ores)+1),
			Type:   "gold-ore",
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}

		if circleRectOverlap(80, 80, playerSpawnSafeRadius, candidate) {
			continue
		}

		overlaps := false

		for _, obs := range existing {
			if obstaclesOverlap(candidate, obs, playerHalf) {
				overlaps = true
				break
			}
		}

		if overlaps {
			continue
		}

		for _, ore := range ores {
			if obstaclesOverlap(candidate, ore, playerHalf) {
				overlaps = true
				break
			}
		}

		if overlaps {
			continue
		}

		ores = append(ores, candidate)
	}

	return ores
}

// circleRectOverlap reports whether a circle intersects an obstacle rectangle.
func circleRectOverlap(cx, cy, radius float64, obs Obstacle) bool {
	closestX := clamp(cx, obs.X, obs.X+obs.Width)
	closestY := clamp(cy, obs.Y, obs.Y+obs.Height)
	dx := cx - closestX
	dy := cy - closestY
	return dx*dx+dy*dy < radius*radius
}

// obstaclesOverlap checks for AABB overlap with optional padding.
func obstaclesOverlap(a, b Obstacle, padding float64) bool {
	return a.X-padding < b.X+b.Width+padding &&
		a.X+a.Width+padding > b.X-padding &&
		a.Y-padding < b.Y+b.Height+padding &&
		a.Y+a.Height+padding > b.Y-padding
}

// clamp limits value to the range [min, max].
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// movePlayerWithObstacles advances a player while clamping speed, bounds, and walls.
func movePlayerWithObstacles(state *playerState, dt float64, obstacles []Obstacle) {
	dx := state.intentX
	dy := state.intentY
	length := math.Hypot(dx, dy)
	if length != 0 {
		dx /= length
		dy /= length
	}

	deltaX := dx * moveSpeed * dt
	deltaY := dy * moveSpeed * dt

	newX := clamp(state.X+deltaX, playerHalf, worldWidth-playerHalf)
	if deltaX != 0 {
		newX = resolveAxisMoveX(state.X, state.Y, newX, deltaX, obstacles)
	}

	newY := clamp(state.Y+deltaY, playerHalf, worldHeight-playerHalf)
	if deltaY != 0 {
		newY = resolveAxisMoveY(newX, state.Y, newY, deltaY, obstacles)
	}

	state.X = newX
	state.Y = newY

	resolveObstaclePenetration(state, obstacles)
}

// resolveAxisMoveX applies horizontal movement while stopping at obstacle edges.
func resolveAxisMoveX(oldX, oldY, proposedX, deltaX float64, obstacles []Obstacle) float64 {
	newX := proposedX
	for _, obs := range obstacles {
		minY := obs.Y - playerHalf
		maxY := obs.Y + obs.Height + playerHalf
		if oldY < minY || oldY > maxY {
			continue
		}

		if deltaX > 0 {
			boundary := obs.X - playerHalf
			if oldX <= boundary && newX > boundary {
				newX = boundary
			}
		} else if deltaX < 0 {
			boundary := obs.X + obs.Width + playerHalf
			if oldX >= boundary && newX < boundary {
				newX = boundary
			}
		}
	}
	return clamp(newX, playerHalf, worldWidth-playerHalf)
}

// resolveAxisMoveY applies vertical movement while stopping at obstacle edges.
func resolveAxisMoveY(oldX, oldY, proposedY, deltaY float64, obstacles []Obstacle) float64 {
	newY := proposedY
	for _, obs := range obstacles {
		minX := obs.X - playerHalf
		maxX := obs.X + obs.Width + playerHalf
		if oldX < minX || oldX > maxX {
			continue
		}

		if deltaY > 0 {
			boundary := obs.Y - playerHalf
			if oldY <= boundary && newY > boundary {
				newY = boundary
			}
		} else if deltaY < 0 {
			boundary := obs.Y + obs.Height + playerHalf
			if oldY >= boundary && newY < boundary {
				newY = boundary
			}
		}
	}
	return clamp(newY, playerHalf, worldHeight-playerHalf)
}

// resolveObstaclePenetration nudges a player out of overlapping obstacles.
func resolveObstaclePenetration(state *playerState, obstacles []Obstacle) {
	for _, obs := range obstacles {
		if !circleRectOverlap(state.X, state.Y, playerHalf, obs) {
			continue
		}

		closestX := clamp(state.X, obs.X, obs.X+obs.Width)
		closestY := clamp(state.Y, obs.Y, obs.Y+obs.Height)
		dx := state.X - closestX
		dy := state.Y - closestY
		distSq := dx*dx + dy*dy

		if distSq == 0 {
			left := math.Abs(state.X - obs.X)
			right := math.Abs((obs.X + obs.Width) - state.X)
			top := math.Abs(state.Y - obs.Y)
			bottom := math.Abs((obs.Y + obs.Height) - state.Y)

			minDist := left
			direction := 0
			if right < minDist {
				minDist = right
				direction = 1
			}
			if top < minDist {
				minDist = top
				direction = 2
			}
			if bottom < minDist {
				direction = 3
			}

			switch direction {
			case 0:
				state.X = obs.X - playerHalf
			case 1:
				state.X = obs.X + obs.Width + playerHalf
			case 2:
				state.Y = obs.Y - playerHalf
			case 3:
				state.Y = obs.Y + obs.Height + playerHalf
			}
		} else {
			dist := math.Sqrt(distSq)
			if dist < playerHalf {
				overlap := playerHalf - dist
				nx := dx / dist
				ny := dy / dist
				state.X += nx * overlap
				state.Y += ny * overlap
			}
		}

		state.X = clamp(state.X, playerHalf, worldWidth-playerHalf)
		state.Y = clamp(state.Y, playerHalf, worldHeight-playerHalf)
	}
}

// resolvePlayerCollisions separates overlapping players while respecting walls.
func resolvePlayerCollisions(players []*playerState, obstacles []Obstacle) {
	if len(players) < 2 {
		return
	}

	const iterations = 4
	for iter := 0; iter < iterations; iter++ {
		adjusted := false
		for i := 0; i < len(players); i++ {
			for j := i + 1; j < len(players); j++ {
				p1 := players[i]
				p2 := players[j]
				dx := p2.X - p1.X
				dy := p2.Y - p1.Y
				distSq := dx*dx + dy*dy
				minDist := playerHalf * 2

				var dist float64
				if distSq == 0 {
					dx = 1
					dy = 0
					dist = 1
				} else {
					dist = math.Sqrt(distSq)
				}

				if dist >= minDist {
					continue
				}

				overlap := (minDist - dist) / 2
				nx := dx / dist
				ny := dy / dist

				p1.X -= nx * overlap
				p1.Y -= ny * overlap
				p2.X += nx * overlap
				p2.Y += ny * overlap

				p1.X = clamp(p1.X, playerHalf, worldWidth-playerHalf)
				p1.Y = clamp(p1.Y, playerHalf, worldHeight-playerHalf)
				p2.X = clamp(p2.X, playerHalf, worldWidth-playerHalf)
				p2.Y = clamp(p2.Y, playerHalf, worldHeight-playerHalf)

				resolveObstaclePenetration(p1, obstacles)
				resolveObstaclePenetration(p2, obstacles)

				adjusted = true
			}
		}

		if !adjusted {
			break
		}
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

// snapshotLocked copies players/effects for broadcasting while holding the mutex.
func (h *Hub) snapshotLocked(now time.Time) ([]Player, []Effect) {
	players := make([]Player, 0, len(h.players))
	for _, player := range h.players {
		if player.Facing == "" {
			player.Facing = defaultFacing
		}
		players = append(players, player.Player)
	}
	effects := make([]Effect, 0, len(h.effects))
	for _, eff := range h.effects {
		if now.Before(eff.expiresAt) {
			effects = append(effects, eff.Effect)
		}
	}
	return players, effects
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

// broadcastState sends the latest world snapshot to every subscriber.
func (h *Hub) broadcastState(players []Player, effects []Effect) {
	if players == nil || effects == nil {
		h.mu.Lock()
		now := time.Now()
		if players == nil || effects == nil {
			players, effects = h.snapshotLocked(now)
		}
		h.mu.Unlock()
	}

	msg := stateMessage{
		Type:       "state",
		Players:    players,
		Obstacles:  h.obstacles,
		Effects:    effects,
		ServerTime: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal state message: %v", err)
		return
	}

	h.mu.Lock()
	subs := make(map[string]*subscriber, len(h.subscribers))
	for id, sub := range h.subscribers {
		subs[id] = sub
	}
	h.mu.Unlock()

	for id, sub := range subs {
		sub.mu.Lock()
		sub.conn.SetWriteDeadline(time.Now().Add(writeWait))
		err := sub.conn.WriteMessage(websocket.TextMessage, data)
		sub.mu.Unlock()
		if err != nil {
			log.Printf("failed to send update to %s: %v", id, err)
			players, effects := h.Disconnect(id)
			if players != nil {
				go h.broadcastState(players, effects)
			}
		}
	}
}

// Join registers a new player and returns the latest snapshot.
func (h *Hub) Join() joinResponse {
	id := h.nextID.Add(1)
	playerID := fmt.Sprintf("player-%d", id)
	now := time.Now()
	player := &playerState{
		Player:        Player{ID: playerID, X: 80, Y: 80, Facing: defaultFacing},
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
	}

	h.mu.Lock()
	h.pruneEffectsLocked(now)
	h.players[playerID] = player
	players, effects := h.snapshotLocked(now)
	h.mu.Unlock()

	go h.broadcastState(players, effects)

	return joinResponse{ID: playerID, Players: players, Obstacles: h.obstacles, Effects: effects}
}

// Subscribe associates a WebSocket connection with an existing player.
func (h *Hub) Subscribe(playerID string, conn *websocket.Conn) (*subscriber, []Player, []Effect, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return nil, nil, nil, false
	}

	state.lastHeartbeat = time.Now()

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn}
	h.subscribers[playerID] = sub
	now := time.Now()
	h.pruneEffectsLocked(now)
	players, effects := h.snapshotLocked(now)
	return sub, players, effects, true
}

// Disconnect removes a player and closes any active subscriber connection.
func (h *Hub) Disconnect(playerID string) ([]Player, []Effect) {
	h.mu.Lock()
	sub, subOK := h.subscribers[playerID]
	if subOK {
		delete(h.subscribers, playerID)
	}

	_, playerOK := h.players[playerID]
	if playerOK {
		delete(h.players, playerID)
	}

	var players []Player
	var effects []Effect
	if playerOK {
		now := time.Now()
		h.pruneEffectsLocked(now)
		players, effects = h.snapshotLocked(now)
	}
	h.mu.Unlock()

	if subOK {
		sub.conn.Close()
	}

	if !playerOK {
		return nil, nil
	}

	return players, effects
}

// UpdateIntent stores the latest movement vector and facing for a player.
func (h *Hub) UpdateIntent(playerID string, dx, dy float64, facing string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return false
	}

	if state.Facing == "" {
		state.Facing = defaultFacing
	}

	length := math.Hypot(dx, dy)
	if length > 1 {
		dx /= length
		dy /= length
	}

	state.intentX = dx
	state.intentY = dy

	state.Facing = deriveFacing(dx, dy, state.Facing)
	if dx == 0 && dy == 0 {
		if face, ok := parseFacing(facing); ok {
			state.Facing = face
		}
	}

	state.lastInput = time.Now()
	return true
}

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

// UpdateHeartbeat records the most recent heartbeat time and RTT for a player.
func (h *Hub) UpdateHeartbeat(playerID string, receivedAt time.Time, clientSent int64) (time.Duration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return 0, false
	}

	state.lastHeartbeat = receivedAt

	var rtt time.Duration
	if clientSent > 0 {
		clientTime := time.UnixMilli(clientSent)
		if clientTime.Before(receivedAt.Add(5 * time.Second)) {
			rtt = receivedAt.Sub(clientTime)
			if rtt < 0 {
				rtt = 0
			}
			state.lastRTT = rtt
		}
	}

	return state.lastRTT, true
}

// advance runs a single simulation step and returns updated snapshots plus stale subscribers.
func (h *Hub) advance(now time.Time, dt float64) ([]Player, []Effect, []*subscriber) {
	h.mu.Lock()

	toClose := make([]*subscriber, 0)
	activeStates := make([]*playerState, 0, len(h.players))
	for id, state := range h.players {
		if now.Sub(state.lastHeartbeat) > disconnectAfter {
			if sub, ok := h.subscribers[id]; ok {
				toClose = append(toClose, sub)
				delete(h.subscribers, id)
			}
			delete(h.players, id)
			log.Printf("disconnecting %s due to heartbeat timeout", id)
			continue
		}

		activeStates = append(activeStates, state)

		if state.intentX != 0 || state.intentY != 0 {
			movePlayerWithObstacles(state, dt, h.obstacles)
		}
	}

	resolvePlayerCollisions(activeStates, h.obstacles)

	h.advanceEffectsLocked(now, dt)
	h.pruneEffectsLocked(now)
	players, effects := h.snapshotLocked(now)
	h.mu.Unlock()

	return players, effects, toClose
}

// RunSimulation drives the fixed-rate tick loop until the stop channel closes.
func (h *Hub) RunSimulation(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			if dt <= 0 {
				dt = 1.0 / float64(tickRate)
			}
			last = now

			players, effects, toClose := h.advance(now, dt)
			for _, sub := range toClose {
				sub.conn.Close()
			}
			h.broadcastState(players, effects)
		}
	}
}

// DiagnosticsSnapshot exposes heartbeat data for the diagnostics endpoint.
func (h *Hub) DiagnosticsSnapshot() []diagnosticsPlayer {
	h.mu.Lock()
	defer h.mu.Unlock()

	players := make([]diagnosticsPlayer, 0, len(h.players))
	for _, state := range h.players {
		players = append(players, diagnosticsPlayer{
			ID:            state.ID,
			LastHeartbeat: state.lastHeartbeat.UnixMilli(),
			RTTMillis:     state.lastRTT.Milliseconds(),
		})
	}
	return players
}

// main wires up HTTP handlers, starts the simulation, and serves the client.
func main() {
	hub := newHub()
	stop := make(chan struct{})
	go hub.RunSimulation(stop)
	defer close(stop)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		payload := struct {
			Status     string              `json:"status"`
			ServerTime int64               `json:"serverTime"`
			Players    []diagnosticsPlayer `json:"players"`
			TickRate   int                 `json:"tickRate"`
			Heartbeat  int64               `json:"heartbeatMillis"`
		}{
			Status:     "ok",
			ServerTime: time.Now().UnixMilli(),
			Players:    hub.DiagnosticsSnapshot(),
			TickRate:   tickRate,
			Heartbeat:  heartbeatInterval.Milliseconds(),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "failed to encode", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		join := hub.Join()
		data, err := json.Marshal(join)
		if err != nil {
			http.Error(w, "failed to encode", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade failed for %s: %v", playerID, err)
			return
		}

		sub, snapshotPlayers, snapshotEffects, ok := hub.Subscribe(playerID, conn)
		if !ok {
			message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
			conn.WriteMessage(websocket.CloseMessage, message)
			conn.Close()
			return
		}

		initial := stateMessage{
			Type:       "state",
			Players:    snapshotPlayers,
			Obstacles:  hub.obstacles,
			Effects:    snapshotEffects,
			ServerTime: time.Now().UnixMilli(),
		}
		data, err := json.Marshal(initial)
		if err != nil {
			log.Printf("failed to marshal initial state for %s: %v", playerID, err)
			players, effects := hub.Disconnect(playerID)
			if players != nil {
				go hub.broadcastState(players, effects)
			}
			return
		}

		sub.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			sub.mu.Unlock()
			players, effects := hub.Disconnect(playerID)
			if players != nil {
				go hub.broadcastState(players, effects)
			}
			return
		}
		sub.mu.Unlock()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				players, effects := hub.Disconnect(playerID)
				if players != nil {
					go hub.broadcastState(players, effects)
				}
				return
			}

			var msg clientMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				log.Printf("discarding malformed message from %s: %v", playerID, err)
				continue
			}

			switch msg.Type {
			case "input":
				if !hub.UpdateIntent(playerID, msg.DX, msg.DY, msg.Facing) {
					log.Printf("input ignored for unknown player %s", playerID)
				}
			case "action":
				if msg.Action == "" {
					continue
				}
				if !hub.HandleAction(playerID, msg.Action) {
					log.Printf("unknown action %q from %s", msg.Action, playerID)
				}
			case "heartbeat":
				now := time.Now()
				rtt, ok := hub.UpdateHeartbeat(playerID, now, msg.SentAt)
				if !ok {
					continue
				}

				ack := heartbeatMessage{
					Type:       "heartbeat",
					ServerTime: now.UnixMilli(),
					ClientTime: msg.SentAt,
					RTTMillis:  rtt.Milliseconds(),
				}

				data, err := json.Marshal(ack)
				if err != nil {
					log.Printf("failed to marshal heartbeat ack for %s: %v", playerID, err)
					continue
				}

				sub.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					sub.mu.Unlock()
					players, effects := hub.Disconnect(playerID)
					if players != nil {
						go hub.broadcastState(players, effects)
					}
					return
				}
				sub.mu.Unlock()
			default:
				log.Printf("unknown message type %q from %s", msg.Type, playerID)
			}
		}
	})

	clientDir := filepath.Clean(filepath.Join("..", "client"))
	fs := http.FileServer(http.Dir(clientDir))
	http.Handle("/", fs)

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
