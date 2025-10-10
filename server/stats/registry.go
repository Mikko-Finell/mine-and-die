package stats

// Archetype identifies the default stat seed used to initialise a component.
type Archetype uint8

const (
	ArchetypePlayer Archetype = iota
	ArchetypeGoblin
	ArchetypeRat
)

var archetypeBase = map[Archetype]ValueSet{
	ArchetypePlayer: {
		StatMight:     20,
		StatResonance: 16,
		StatFocus:     12,
		StatSpeed:     11,
	},
	ArchetypeGoblin: {
		StatMight:     12,
		StatResonance: 6,
		StatFocus:     7,
		StatSpeed:     8,
	},
	ArchetypeRat: {
		StatMight:     3.6,
		StatResonance: 2.4,
		StatFocus:     3,
		StatSpeed:     6,
	},
}

// DefaultBase returns a copy of the base values for the given archetype.
func DefaultBase(archetype Archetype) ValueSet {
	base := archetypeBase[archetype]
	return base
}

// DefaultComponent constructs and resolves a component using the archetype defaults.
func DefaultComponent(archetype Archetype) Component {
	comp := NewComponent(DefaultBase(archetype))
	comp.Resolve(0)
	return comp
}

// DefaultDerived returns the resolved derived stats for the given archetype.
func DefaultDerived(archetype Archetype) DerivedSet {
	comp := DefaultComponent(archetype)
	return comp.DerivedValues()
}

// DefaultMaxHealth returns the resolved max health for the given archetype.
func DefaultMaxHealth(archetype Archetype) float64 {
	derived := DefaultDerived(archetype)
	return derived[DerivedMaxHealth]
}

// Formula tuning values. These constants are intentionally simple to keep
// milestone-one behaviour predictable while leaving room for future balancing.
const (
	baseHealthFlat       = 0.0
	mightHealthScalar    = 5.0
	baseManaFlat         = 45.0
	resonanceManaScalar  = 3.5
	baseAccuracy         = 0.75
	focusAccuracyScalar  = 0.006
	baseEvasion          = 0.05
	speedEvasionScalar   = 0.005
	castSpeedScalar      = 0.008
	cooldownRateScalar   = 0.006
	staggerBase          = 0.1
	staggerMightScalar   = 0.003
	damagePhysicalScalar = 0.12
	damageMagicalScalar  = 0.14
	decayRatio           = 0.94
)
