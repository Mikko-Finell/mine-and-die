package stats

import "math"

func computeDerived(total ValueSet) DerivedSet {
	var derived DerivedSet

	might := clamp(total[StatMight], 0, 1e9)
	resonance := clamp(total[StatResonance], 0, 1e9)
	focus := clamp(total[StatFocus], 0, 1e9)
	speed := clamp(total[StatSpeed], 0, 1e9)

	derived[DerivedMaxHealth] = computeMaxHealth(might)
	derived[DerivedMaxMana] = computeMaxMana(resonance)
	derived[DerivedDamageScalarPhysical] = computeDamageScalar(might, damagePhysicalScalar)
	derived[DerivedDamageScalarMagical] = computeDamageScalar(resonance, damageMagicalScalar)
	derived[DerivedAccuracy] = clamp(baseAccuracy+focus*focusAccuracyScalar, 0, 0.99)
	derived[DerivedEvasion] = clamp(baseEvasion+speed*speedEvasionScalar, 0, 0.9)
	derived[DerivedCastSpeed] = clamp(1+focus*castSpeedScalar, 0.1, 5)
	derived[DerivedCooldownRate] = clamp(1+speed*cooldownRateScalar, 0.1, 5)
	derived[DerivedStaggerResist] = clamp(staggerBase+might*staggerMightScalar, 0, 1)

	return derived
}

func computeMaxHealth(might float64) float64 {
	return baseHealthFlat + might*mightHealthScalar
}

func computeMaxMana(resonance float64) float64 {
	return baseManaFlat + resonance*resonanceManaScalar
}

func computeDamageScalar(attribute float64, coeff float64) float64 {
	scaled := 1 + coeff*(1-math.Pow(decayRatio, attribute))
	return clamp(scaled, 0.1, 10)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
