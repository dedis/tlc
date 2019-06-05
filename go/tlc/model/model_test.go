package model

import (
	"testing"
	"time"
)


func testRun(t *testing.T, thres, nodes, maxSteps int, maxSleep time.Duration) {

	t.Logf("QSC test: T=%v N=%v Steps=%v Sleep=%v",
		thres, nodes, maxSteps, maxSleep)

	MaxSteps = maxSteps
	MaxSleep = maxSleep
	Run(thres, nodes)
}

func TestQSC(t *testing.T) {

	// Trivial case: 1 of 1 consensus!
	testRun(t, 1, 1, 100, 0)

	testRun(t, 2, 3, 100, 0)
}

