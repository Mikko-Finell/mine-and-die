package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func findPlayer(players []Player, id string) *Player {
	for i := range players {
		if players[i].ID == id {
			return &players[i]
		}
	}
	return nil
}

func TestHubJoinCreatesPlayer(t *testing.T) {
	hub := newHub()

	first := hub.Join()
	if first.ID == "" {
		t.Fatalf("expected join response to include an id")
	}
	if len(first.Players) != 1 {
		t.Fatalf("expected snapshot to contain 1 player, got %d", len(first.Players))
	}
	if p := findPlayer(first.Players, first.ID); p == nil {
		t.Fatalf("snapshot missing newly joined player %q", first.ID)
	} else if p.Facing == "" {
		t.Fatalf("expected joined player to include facing direction")
	}

	second := hub.Join()
	if second.ID == first.ID {
		t.Fatalf("expected unique ids, both were %q", second.ID)
	}
	if len(second.Players) != 2 {
		t.Fatalf("expected snapshot to contain 2 players, got %d", len(second.Players))
	}
	if _, ok := hub.players[first.ID]; !ok {
		t.Fatalf("hub players map missing first player")
	}
	if _, ok := hub.players[second.ID]; !ok {
		t.Fatalf("hub players map missing second player")
	}

	if len(first.Obstacles) != len(hub.obstacles) {
		t.Fatalf("expected join response to include %d obstacles, got %d", len(hub.obstacles), len(first.Obstacles))
	}
	if len(first.Effects) != 0 {
		t.Fatalf("expected initial effect list to be empty, got %d", len(first.Effects))
	}
}

func TestUpdateIntentNormalizesVector(t *testing.T) {
	hub := newHub()
	playerID := "player-1"
	hub.players[playerID] = &playerState{Player: Player{ID: playerID, Facing: defaultFacing}}

	ok := hub.UpdateIntent(playerID, 10, 0, string(FacingRight))
	if !ok {
		t.Fatalf("expected UpdateIntent to succeed for existing player")
	}

	state := hub.players[playerID]
	if state.intentX <= 0 || state.intentX > 1 {
		t.Fatalf("expected normalized intentX in (0,1], got %f", state.intentX)
	}
	if state.intentY != 0 {
		t.Fatalf("expected intentY to remain 0, got %f", state.intentY)
	}
	if state.lastInput.IsZero() {
		t.Fatalf("expected lastInput to be recorded")
	}
	if state.Facing != FacingRight {
		t.Fatalf("expected facing to update to %q, got %q", FacingRight, state.Facing)
	}
}

func TestDeriveFacingFromMovement(t *testing.T) {
	tests := []struct {
		name       string
		dx, dy     float64
		fallback   FacingDirection
		wantFacing FacingDirection
	}{
		{name: "upwardsMovement", dx: 0, dy: -1, fallback: FacingRight, wantFacing: FacingUp},
		{name: "downwardsMovement", dx: 0, dy: 1, fallback: FacingLeft, wantFacing: FacingDown},
		{name: "leftMovement", dx: -1, dy: 0, fallback: FacingUp, wantFacing: FacingLeft},
		{name: "rightMovement", dx: 1, dy: 0, fallback: FacingUp, wantFacing: FacingRight},
		{name: "diagonalPrefersVertical", dx: 1, dy: -1, fallback: FacingDown, wantFacing: FacingUp},
		{name: "diagonalPrefersVerticalDown", dx: -1, dy: 1, fallback: FacingUp, wantFacing: FacingDown},
		{name: "zeroFallsBack", dx: 0, dy: 0, fallback: FacingLeft, wantFacing: FacingLeft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveFacing(tt.dx, tt.dy, tt.fallback); got != tt.wantFacing {
				t.Fatalf("deriveFacing(%f,%f,%q)=%q, want %q", tt.dx, tt.dy, tt.fallback, got, tt.wantFacing)
			}
		})
	}
}

