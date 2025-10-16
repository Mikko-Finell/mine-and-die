package contract

// Hook identifiers used by the built-in effect definitions. These values are
// part of the authoritative contract consumed by the runtime and client
// generator, so they live alongside the other contract metadata.
const (
	HookMeleeSpawn          = "melee.spawn"
	HookProjectileLifecycle = "projectile.fireball.lifecycle"
	HookStatusBurningVisual = "status.burning.visual"
	HookStatusBurningDamage = "status.burning.tick"
	HookVisualBloodSplatter = "visual.blood.splatter"
)
