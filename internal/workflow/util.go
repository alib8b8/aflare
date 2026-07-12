package workflow

import (
	"math/rand"
	"sync"
	"time"
)

var (
	rngMu  sync.Mutex
	rngSrc = rand.NewSource(time.Now().UnixNano())
	rng    = rand.New(rngSrc)
)

// pseudoRand returns a float64 in [0, 1) for jitter calculations.
func pseudoRand() float64 {
	rngMu.Lock()
	defer rngMu.Unlock()
	return rng.Float64()
}
