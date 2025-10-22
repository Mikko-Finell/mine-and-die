package combat

import (
	"context"
	"testing"

	"mine-and-die/server/logging"
	loggingcombat "mine-and-die/server/logging/combat"
)

type capturePublisher struct {
	events []logging.Event
}

func (p *capturePublisher) Publish(ctx context.Context, event logging.Event) {
	p.events = append(p.events, event)
}

func TestNewDamageTelemetryRecorder(t *testing.T) {
	pub := &capturePublisher{}

	recorder := NewDamageTelemetryRecorder(DamageTelemetryRecorderConfig{
		Publisher: pub,
		LookupEntity: func(id string) logging.EntityRef {
			return logging.EntityRef{ID: "mapped-" + id, Kind: logging.EntityKind("kind-" + id)}
		},
		CurrentTick: func() uint64 { return 42 },
	})

	if recorder == nil {
		t.Fatalf("expected recorder")
	}

	effect := EffectRef{Effect: Effect{OwnerID: "caster", Type: EffectTypeFireball}}
	target := ActorRef{Actor: Actor{ID: "victim"}}

	recorder(effect, target, 12.5, 3.5, StatusEffectBurning)

	if len(pub.events) != 1 {
		t.Fatalf("expected one event, got %d", len(pub.events))
	}

	event := pub.events[0]
	if event.Type != loggingcombat.EventDamage {
		t.Fatalf("unexpected event type %q", event.Type)
	}
	if event.Tick != 42 {
		t.Fatalf("unexpected tick %d", event.Tick)
	}
	if event.Actor.ID != "mapped-caster" || event.Actor.Kind != logging.EntityKind("kind-caster") {
		t.Fatalf("unexpected actor ref: %+v", event.Actor)
	}
	if len(event.Targets) != 1 {
		t.Fatalf("expected single target, got %d", len(event.Targets))
	}
	if event.Targets[0].ID != "mapped-victim" || event.Targets[0].Kind != logging.EntityKind("kind-victim") {
		t.Fatalf("unexpected target ref: %+v", event.Targets[0])
	}

	payload, ok := event.Payload.(loggingcombat.DamagePayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", event.Payload)
	}
	if payload.Ability != EffectTypeFireball {
		t.Fatalf("unexpected ability %q", payload.Ability)
	}
	if payload.Amount != 12.5 {
		t.Fatalf("unexpected damage %f", payload.Amount)
	}
	if payload.TargetHealth != 3.5 {
		t.Fatalf("unexpected target health %f", payload.TargetHealth)
	}
	if payload.StatusEffect != StatusEffectBurning {
		t.Fatalf("unexpected status %q", payload.StatusEffect)
	}
}

func TestNewDefeatTelemetryRecorder(t *testing.T) {
	pub := &capturePublisher{}

	recorder := NewDefeatTelemetryRecorder(DefeatTelemetryRecorderConfig{
		Publisher: pub,
		LookupEntity: func(id string) logging.EntityRef {
			return logging.EntityRef{ID: "mapped-" + id, Kind: logging.EntityKind("kind-" + id)}
		},
		// Intentionally omit CurrentTick to exercise the default path.
	})

	if recorder == nil {
		t.Fatalf("expected recorder")
	}

	effect := EffectRef{Effect: Effect{OwnerID: "caster", Type: EffectTypeAttack}}
	target := ActorRef{Actor: Actor{ID: "defeated"}}

	recorder(effect, target, StatusEffectBurning)

	if len(pub.events) != 1 {
		t.Fatalf("expected one event, got %d", len(pub.events))
	}

	event := pub.events[0]
	if event.Type != loggingcombat.EventDefeat {
		t.Fatalf("unexpected event type %q", event.Type)
	}
	if event.Tick != 0 {
		t.Fatalf("expected default tick 0, got %d", event.Tick)
	}
	if event.Actor.ID != "mapped-caster" || event.Actor.Kind != logging.EntityKind("kind-caster") {
		t.Fatalf("unexpected actor ref: %+v", event.Actor)
	}
	if len(event.Targets) != 1 {
		t.Fatalf("expected single target, got %d", len(event.Targets))
	}
	if event.Targets[0].ID != "mapped-defeated" || event.Targets[0].Kind != logging.EntityKind("kind-defeated") {
		t.Fatalf("unexpected target ref: %+v", event.Targets[0])
	}

	payload, ok := event.Payload.(loggingcombat.DefeatPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", event.Payload)
	}
	if payload.Ability != EffectTypeAttack {
		t.Fatalf("unexpected ability %q", payload.Ability)
	}
	if payload.StatusEffect != StatusEffectBurning {
		t.Fatalf("unexpected status %q", payload.StatusEffect)
	}
}

