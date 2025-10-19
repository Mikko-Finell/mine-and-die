package sim

import (
	"log"
	"math/rand"

	"mine-and-die/server/logging"
)

// Deps carries shared infrastructure dependencies required by the simulation engine.
type Deps struct {
	Logger  *log.Logger
	Metrics *logging.Metrics
	Clock   logging.Clock
	RNG     *rand.Rand
}
