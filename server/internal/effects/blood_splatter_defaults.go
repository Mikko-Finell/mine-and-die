package effects

var bloodSplatterColorPalette = []string{"#7a0e12", "#4a090b"}

// NewBloodSplatterParams returns the default blood decal configuration values
// used when spawning contract-managed splatter effects. The map mirrors the
// legacy world defaults so callers can reuse the centralized configuration.
func NewBloodSplatterParams() map[string]float64 {
	return map[string]float64{
		"drag":           0.92,
		"dropletRadius":  3,
		"maxBursts":      0,
		"maxDroplets":    33,
		"maxStainRadius": 6,
		"maxStains":      140,
		"minDroplets":    4,
		"minStainRadius": 4,
		"spawnInterval":  1.1,
		"speed":          3,
	}
}

// BloodSplatterColors returns a copy of the default color palette for
// contract-managed blood decals so callers can safely reuse the shared
// configuration without mutating the canonical slice.
func BloodSplatterColors() []string {
	if len(bloodSplatterColorPalette) == 0 {
		return nil
	}
	colors := make([]string, len(bloodSplatterColorPalette))
	copy(colors, bloodSplatterColorPalette)
	return colors
}