func TestUpdateIntentDerivesFacingFromMovement(t *testing.T) {
	hub := newHub()
	playerID := "vector-facing"
	hub.players[playerID] = &playerState{Player: Player{ID: playerID, Facing: defaultFacing}}

	cases := []struct {
		name string
		dx   float64
		dy   float64
		want FacingDirection
	}{
		{name: "conflictingKeysUp", dx: 0, dy: -1, want: FacingUp},
		{name: "conflictingKeysDown", dx: 0, dy: 1, want: FacingDown},
		{name: "conflictingKeysLeft", dx: -1, dy: 0, want: FacingLeft},
		{name: "conflictingKeysRight", dx: 1, dy: 0, want: FacingRight},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !hub.UpdateIntent(playerID, tc.dx, tc.dy, string(FacingRight)) {
				t.Fatalf("UpdateIntent failed for player")
			}
			state := hub.players[playerID]
			if state.Facing != tc.want {
				t.Fatalf("expected facing %q, got %q", tc.want, state.Facing)
			}
		})
	}

	if !hub.UpdateIntent(playerID, 0, 0, string(FacingLeft)) {
		t.Fatalf("UpdateIntent failed for stationary update")
	}
	if got := hub.players[playerID].Facing; got != FacingLeft {
		t.Fatalf("expected stationary facing update to respect client facing, got %q", got)
	}
}

func TestAdvanceMovesAndClampsPlayers(t *testing.T) {
	hub := newHub()
	hub.obstacles = nil
	now := time.Now()

	moverID := "mover"
	boundaryID := "boundary"

	hub.players[moverID] = &playerState{
		Player:        Player{ID: moverID, X: 80, Y: 80, Facing: defaultFacing},
		intentX:       1,
		intentY:       0,
		lastHeartbeat: now,
	}
	hub.players[boundaryID] = &playerState{
		Player:        Player{ID: boundaryID, X: worldWidth - playerHalf - 5, Y: 100, Facing: defaultFacing},
		intentX:       1,
		lastHeartbeat: now,
	}

	players, _, toClose := hub.advance(now, 0.5)
	if len(toClose) != 0 {
		t.Fatalf("expected no subscribers to close, got %d", len(toClose))
	}

	mover := findPlayer(players, moverID)
	if mover == nil {
		t.Fatalf("updated snapshot missing mover")
	}
	expectedMoverX := 80 + moveSpeed*0.5
	if mover.X != expectedMoverX {
		t.Fatalf("expected mover X %.1f, got %.1f", expectedMoverX, mover.X)
	}

	boundary := findPlayer(players, boundaryID)
	if boundary == nil {
		t.Fatalf("updated snapshot missing boundary player")
	}
	expectedBoundaryX := worldWidth - playerHalf
	if boundary.X != expectedBoundaryX {
		t.Fatalf("expected boundary player to clamp to %.1f, got %.1f", expectedBoundaryX, boundary.X)
	}
}

func TestAdvanceRemovesStalePlayers(t *testing.T) {
	hub := newHub()
	hub.obstacles = nil
	staleID := "stale"
	hub.players[staleID] = &playerState{
		Player:        Player{ID: staleID, X: 100, Y: 100, Facing: defaultFacing},
		lastHeartbeat: time.Now().Add(-disconnectAfter - time.Second),
	}

	players, _, toClose := hub.advance(time.Now(), 0)
	if len(toClose) != 0 {
		t.Fatalf("expected no subscribers returned when none registered")
	}
	if findPlayer(players, staleID) != nil {
		t.Fatalf("stale player still present after advance")
	}
	if _, ok := hub.players[staleID]; ok {
		t.Fatalf("stale player still in hub map")
	}
}

