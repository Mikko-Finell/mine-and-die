package main

import (
	"testing"
	"time"
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
}

func TestUpdateIntentNormalizesVector(t *testing.T) {
	hub := newHub()
	playerID := "player-1"
	hub.players[playerID] = &playerState{Player: Player{ID: playerID}}

	ok := hub.UpdateIntent(playerID, 10, 0)
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
}

func TestAdvanceMovesAndClampsPlayers(t *testing.T) {
	hub := newHub()
	now := time.Now()

	moverID := "mover"
	boundaryID := "boundary"

	hub.players[moverID] = &playerState{
		Player:        Player{ID: moverID, X: 80, Y: 80},
		intentX:       1,
		intentY:       0,
		lastHeartbeat: now,
	}
	hub.players[boundaryID] = &playerState{
		Player:        Player{ID: boundaryID, X: worldWidth - playerHalf - 5, Y: 100},
		intentX:       1,
		lastHeartbeat: now,
	}

	players, toClose := hub.advance(now, 0.5)
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
	staleID := "stale"
	hub.players[staleID] = &playerState{
		Player:        Player{ID: staleID, X: 100, Y: 100},
		lastHeartbeat: time.Now().Add(-disconnectAfter - time.Second),
	}

	players, toClose := hub.advance(time.Now(), 0)
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

func TestUpdateHeartbeatRecordsRTT(t *testing.T) {
	hub := newHub()
	playerID := "player"
	hub.players[playerID] = &playerState{Player: Player{ID: playerID}}

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
		Player:        Player{ID: playerID, X: 120, Y: 140},
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
