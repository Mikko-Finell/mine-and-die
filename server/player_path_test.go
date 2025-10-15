package main

import (
	"math"
	"testing"

	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestFollowPlayerPathNormalizesIntentVectors(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{
		actorState: actorState{Actor: Actor{
			ID:        "path-player",
			X:         100,
			Y:         100,
			Health:    baselinePlayerMaxHealth,
			MaxHealth: baselinePlayerMaxHealth,
		}},
		stats: stats.DefaultComponent(stats.ArchetypePlayer),
		path: playerPathState{
			Path:       []vec2{{X: 340, Y: 220}},
			PathTarget: vec2{X: 340, Y: 220},
		},
	}

	w.AddPlayer(player)

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches before path follow, got %d", len(patches))
	}

	rawDX := player.path.Path[0].X - player.X
	rawDY := player.path.Path[0].Y - player.Y
	if math.Hypot(rawDX, rawDY) <= 1 {
		t.Fatalf("test setup error: raw delta %.2f, %.2f should exceed unit length", rawDX, rawDY)
	}

	w.followPlayerPath(player, 0)

	patches := w.snapshotPatchesLocked()
	if len(patches) == 0 {
		t.Fatal("expected patches after following path, got none")
	}

	var intentPayload PlayerIntentPayload
	found := false
	for _, patch := range patches {
		if patch.Kind != PatchPlayerIntent {
			continue
		}

		payload, ok := patch.Payload.(PlayerIntentPayload)
		if !ok {
			t.Fatalf("expected PlayerIntentPayload, got %T", patch.Payload)
		}
		intentPayload = payload
		found = true
		break
	}

	if !found {
		t.Fatalf("expected PlayerIntent patch, got %d patches", len(patches))
	}

	dist := math.Hypot(rawDX, rawDY)
	expectedDX := rawDX
	expectedDY := rawDY
	if dist > 1 {
		expectedDX = rawDX / dist
		expectedDY = rawDY / dist
	}

	if math.Abs(intentPayload.DX-expectedDX) > 1e-6 || math.Abs(intentPayload.DY-expectedDY) > 1e-6 {
		t.Fatalf("expected normalized intent (%.6f, %.6f), got (%.6f, %.6f)", expectedDX, expectedDY, intentPayload.DX, intentPayload.DY)
	}

	if math.Hypot(intentPayload.DX, intentPayload.DY) > 1+1e-6 {
		t.Fatalf("expected unit-length intent, got magnitude %.6f", math.Hypot(intentPayload.DX, intentPayload.DY))
	}
}