func TestMeleeAttackCreatesEffectAndRespectsCooldown(t *testing.T) {
	hub := newHub()
	attackerID := "attacker"
	hub.players[attackerID] = &playerState{
		Player:        Player{ID: attackerID, X: 200, Y: 200, Facing: FacingRight},
		lastHeartbeat: time.Now(),
		cooldowns:     make(map[string]time.Time),
	}

	if !hub.HandleAction(attackerID, effectTypeAttack) {
		t.Fatalf("expected attack action to be recognized")
	}

	hub.mu.Lock()
	if len(hub.effects) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected exactly one effect after first attack, got %d", len(hub.effects))
	}
	first := hub.effects[0]
	if first.Type != effectTypeAttack {
		hub.mu.Unlock()
		t.Fatalf("expected effect type %q, got %q", effectTypeAttack, first.Type)
	}
	if first.Width <= 0 || first.Height <= 0 {
		hub.mu.Unlock()
		t.Fatalf("expected non-zero effect bounds, got width=%f height=%f", first.Width, first.Height)
	}
	firstStart := first.Start
	hub.mu.Unlock()

	hub.HandleAction(attackerID, effectTypeAttack)
	hub.mu.Lock()
	if len(hub.effects) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected cooldown to prevent new effect, have %d", len(hub.effects))
	}

	hub.players[attackerID].cooldowns[effectTypeAttack] = time.Now().Add(-meleeAttackCooldown)
	hub.mu.Unlock()

	hub.HandleAction(attackerID, effectTypeAttack)
	hub.mu.Lock()
	if len(hub.effects) != 2 {
		hub.mu.Unlock()
		t.Fatalf("expected second effect after cooldown reset, have %d", len(hub.effects))
	}
	second := hub.effects[1]
	hub.mu.Unlock()

	if second.ID == first.ID {
		t.Fatalf("expected unique effect IDs, both were %q", second.ID)
	}
	if second.Start < firstStart {
		t.Fatalf("expected second effect start (%d) to be >= first (%d)", second.Start, firstStart)
	}
}

func TestUpdateHeartbeatRecordsRTT(t *testing.T) {
	hub := newHub()
	playerID := "player"
	hub.players[playerID] = &playerState{Player: Player{ID: playerID, Facing: defaultFacing}}

	received := time.Now()
	clientSent := received.Add(-45 * time.Millisecond).UnixMilli()

	rtt, ok := hub.UpdateHeartbeat(playerID, received, clientSent)
	if !ok {
		t.Fatalf("expected heartbeat update to succeed")
	}
	if rtt <= 0 {
		t.Fatalf("expected positive RTT, got %s", rtt)
	}

	state := hub.players[playerID]
	if state.lastHeartbeat != received {
		t.Fatalf("lastHeartbeat not updated: want %v, got %v", received, state.lastHeartbeat)
	}
	if got := state.lastRTT; got != rtt {
		t.Fatalf("state lastRTT mismatch: want %s, got %s", rtt, got)
	}
}

func TestDiagnosticsSnapshotIncludesHeartbeatData(t *testing.T) {
	hub := newHub()
	playerID := "diag"
	now := time.Now()
	hub.players[playerID] = &playerState{
		Player:        Player{ID: playerID, X: 120, Y: 140, Facing: defaultFacing},
		lastHeartbeat: now,
		lastRTT:       30 * time.Millisecond,
	}

	snapshot := hub.DiagnosticsSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected diagnostics snapshot with 1 player, got %d", len(snapshot))
	}
	entry := snapshot[0]
	if entry.ID != playerID {
		t.Fatalf("expected diagnostics entry for %q, got %q", playerID, entry.ID)
	}
	if entry.LastHeartbeat != now.UnixMilli() {
		t.Fatalf("expected last heartbeat %d, got %d", now.UnixMilli(), entry.LastHeartbeat)
	}
	if entry.RTTMillis != 30 {
		t.Fatalf("expected RTTMillis 30, got %d", entry.RTTMillis)
	}
}

func TestPlayerStopsAtObstacle(t *testing.T) {
	hub := newHub()
	now := time.Now()

	hub.obstacles = []Obstacle{{
		ID:     "obstacle-test",
		X:      160,
		Y:      40,
		Width:  80,
		Height: 160,
	}}

	playerID := "block"
	hub.players[playerID] = &playerState{
		Player:        Player{ID: playerID, X: 100, Y: 120, Facing: defaultFacing},
		intentX:       1,
		intentY:       0,
		lastHeartbeat: now,
	}

	players, _, _ := hub.advance(now, 1)
	blocker := findPlayer(players, playerID)
	if blocker == nil {
		t.Fatalf("expected player in snapshot")
	}

	maxX := hub.obstacles[0].X - playerHalf
	if blocker.X > maxX+1e-6 {
		t.Fatalf("expected player to stop before obstacle at %.2f, got %.2f", maxX, blocker.X)
	}
}

