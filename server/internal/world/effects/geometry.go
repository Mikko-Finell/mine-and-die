package effects

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func QuantizeWorldCoord(value float64, tileSize float64) int {
	if tileSize <= 0 {
		tileSize = 40
	}
	return int(math.Round((value / tileSize) * effectcontract.CoordScale))
}

func DequantizeWorldCoord(value int, tileSize float64) float64 {
	if tileSize <= 0 {
		tileSize = 40
	}
	return (float64(value) / effectcontract.CoordScale) * tileSize
}

func TicksToDuration(ticks int, tickRate int) time.Duration {
	if ticks <= 0 || tickRate <= 0 {
		return 0
	}
	seconds := float64(ticks) / float64(tickRate)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}
