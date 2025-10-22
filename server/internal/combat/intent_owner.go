package combat

// AbilityActor captures the subset of actor metadata required to sanitize
// ability owners before constructing combat intents. It mirrors the legacy
// actor state fields without depending on the server package.
type AbilityActor struct {
	ID     string
	X      float64
	Y      float64
	Facing string
}

// NewMeleeIntentOwnerFromActor converts an ability actor snapshot into the
// typed melee intent owner used by the combat package.
func NewMeleeIntentOwnerFromActor(actor *AbilityActor) (MeleeIntentOwner, bool) {
	if actor == nil || actor.ID == "" {
		return MeleeIntentOwner{}, false
	}

	return MeleeIntentOwner{
		ID:     actor.ID,
		X:      actor.X,
		Y:      actor.Y,
		Facing: actor.Facing,
	}, true
}

// NewProjectileIntentOwnerFromActor converts an ability actor snapshot into the
// typed projectile intent owner used by the combat package.
func NewProjectileIntentOwnerFromActor(actor *AbilityActor) (ProjectileIntentOwner, bool) {
	if actor == nil {
		return ProjectileIntentOwner{}, false
	}

	return ProjectileIntentOwner{
		ID:     actor.ID,
		X:      actor.X,
		Y:      actor.Y,
		Facing: actor.Facing,
	}, true
}