func TestNewDamageTelemetryRecorderNilPublisher(t *testing.T) {
	if recorder := NewDamageTelemetryRecorder(DamageTelemetryRecorderConfig{}); recorder != nil {
		t.Fatalf("expected nil recorder when publisher missing")
	}
}

func TestNewDefeatTelemetryRecorderNilPublisher(t *testing.T) {
	if recorder := NewDefeatTelemetryRecorder(DefeatTelemetryRecorderConfig{}); recorder != nil {
		t.Fatalf("expected nil recorder when publisher missing")
	}
}

func TestNewAttackOverlapTelemetryRecorder(t *testing.T) {
	pub := &capturePublisher{}

	recorder := NewAttackOverlapTelemetryRecorder(AttackOverlapTelemetryRecorderConfig{
		Publisher: pub,
		LookupEntity: func(id string) logging.EntityRef {
			return logging.EntityRef{ID: "mapped-" + id, Kind: logging.EntityKind("kind-" + id)}
		},
	})

	if recorder == nil {
		t.Fatalf("expected recorder")
	}

	metadata := map[string]any{"projectile": "fireball"}
	recorder("caster", 17, "ability.test", []string{"player-1"}, []string{"npc-1", "npc-2"}, metadata)

	if len(pub.events) != 1 {
		t.Fatalf("expected one event, got %d", len(pub.events))
	}

	event := pub.events[0]
	if event.Type != loggingcombat.EventAttackOverlap {
		t.Fatalf("unexpected event type %q", event.Type)
	}
	if event.Tick != 17 {
		t.Fatalf("unexpected tick %d", event.Tick)
	}
	if event.Actor.ID != "mapped-caster" || event.Actor.Kind != logging.EntityKind("kind-caster") {
		t.Fatalf("unexpected actor ref: %+v", event.Actor)
	}
	if len(event.Targets) != 3 {
		t.Fatalf("expected three targets, got %d", len(event.Targets))
	}
	expectedTargets := []logging.EntityRef{
		{ID: "mapped-player-1", Kind: logging.EntityKind("kind-player-1")},
		{ID: "mapped-npc-1", Kind: logging.EntityKind("kind-npc-1")},
		{ID: "mapped-npc-2", Kind: logging.EntityKind("kind-npc-2")},
	}
	for i, target := range event.Targets {
		if target != expectedTargets[i] {
			t.Fatalf("unexpected target %d: %+v", i, target)
		}
	}

	payload, ok := event.Payload.(loggingcombat.AttackOverlapPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", event.Payload)
	}
	if payload.Ability != "ability.test" {
		t.Fatalf("unexpected ability %q", payload.Ability)
	}
	if len(payload.PlayerHits) != 1 || payload.PlayerHits[0] != "player-1" {
		t.Fatalf("unexpected player hits: %#v", payload.PlayerHits)
	}
	if len(payload.NPCHits) != 2 || payload.NPCHits[0] != "npc-1" || payload.NPCHits[1] != "npc-2" {
		t.Fatalf("unexpected NPC hits: %#v", payload.NPCHits)
	}
	if event.Extra == nil || event.Extra["projectile"] != "fireball" {
		t.Fatalf("unexpected metadata: %#v", event.Extra)
	}

	recorder("caster", 18, "ability.test", nil, nil, nil)
	if len(pub.events) != 1 {
		t.Fatalf("expected recorder to ignore empty hits, got %d events", len(pub.events))
	}
}

func TestNewAttackOverlapTelemetryRecorderNilPublisher(t *testing.T) {
	if recorder := NewAttackOverlapTelemetryRecorder(AttackOverlapTelemetryRecorderConfig{}); recorder != nil {
		t.Fatalf("expected nil recorder when publisher missing")
	}
}
