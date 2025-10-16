package contract

// InstanceSpawnPayload mirrors the authoritative EffectInstance state broadcast
// when an effect spawns. Individual contract definitions alias this type until
// specialised payloads are authored.
type InstanceSpawnPayload struct {
	ContractPayload
	Instance EffectInstance `json:"instance"`
}

// InstanceUpdatePayload captures the mutable fields emitted for an in-flight
// effect instance. Contract definitions alias this type so the generator has a
// concrete struct to examine.
type InstanceUpdatePayload struct {
	ContractPayload
	ID            string               `json:"id"`
	DeliveryState *EffectDeliveryState `json:"deliveryState,omitempty"`
	BehaviorState *EffectBehaviorState `json:"behaviorState,omitempty"`
	Params        map[string]int       `json:"params,omitempty"`
}

// InstanceEndPayload is the canonical end payload shared by all current effect
// contracts.
type InstanceEndPayload struct {
	ContractPayload
	ID     string    `json:"id"`
	Reason EndReason `json:"reason"`
}

// AttackSpawnPayload is currently identical to InstanceSpawnPayload but defined
// separately so future iterations can specialise it without breaking the
// registry shape.
type AttackSpawnPayload = InstanceSpawnPayload

// AttackUpdatePayload mirrors the update payload emitted for attack effects.
type AttackUpdatePayload = InstanceUpdatePayload

// AttackEndPayload mirrors the end payload emitted for attack effects.
type AttackEndPayload = InstanceEndPayload

// FireballSpawnPayload exposes the spawn payload for projectile fireballs.
type FireballSpawnPayload = InstanceSpawnPayload

// FireballUpdatePayload captures fireball update payloads.
type FireballUpdatePayload = InstanceUpdatePayload

// FireballEndPayload captures fireball end payloads.
type FireballEndPayload = InstanceEndPayload

// BurningTickSpawnPayload represents the spawn payload for burning damage ticks.
type BurningTickSpawnPayload = InstanceSpawnPayload

// BurningTickUpdatePayload captures burning tick updates.
type BurningTickUpdatePayload = InstanceUpdatePayload

// BurningTickEndPayload captures burning tick end payloads.
type BurningTickEndPayload = InstanceEndPayload

// BurningVisualSpawnPayload represents the spawn payload for burning visuals.
type BurningVisualSpawnPayload = InstanceSpawnPayload

// BurningVisualUpdatePayload captures burning visual updates.
type BurningVisualUpdatePayload = InstanceUpdatePayload

// BurningVisualEndPayload captures burning visual end payloads.
type BurningVisualEndPayload = InstanceEndPayload

// BloodSplatterSpawnPayload represents the spawn payload for blood splatters.
type BloodSplatterSpawnPayload = InstanceSpawnPayload

// BloodSplatterUpdatePayload mirrors blood splatter updates.
type BloodSplatterUpdatePayload = InstanceUpdatePayload

// BloodSplatterEndPayload mirrors blood splatter end payloads.
type BloodSplatterEndPayload = InstanceEndPayload
