package world

import (
	"errors"
	"testing"
	"time"
)

func TestResolveMeleeImpactAwardsGoldToNPC(t *testing.T) {
	effect := &struct{}{}
	area := Obstacle{X: 0, Y: 0, Width: 64, Height: 64}
	obstacles := []Obstacle{{ID: "ore-1", Type: ObstacleTypeGoldOre, X: 0, Y: 0, Width: 64, Height: 64}}

	t.Run("awards gold", func(t *testing.T) {
		t.Helper()

		var npcGranted bool
		var ownerCalled bool
		var playerAttempted bool
		cfg := ResolveMeleeImpactConfig{
			EffectType:    "attack",
			Effect:        effect,
			Owner:         &struct{}{},
			ActorID:       "npc-1",
			Tick:          1,
			Now:           time.Now(),
			Area:          area,
			Obstacles:     obstacles,
			ForEachPlayer: func(func(string, float64, float64, any)) {},
			ForEachNPC:    func(func(string, float64, float64, any)) {},
			GivePlayerGold: func(string) (bool, error) {
				playerAttempted = true
				return false, nil
			},
			GiveNPCGold: func(id string) (bool, error) {
				if id != "npc-1" {
					t.Fatalf("expected grant for npc-1, got %s", id)
				}
				npcGranted = true
				return true, nil
			},
			GiveOwnerGold: func(any) error {
				ownerCalled = true
				return nil
			},
			RecordGoldGrantFailure: func(actorID, obstacleID string, err error) {
				t.Fatalf("unexpected failure logging for %s/%s: %v", actorID, obstacleID, err)
			},
			RecordAttackOverlap: func(string, uint64, string, []string, []string) {},
		}

		ResolveMeleeImpact(cfg)

		if !npcGranted {
			t.Fatalf("expected NPC gold grant to be attempted")
		}
		if ownerCalled {
			t.Fatalf("expected owner grant to be skipped when NPC grant succeeds")
		}
		if !playerAttempted {
			t.Fatalf("expected player grant check to run before NPC grant")
		}
	})

	t.Run("logs failure", func(t *testing.T) {
		t.Helper()

		var failureLogged bool
		var playerAttempted bool
		cfg := ResolveMeleeImpactConfig{
			EffectType:    "attack",
			Effect:        effect,
			ActorID:       "npc-2",
			Tick:          2,
			Now:           time.Now(),
			Area:          area,
			Obstacles:     obstacles,
			ForEachPlayer: func(func(string, float64, float64, any)) {},
			ForEachNPC:    func(func(string, float64, float64, any)) {},
			GivePlayerGold: func(string) (bool, error) {
				playerAttempted = true
				return false, nil
			},
			GiveNPCGold: func(id string) (bool, error) {
				if id != "npc-2" {
					t.Fatalf("expected grant for npc-2, got %s", id)
				}
				return true, errors.New("grant failed")
			},
			GiveOwnerGold: func(any) error { return nil },
			RecordGoldGrantFailure: func(actorID, obstacleID string, err error) {
				if actorID != "npc-2" {
					t.Fatalf("unexpected actor id %s", actorID)
				}
				if obstacleID != "ore-1" {
					t.Fatalf("unexpected obstacle %s", obstacleID)
				}
				if err == nil {
					t.Fatalf("expected error to be reported")
				}
				failureLogged = true
			},
			RecordAttackOverlap: func(string, uint64, string, []string, []string) {},
		}

		ResolveMeleeImpact(cfg)

		if !failureLogged {
			t.Fatalf("expected failure to be reported when grant errors")
		}
		if !playerAttempted {
			t.Fatalf("expected player grant check to run before NPC grant")
		}
	})
}

func TestResolveMeleeImpactAppliesHitsAndTelemetry(t *testing.T) {
	effect := &struct{}{}
	area := Obstacle{X: 0, Y: 0, Width: 128, Height: 128}

	var appliedPlayer, appliedNPC bool
	var recordedActor string
	var recordedTick uint64
	var recordedAbility string
	var recordedPlayerHits, recordedNPCHits []string

	cfg := ResolveMeleeImpactConfig{
		EffectType: "attack",
		Effect:     effect,
		ActorID:    "attacker",
		Tick:       42,
		Now:        time.Now(),
		Area:       area,
		Obstacles:  nil,
		ForEachPlayer: func(visit func(string, float64, float64, any)) {
			visit("player-1", 32, 32, "player-ref")
		},
		ForEachNPC: func(visit func(string, float64, float64, any)) {
			visit("npc-1", 40, 32, "npc-ref")
		},
		GivePlayerGold: func(string) (bool, error) { return false, nil },
		GiveNPCGold:    func(string) (bool, error) { return false, nil },
		GiveOwnerGold:  func(any) error { return nil },
		ApplyPlayerHit: func(effectRef any, target any, when time.Time) {
			if effectRef != effect {
				t.Fatalf("expected effect reference to be passed through")
			}
			if target != "player-ref" {
				t.Fatalf("unexpected player target %v", target)
			}
			appliedPlayer = true
		},
		ApplyNPCHit: func(effectRef any, target any, when time.Time) {
			if effectRef != effect {
				t.Fatalf("expected effect reference to be passed through")
			}
			if target != "npc-ref" {
				t.Fatalf("unexpected npc target %v", target)
			}
			appliedNPC = true
		},
		RecordGoldGrantFailure: func(string, string, error) {
			t.Fatalf("did not expect gold grant failure")
		},
		RecordAttackOverlap: func(actorID string, tick uint64, ability string, playerHits []string, npcHits []string) {
			recordedActor = actorID
			recordedTick = tick
			recordedAbility = ability
			recordedPlayerHits = append([]string{}, playerHits...)
			recordedNPCHits = append([]string{}, npcHits...)
		},
	}

	ResolveMeleeImpact(cfg)

	if !appliedPlayer {
		t.Fatalf("expected player hit callback to run")
	}
	if !appliedNPC {
		t.Fatalf("expected npc hit callback to run")
	}
	if recordedActor != "attacker" {
		t.Fatalf("expected telemetry actor 'attacker', got %s", recordedActor)
	}
	if recordedTick != 42 {
		t.Fatalf("expected telemetry tick 42, got %d", recordedTick)
	}
	if recordedAbility != "attack" {
		t.Fatalf("expected ability 'attack', got %s", recordedAbility)
	}
	if len(recordedPlayerHits) != 1 || recordedPlayerHits[0] != "player-1" {
		t.Fatalf("expected player hits [player-1], got %v", recordedPlayerHits)
	}
	if len(recordedNPCHits) != 1 || recordedNPCHits[0] != "npc-1" {
		t.Fatalf("expected npc hits [npc-1], got %v", recordedNPCHits)
	}
}
