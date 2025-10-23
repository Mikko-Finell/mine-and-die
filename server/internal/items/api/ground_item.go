package api

// GroundItem mirrors the legacy ground-item snapshot exposed to callers.
type GroundItem struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	FungibilityKey string  `json:"fungibility_key"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Qty            int     `json:"qty"`
}
