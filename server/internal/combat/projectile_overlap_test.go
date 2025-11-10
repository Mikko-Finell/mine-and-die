package combat

import (
	"reflect"
	"testing"

	internaleffects "mine-and-die/server/internal/effects"
)

func TestResolveProjectileOverlapsHitsPlayersAndNPCs(t *testing.T) {
	projectile := &internaleffects.ProjectileState{}
	area := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}

	var playerCallbacks []string
	var npcCallbacks []string
	var recordedOwner string
	var recordedTick uint64
	var recordedAbility string
	var recordedPlayers []string
	var recordedNPCs []string
	var recordedMetadata map[string]any
	recordCount := 0

	cfg := ProjectileOverlapResolutionConfig{
		Projectile: projectile,
		Impact: ProjectileImpactRules{
			StopOnHit:    false,
			MaxTargets:   0,
			AffectsOwner: false,
		},
		OwnerID:  "owner",
		Ability:  "fireball",
		Tick:     42,
		Metadata: map[string]any{"projectile": "fireball"},
		Area:     area,
		RecordAttackOverlap: func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any) {
			recordCount++
			recordedOwner = ownerID
			recordedTick = tick
			recordedAbility = ability
			recordedPlayers = append([]string(nil), playerHits...)
			recordedNPCs = append([]string(nil), npcHits...)
			recordedMetadata = metadata
		},
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			ownerTarget := ProjectileOverlapTarget{ID: "owner", X: 0, Y: 0, Radius: 1, Raw: "owner"}
			if !visitor(ownerTarget) {
				return
			}
			overlappingPlayer := ProjectileOverlapTarget{ID: "player-1", X: 5, Y: 5, Radius: 1, Raw: "player-1"}
			if !visitor(overlappingPlayer) {
				return
			}
			nonOverlapping := ProjectileOverlapTarget{ID: "player-miss", X: 20, Y: 20, Radius: 1, Raw: "player-miss"}
			_ = visitor(nonOverlapping)
		},
		VisitNPCs: func(visitor ProjectileOverlapVisitor) {
			overlappingNPC := ProjectileOverlapTarget{ID: "npc-1", X: 5, Y: 5, Radius: 1, Raw: "npc-1"}
			_ = visitor(overlappingNPC)
		},
		OnPlayerHit: func(target ProjectileOverlapTarget) {
			playerCallbacks = append(playerCallbacks, target.ID)
		},
		OnNPCHit: func(target ProjectileOverlapTarget) {
			npcCallbacks = append(npcCallbacks, target.ID)
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if result.HitsApplied != 2 {
		t.Fatalf("expected 2 hits applied, got %d", result.HitsApplied)
	}
	if result.ShouldStop {
		t.Fatalf("expected projectile to continue, got stop")
	}
	if projectile.HitCount != 2 {
		t.Fatalf("expected projectile hit count 2, got %d", projectile.HitCount)
	}
	if recordCount != 1 {
		t.Fatalf("expected telemetry recorded once, got %d", recordCount)
	}
	if recordedOwner != "owner" || recordedTick != 42 || recordedAbility != "fireball" {
		t.Fatalf("unexpected telemetry metadata: owner=%s tick=%d ability=%s", recordedOwner, recordedTick, recordedAbility)
	}
	if !reflect.DeepEqual(recordedPlayers, []string{"player-1"}) {
		t.Fatalf("unexpected player telemetry hits: %v", recordedPlayers)
	}
	if !reflect.DeepEqual(recordedNPCs, []string{"npc-1"}) {
		t.Fatalf("unexpected NPC telemetry hits: %v", recordedNPCs)
	}
	if recordedMetadata["projectile"] != "fireball" {
		t.Fatalf("expected metadata to include projectile type, got %v", recordedMetadata)
	}
	if !reflect.DeepEqual(playerCallbacks, []string{"player-1"}) {
		t.Fatalf("expected player callback for player-1, got %v", playerCallbacks)
	}
	if !reflect.DeepEqual(npcCallbacks, []string{"npc-1"}) {
		t.Fatalf("expected NPC callback for npc-1, got %v", npcCallbacks)
	}
}

func TestResolveProjectileOverlapsStopOnHitSkipsNPCs(t *testing.T) {
	projectile := &internaleffects.ProjectileState{}
	area := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}

	npcVisited := false
	cfg := ProjectileOverlapResolutionConfig{
		Projectile: projectile,
		Impact:     ProjectileImpactRules{StopOnHit: true},
		OwnerID:    "owner",
		Ability:    "attack",
		Tick:       7,
		Metadata:   map[string]any{"projectile": "attack"},
		Area:       area,
		RecordAttackOverlap: func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any) {
		},
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			first := ProjectileOverlapTarget{ID: "player-1", X: 5, Y: 5, Radius: 1}
			if !visitor(first) {
				return
			}
			second := ProjectileOverlapTarget{ID: "player-2", X: 6, Y: 6, Radius: 1}
			_ = visitor(second)
		},
		VisitNPCs: func(visitor ProjectileOverlapVisitor) {
			npcVisited = true
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if !result.ShouldStop {
		t.Fatalf("expected stop on hit")
	}
	if projectile.HitCount != 1 {
		t.Fatalf("expected one hit recorded, got %d", projectile.HitCount)
	}
	if npcVisited {
		t.Fatalf("expected NPC iterator to be skipped after stop")
	}
}