func TestPlayersSeparateWhenColliding(t *testing.T) {
	hub := newHub()
	hub.obstacles = nil
	now := time.Now()

	firstID := "first"
	secondID := "second"

	hub.players[firstID] = &playerState{
		Player:        Player{ID: firstID, X: 300, Y: 200, Facing: defaultFacing},
		intentX:       1,
		intentY:       0,
		lastHeartbeat: now,
	}
	hub.players[secondID] = &playerState{
		Player:        Player{ID: secondID, X: 300 + playerHalf/2, Y: 200, Facing: defaultFacing},
		intentX:       -1,
		intentY:       0,
		lastHeartbeat: now,
	}

	players, _, _ := hub.advance(now, 1)

	first := findPlayer(players, firstID)
	second := findPlayer(players, secondID)
	if first == nil || second == nil {
		t.Fatalf("expected both players in snapshot")
	}

	dx := second.X - first.X
	dy := second.Y - first.Y
	distance := math.Hypot(dx, dy)
	minSeparation := playerHalf * 2
	if distance+1e-6 < minSeparation {
		t.Fatalf("expected players separated by at least %.2f, got %.2f", minSeparation, distance)
	}
}

func TestTriggerFireballCreatesProjectile(t *testing.T) {
	hub := newHub()
	shooterID := "shooter"
	now := time.Now()

	hub.players[shooterID] = &playerState{
		Player:        Player{ID: shooterID, X: 200, Y: 200, Facing: FacingRight},
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
	}

	if !hub.triggerFireball(shooterID) {
		t.Fatalf("expected triggerFireball to succeed")
	}

	if len(hub.effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(hub.effects))
	}

	eff := hub.effects[0]
	if eff.Type != effectTypeFireball {
		t.Fatalf("expected effect type %q, got %q", effectTypeFireball, eff.Type)
	}
	if eff.remainingRange != fireballRange {
		t.Fatalf("expected remaining range %.2f, got %.2f", fireballRange, eff.remainingRange)
	}
	if eff.velocityX <= 0 || eff.velocityY != 0 {
		t.Fatalf("expected projectile to move right, got velocity (%.2f, %.2f)", eff.velocityX, eff.velocityY)
	}
}

func TestMultiplePlayersCanTriggerFireballs(t *testing.T) {
	hub := newHub()
	now := time.Now()

	firstID := "player-1"
	secondID := "player-2"

	hub.players[firstID] = &playerState{
		Player:        Player{ID: firstID, X: 200, Y: 200, Facing: FacingRight},
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
	}
	hub.players[secondID] = &playerState{
		Player:        Player{ID: secondID, X: 260, Y: 200, Facing: FacingLeft},
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
	}

	if !hub.triggerFireball(firstID) {
		t.Fatalf("expected first player to trigger fireball")
	}
	if !hub.triggerFireball(secondID) {
		t.Fatalf("expected second player to trigger fireball")
	}

	if len(hub.effects) != 2 {
		t.Fatalf("expected two effects, got %d", len(hub.effects))
	}

	owners := map[string]bool{}
	for _, eff := range hub.effects {
		owners[eff.Owner] = true
	}

	if !owners[firstID] || !owners[secondID] {
		t.Fatalf("expected fireballs owned by both players, owners: %v", owners)
	}
}

