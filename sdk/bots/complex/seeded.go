package complex

import (
	"github.com/rs/zerolog"
	rand "math/rand/v2"
)

// NewSeededHandler preserves the built-in strategy while avoiding environment
// variables and wall-clock seeds in independent, parallel simulation hands.
func NewSeededHandler(seed uint64) *Handler {
	return &complexBot{id: "complex", logger: zerolog.Nop(), rng: rand.New(rand.NewPCG(seed, 0)), bigBlind: 10, strategy: defaultStrategy}
}
