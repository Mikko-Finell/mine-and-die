package contract

// Effect identifiers registered by the Mine & Die server. These constants are
// exported so gameplay code can reference the canonical IDs instead of
// duplicating string literals.
const (
	EffectIDAttack        = "attack"
	EffectIDFireball      = "fireball"
	EffectIDBloodSplatter = "blood-splatter"
	EffectIDBurningTick   = "burning-tick"
	EffectIDBurningVisual = "fire"
)

// BuiltInRegistry enumerates the contract payload declarations for the existing
// gameplay effects. Callers should Validate the registry before indexing it.
var BuiltInRegistry = Registry{
	{
		ID:     EffectIDAttack,
		Spawn:  (*AttackSpawnPayload)(nil),
		Update: (*AttackUpdatePayload)(nil),
		End:    (*AttackEndPayload)(nil),
	},
	{
		ID:     EffectIDFireball,
		Spawn:  (*FireballSpawnPayload)(nil),
		Update: (*FireballUpdatePayload)(nil),
		End:    (*FireballEndPayload)(nil),
	},
	{
		ID:     EffectIDBurningTick,
		Spawn:  (*BurningTickSpawnPayload)(nil),
		Update: (*BurningTickUpdatePayload)(nil),
		End:    (*BurningTickEndPayload)(nil),
	},
	{
		ID:     EffectIDBurningVisual,
		Spawn:  (*BurningVisualSpawnPayload)(nil),
		Update: (*BurningVisualUpdatePayload)(nil),
		End:    (*BurningVisualEndPayload)(nil),
	},
	{
		ID:     EffectIDBloodSplatter,
		Spawn:  (*BloodSplatterSpawnPayload)(nil),
		Update: (*BloodSplatterUpdatePayload)(nil),
		End:    (*BloodSplatterEndPayload)(nil),
	},
}