func TestFireballBroadcastsToAllSubscribers(t *testing.T) {
	hub := newHub()
	stop := make(chan struct{})
	go hub.RunSimulation(stop)
	defer close(stop)

	mux := http.NewServeMux()
	mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
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

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		sub, players, effects, ok := hub.Subscribe(playerID, conn)
		if !ok {
			message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
			conn.WriteMessage(websocket.CloseMessage, message)
			conn.Close()
			return
		}

		initial := stateMessage{
			Type:       "state",
			Players:    players,
			Obstacles:  hub.obstacles,
			Effects:    effects,
			ServerTime: time.Now().UnixMilli(),
		}
		data, err := json.Marshal(initial)
		if err != nil {
			conn.Close()
			hub.Disconnect(playerID)
			return
		}

		sub.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			sub.mu.Unlock()
			hub.Disconnect(playerID)
			return
		}
		sub.mu.Unlock()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				hub.Disconnect(playerID)
				return
			}

			var msg clientMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case "input":
				hub.UpdateIntent(playerID, msg.DX, msg.DY, msg.Facing)
			case "action":
				if msg.Action == "" {
					continue
				}
				if !hub.HandleAction(playerID, msg.Action) {
					continue
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
					continue
				}

				sub.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					sub.mu.Unlock()
					hub.Disconnect(playerID)
					return
				}
				sub.mu.Unlock()
			}
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	wsBase := strings.Replace(server.URL, "http", "ws", 1)
	type testClient struct {
		id   string
		conn *websocket.Conn
	}

	clients := make([]testClient, 0, 3)
	for i := 0; i < 3; i++ {
		resp, err := server.Client().Post(server.URL+"/join", "application/json", nil)
		if err != nil {
			t.Fatalf("join request failed: %v", err)
		}
		var join joinResponse
		if err := json.NewDecoder(resp.Body).Decode(&join); err != nil {
			resp.Body.Close()
			t.Fatalf("failed to decode join response: %v", err)
		}
		resp.Body.Close()

		wsURL := wsBase + "/ws?id=" + url.QueryEscape(join.ID)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect websocket: %v", err)
		}

		conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, data, err := conn.ReadMessage(); err != nil {
			t.Fatalf("failed to read initial state: %v", err)
		} else {
			var initial stateMessage
			if err := json.Unmarshal(data, &initial); err != nil {
				t.Fatalf("failed to decode initial state: %v", err)
			}
			if initial.Type != "state" {
				t.Fatalf("expected initial message to be state, got %q", initial.Type)
			}
		}

		clients = append(clients, testClient{id: join.ID, conn: conn})
	}

	expectedOwners := make([]string, len(clients))
	for i, c := range clients {
		expectedOwners[i] = c.id
	}

	for _, c := range clients {
		msg := clientMessage{Type: "action", Action: effectTypeFireball}
		if err := c.conn.WriteJSON(msg); err != nil {
			t.Fatalf("failed to send action for %s: %v", c.id, err)
		}
	}

	for _, c := range clients {
		deadline := time.Now().Add(2 * time.Second)
		remaining := make(map[string]struct{}, len(expectedOwners))
		for _, id := range expectedOwners {
			remaining[id] = struct{}{}
		}

		for len(remaining) > 0 && time.Now().Before(deadline) {
			c.conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read state for %s: %v", c.id, err)
			}

			var msg stateMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("failed to decode state for %s: %v", c.id, err)
			}

			if msg.Type != "state" {
				continue
			}

			for _, eff := range msg.Effects {
				if eff.Type == effectTypeFireball {
					delete(remaining, eff.Owner)
				}
			}
		}

		if len(remaining) > 0 {
			t.Fatalf("player %s did not receive fireballs from players: %v", c.id, remaining)
		}
	}

	for _, c := range clients {
		c.conn.Close()
	}
}

func TestFireballExpiresOnObstacleCollision(t *testing.T) {
	hub := newHub()
	now := time.Now()

	shooterID := "caster"
	hub.players[shooterID] = &playerState{
		Player:        Player{ID: shooterID, X: 200, Y: 200, Facing: FacingRight},
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
	}

	hub.obstacles = []Obstacle{{
		ID:     "wall",
		X:      260,
		Y:      180,
		Width:  40,
		Height: 40,
	}}

	if !hub.triggerFireball(shooterID) {
		t.Fatalf("expected fireball to be created")
	}

	step := time.Second / time.Duration(tickRate)
	dt := 1.0 / float64(tickRate)
	expired := false
	current := now

	for i := 0; i < tickRate*2; i++ {
		current = current.Add(step)
		hub.advance(current, dt)
		if len(hub.effects) == 0 {
			expired = true
			break
		}
	}

	if !expired {
		t.Fatalf("expected fireball to expire after hitting obstacle")
	}
}
