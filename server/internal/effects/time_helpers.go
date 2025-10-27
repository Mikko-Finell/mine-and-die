package effects

import (
	"math"
	"time"
)

func durationToTicks(duration time.Duration, tickRate int) int {
	if duration <= 0 || tickRate <= 0 {
		return 0
	}
	ticks := int(math.Ceil(duration.Seconds() * float64(tickRate)))
	if ticks < 1 {
		ticks = 1
	}
	return ticks
}
