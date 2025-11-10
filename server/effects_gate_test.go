package server

import (
	"testing"
	"time"

	worldpkg "mine-and-die/server/internal/world"
)

func TestConfigureMeleeAbilityGatePopulatesIntentOwner(t *testing.T) {
	const actorID = "player-gate"

	player := &playerState{
		ActorState: actorState{Actor: Actor{
			ID:     actorID,
			X:      180,
			Y:      140,
			Facing: FacingUp,
		}},
		Cooldowns: make(map[string]time.Time),
	}

	internalWorld, err := worldpkg.New(worldpkg.Config{}, worldpkg.Deps{})
	if err != nil {
		t.Fatalf("failed to construct internal world: %v", err)
	}
	internalWorld.Players()[actorID] = player

	world := &World{
		players: map[string]*playerState{actorID: player},
	}
	world.internalWorld = internalWorld
	world.configureAbilityOwnerAdapters()

	gates, ok := internalWorld.AbilityGates()
	if !ok {
		t.Fatal("expected internal world to expose ability gates")
	}
	world.meleeAbilityGate = gates.Melee
	if world.meleeAbilityGate == nil {
		t.Fatal("expected melee ability gate to be configured")
	}

	owner, ok := world.meleeAbilityGate(actorID, time.Unix(0, 0))
	if !ok {
		t.Fatal("expected melee ability gate to return ability owner")
	}

	if owner.ID != actorID {
		t.Fatalf("expected owner ID %q, got %q", actorID, owner.ID)
	}
	if owner.X != 180 || owner.Y != 140 {
		t.Fatalf("expected owner position (180,140), got (%f,%f)", owner.X, owner.Y)
	}
	if owner.Facing != string(FacingUp) {
		t.Fatalf("expected owner facing %q, got %q", FacingUp, owner.Facing)
	}
}

func TestConfigureProjectileAbilityGatePopulatesIntentOwner(t *testing.T) {
	const actorID = "caster-gate"

	player := &playerState{
		ActorState: actorState{Actor: Actor{
			ID:     actorID,
			X:      120,
			Y:      160,
			Facing: FacingRight,
		}},
		Cooldowns: make(map[string]time.Time),
	}

	internalWorld, err := worldpkg.New(worldpkg.Config{}, worldpkg.Deps{})
	if err != nil {
		t.Fatalf("failed to construct internal world: %v", err)
	}
	internalWorld.Players()[actorID] = player

	world := &World{
		players:             map[string]*playerState{actorID: player},
		projectileTemplates: newProjectileTemplates(),
	}
	world.internalWorld = internalWorld
	world.configureAbilityOwnerAdapters()

	gateOptions, ok := internalWorld.AbilityGateOptions()
	if !ok {
		t.Fatal("expected internal world to expose ability gate options")
	}

	projectileOpts := gateOptions.Projectile
	if tpl := world.projectileTemplates[effectTypeFireball]; tpl != nil {
		projectileOpts.AbilityID = tpl.Type
		projectileOpts.Cooldown = tpl.Cooldown
	}

	gate, ok := internalWorld.BindProjectileAbilityGate(projectileOpts)
	if !ok {
		t.Fatal("expected projectile ability gate to bind")
	}
	world.projectileAbilityGate = gate
	if world.projectileAbilityGate == nil {
		t.Fatal("expected projectile ability gate to be configured")
	}

	owner, ok := world.projectileAbilityGate(actorID, time.Unix(0, 0))
	if !ok {
		t.Fatal("expected projectile ability gate to return ability owner")
	}

	if owner.ID != actorID {
		t.Fatalf("expected owner ID %q, got %q", actorID, owner.ID)
	}
	if owner.X != 120 || owner.Y != 160 {
		t.Fatalf("expected owner position (120,160), got (%f,%f)", owner.X, owner.Y)
	}
	if owner.Facing != string(FacingRight) {
		t.Fatalf("expected owner facing %q, got %q", FacingRight, owner.Facing)
	}
}
