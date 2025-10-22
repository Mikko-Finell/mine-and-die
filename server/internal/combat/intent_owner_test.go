package combat

import "testing"

func TestNewMeleeIntentOwnerFromActor(t *testing.T) {
	owner, ok := NewMeleeIntentOwnerFromActor(nil)
	if ok {
		t.Fatal("expected nil actor to be rejected")
	}
	if owner != (MeleeIntentOwner{}) {
		t.Fatalf("expected zero owner from nil actor, got %#v", owner)
	}

	owner, ok = NewMeleeIntentOwnerFromActor(&AbilityActor{})
	if ok {
		t.Fatal("expected empty actor ID to be rejected")
	}

	actor := &AbilityActor{ID: "player", X: 64, Y: 96, Facing: "up"}
	owner, ok = NewMeleeIntentOwnerFromActor(actor)
	if !ok {
		t.Fatal("expected melee intent owner to be constructed")
	}

	if owner.ID != actor.ID {
		t.Fatalf("expected owner ID %q, got %q", actor.ID, owner.ID)
	}
	if owner.X != actor.X || owner.Y != actor.Y {
		t.Fatalf("expected owner position (%f,%f), got (%f,%f)", actor.X, actor.Y, owner.X, owner.Y)
	}
	if owner.Facing != actor.Facing {
		t.Fatalf("expected owner facing %q, got %q", actor.Facing, owner.Facing)
	}
}

func TestNewProjectileIntentOwnerFromActor(t *testing.T) {
	owner, ok := NewProjectileIntentOwnerFromActor(nil)
	if ok {
		t.Fatal("expected nil actor to be rejected")
	}
	if owner != (ProjectileIntentOwner{}) {
		t.Fatalf("expected zero owner from nil actor, got %#v", owner)
	}

	actor := &AbilityActor{ID: "caster", X: 48, Y: 72, Facing: "right"}
	owner, ok = NewProjectileIntentOwnerFromActor(actor)
	if !ok {
		t.Fatal("expected projectile intent owner to be constructed")
	}

	if owner.ID != actor.ID {
		t.Fatalf("expected owner ID %q, got %q", actor.ID, owner.ID)
	}
	if owner.X != actor.X || owner.Y != actor.Y {
		t.Fatalf("expected owner position (%f,%f), got (%f,%f)", actor.X, actor.Y, owner.X, owner.Y)
	}
	if owner.Facing != actor.Facing {
		t.Fatalf("expected owner facing %q, got %q", actor.Facing, owner.Facing)
	}
}
