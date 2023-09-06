package transfer

import (
	"sync"
	"time"
)

// datanodeFailures is a global map of address to the last recorded failure
var datanodeFailures = make(map[string]time.Time)
var datanodeFailuresLock sync.Mutex

// a datanodeFailover provides some common code for trying multiple datanodes
// in the context of a single operation on a single block.
type datanodeFailover struct {
	datanodes       []string
	currentDatanode string
	err             error
}

func newDatanodeFailover(datanodes []string) *datanodeFailover {
	return &datanodeFailover{
		datanodes:       datanodes,
		currentDatanode: "",
		err:             nil,
	}
}

func (df *datanodeFailover) recordFailure(err error) {
	datanodeFailuresLock.Lock()
	defer datanodeFailuresLock.Unlock()

	datanodeFailures[df.currentDatanode] = time.Now()
	df.err = err
}

func (df *datanodeFailover) next() string {
	if df.numRemaining() == 0 {
		return ""
	}

	var picked = -1
	var oldestFailure time.Time

	for i, address := range df.datanodes {
		datanodeFailuresLock.Lock()
		failedAt, hasFailed := datanodeFailures[address]
		datanodeFailuresLock.Unlock()

		if !hasFailed {
			picked = i
			break
		} else if oldestFailure.IsZero() || failedAt.Before(oldestFailure) {
			picked = i
			oldestFailure = failedAt
		}
	}

	address := df.datanodes[picked]
	df.datanodes = append(df.datanodes[:picked], df.datanodes[picked+1:]...)

	df.currentDatanode = address
	return address
}

func (df *datanodeFailover) numRemaining() int {
	return len(df.datanodes)
}

func (df *datanodeFailover) lastError() error {
	return df.err
}
