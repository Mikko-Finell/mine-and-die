package abilities

import "time"

const (
	// MeleeAttackCooldown mirrors the legacy swing rate between melee
	// triggers. The value matches combat.MeleeAttackCooldown without
	// importing the combat package to avoid cycles.
	MeleeAttackCooldown = 400 * time.Millisecond
	// FireballCooldown mirrors the legacy projectile cadence for fireball casts.
	FireballCooldown = 650 * time.Millisecond
)
