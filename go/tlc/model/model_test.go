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


	for i := 1; i < 10; i++ {
	testRun(t, 1, 1, 1000, 0)	// Trivial case: 1 of 1 consensus!
	testRun(t, 2, 2, 1000, 0)	// Another trivial case: 2 of 2
	testRun(t, 2, 3, 1000, 0)	// Basic realistic case: 2 of 3
	testRun(t, 2, 3, 1000, 0)	// Basic realistic case: 2 of 3
	testRun(t, 3, 5, 1000, 0)	// Standard f=2 case
	testRun(t, 4, 7, 1000, 0)	// Standard f=3 case
	testRun(t, 6, 7, 1000, 0)	// Larger-than-necessary threshold
	testRun(t, 5, 9, 1000, 0)	// Standard f=4 case
	testRun(t, 11, 21, 100, 0)	// Standard f=10 case
	testRun(t, 101, 201, 10, 0)	// Standard f=100 case
	}
}