func TestResolveProjectileOverlapsMaxTargetsEnforced(t *testing.T) {
	projectile := &internaleffects.ProjectileState{}
	area := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}

	cfg := ProjectileOverlapResolutionConfig{
		Projectile: projectile,
		Impact:     ProjectileImpactRules{MaxTargets: 1},
		OwnerID:    "owner",
		Area:       area,
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			first := ProjectileOverlapTarget{ID: "player-1", X: 5, Y: 5, Radius: 1}
			if !visitor(first) {
				return
			}
			second := ProjectileOverlapTarget{ID: "player-2", X: 6, Y: 6, Radius: 1}
			_ = visitor(second)
		},
		VisitNPCs: func(visitor ProjectileOverlapVisitor) {
			npc := ProjectileOverlapTarget{ID: "npc-1", X: 5, Y: 5, Radius: 1}
			_ = visitor(npc)
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if !result.ShouldStop {
		t.Fatalf("expected stop after reaching max targets")
	}
	if projectile.HitCount != 1 {
		t.Fatalf("expected exactly one hit recorded, got %d", projectile.HitCount)
	}
}

func TestResolveProjectileOverlapsSkipsDuplicateHits(t *testing.T) {
	projectile := &internaleffects.ProjectileState{HitActors: map[string]struct{}{"player-1": {}}}
	projectile.HitCount = 1
	area := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}

	cfg := ProjectileOverlapResolutionConfig{
		Projectile: projectile,
		Impact:     ProjectileImpactRules{},
		OwnerID:    "owner",
		Area:       area,
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			duplicate := ProjectileOverlapTarget{ID: "player-1", X: 5, Y: 5, Radius: 1}
			if !visitor(duplicate) {
				return
			}
			fresh := ProjectileOverlapTarget{ID: "player-2", X: 5, Y: 5, Radius: 1}
			_ = visitor(fresh)
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if result.HitsApplied != 1 {
		t.Fatalf("expected one new hit applied, got %d", result.HitsApplied)
	}
	if projectile.HitCount != 2 {
		t.Fatalf("expected hit count to increment to 2, got %d", projectile.HitCount)
	}
}

func TestResolveProjectileOverlapsAllowsOwnerWhenConfigured(t *testing.T) {
	projectile := &internaleffects.ProjectileState{}
	area := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}

	cfg := ProjectileOverlapResolutionConfig{
		Projectile: projectile,
		Impact:     ProjectileImpactRules{AffectsOwner: true},
		OwnerID:    "owner",
		Area:       area,
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			owner := ProjectileOverlapTarget{ID: "owner", X: 5, Y: 5, Radius: 1}
			_ = visitor(owner)
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if result.HitsApplied != 1 {
		t.Fatalf("expected owner hit to register, got %d", result.HitsApplied)
	}
	if projectile.HitCount != 1 {
		t.Fatalf("expected hit count 1, got %d", projectile.HitCount)
	}
}

func TestResolveProjectileOverlapsNilProjectile(t *testing.T) {
	visited := false
	cfg := ProjectileOverlapResolutionConfig{
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			visited = true
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if result.HitsApplied != 0 || result.ShouldStop {
		t.Fatalf("expected zero result for nil projectile: %+v", result)
	}
	if visited {
		t.Fatalf("expected iterators to be skipped when projectile is nil")
	}
}

func TestResolveProjectileOverlapsNoTelemetryWhenNoHits(t *testing.T) {
	projectile := &internaleffects.ProjectileState{}
	area := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}

	recordCount := 0
	cfg := ProjectileOverlapResolutionConfig{
		Projectile: projectile,
		Impact:     ProjectileImpactRules{},
		OwnerID:    "owner",
		Ability:    "fireball",
		Tick:       100,
		Area:       area,
		RecordAttackOverlap: func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any) {
			recordCount++
		},
		VisitPlayers: func(visitor ProjectileOverlapVisitor) {
			miss := ProjectileOverlapTarget{ID: "player-miss", X: 20, Y: 20, Radius: 1}
			_ = visitor(miss)
		},
	}

	result := ResolveProjectileOverlaps(cfg)

	if result.HitsApplied != 0 {
		t.Fatalf("expected no hits, got %d", result.HitsApplied)
	}
	if recordCount != 0 {
		t.Fatalf("expected no telemetry when no hits, got %d", recordCount)
	}
}
