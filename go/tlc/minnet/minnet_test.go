package minnet

import (
	"testing"
	"time"
	"fmt"
)


func testRun(t *testing.T, threshold, nnodes, maxSteps, maxTicket int,
		maxSleep time.Duration) {

	if maxTicket == 0 {		// Default to moderate-entropy tickets
		maxTicket = 10 * nnodes
	}

	desc := fmt.Sprintf("T=%v,N=%v,Steps=%v,Tickets=%v,Sleep=%v",
		threshold, nnodes, maxSteps, maxTicket, maxSleep)
	t.Run(desc, func(t *testing.T) {

		// Configure and run the test case.
		MaxSteps = maxSteps
		MaxTicket = int32(maxTicket)
		MaxSleep = maxSleep

		Run(threshold, nnodes)

		testResults(t)
	})
}

// Globally sanity-check and summarize each node's observed results
func testResults(t *testing.T) {
	for i, n := range All {
		commits := 0
		for s, committed := range n.commit {
			if committed {
				commits++
				for _, nn := range All {
					if nn.choice[s].From != n.choice[s].From {
						t.Fatalf("safety violation!" +
							"step %v", s)
					}
				}
			}
		}
		t.Logf("node %v committed %v of %v (%v%% success rate)",
			i, commits, len(n.commit), (commits*100)/len(n.commit))
	}
}

func TestQSC(t *testing.T) {

	testRun(t, 1, 1, 10000, 0, 0)	// Trivial case: 1 of 1 consensus!
	testRun(t, 2, 2, 10000, 0, 0)	// Another trivial case: 2 of 2

	testRun(t, 2, 3, 1000, 0, 0)	// Standard f=1 case
	testRun(t, 3, 5, 100, 0, 0)	// Standard f=2 case
	testRun(t, 4, 7, 100, 0, 0)	// Standard f=3 case
	testRun(t, 5, 9, 100, 0, 0)	// Standard f=4 case
	testRun(t, 11, 21, 20, 0, 0)	// Standard f=10 case
	//testRun(t, 101, 201, 10, 0, 0) // Standard f=100 case - blows up

	testRun(t, 3, 3, 1000, 0, 0)	// Larger-than-minimum thresholds
	testRun(t, 6, 7, 1000, 0, 0)
	testRun(t, 9, 10, 100, 0, 0)

	// Test with low-entropy tickets:
	// commit success rate will be bad, but still must remain safe!
	testRun(t, 2, 3, 1000, 1, 0)	// Limit case: will never commit
	testRun(t, 2, 3, 1000, 2, 0)	// Extreme low-entropy: rarely commits
	testRun(t, 2, 3, 1000, 3, 0)	// A bit better bit still bad...

	// Test with random delays inserted
	testRun(t, 2, 3, 1000, 0, 1 * time.Nanosecond)
	testRun(t, 2, 3, 1000, 0, 1 * time.Microsecond)
	testRun(t, 2, 3, 100, 0, 1 * time.Millisecond)
	testRun(t, 4, 7, 100, 0, 1 * time.Microsecond)
	testRun(t, 4, 7, 100, 0, 1 * time.Millisecond)
}

