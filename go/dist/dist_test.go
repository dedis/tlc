package dist

import (
	"fmt"
	"testing"
	"math/rand"
)

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, threshold, nnodes, maxSteps, maxTicket int) {
	desc := fmt.Sprintf("T=%v,N=%v,Steps=%v,Tickets=%v",
		threshold, nnodes, maxSteps, maxTicket)
	t.Run(desc, func(t *testing.T) {
		Threshold = threshold
		All = make([]*Node, nnodes)
		MaxSteps = maxSteps

		for i := range All { // Initialize all the nodes
			All[i] = newNode(i)
			if maxTicket > 0 {
				All[i].Rand = func() int64 {
					return rand.Int63n(int64(maxTicket))
				}
			}
		}
		for _, n := range All { // Run the nodes on separate goroutines
			go n.run()
		}
		for _, n := range All { // Wait for each to complete the test
			<-n.done
		}
		testResults(t) // Report test results
	})
}

// Dump the consensus state of node n in round s
func (n *Node) testDump(t *testing.T, s int) {
	r := &n.qsc[s]
	t.Errorf("%v %v conf %v %v %v re %v %v %v spoil %v %v %v", n.from, s,
		r.conf.from, int(r.conf.tkt)/len(All), int(r.conf.tkt)%len(All),
		r.reconf.from, int(r.reconf.tkt)/len(All), int(r.reconf.tkt)%len(All),
		r.spoil.from, int(r.spoil.tkt)/len(All), int(r.spoil.tkt)%len(All))
}

// Globally sanity-check and summarize each node's observed results.
func testResults(t *testing.T) {
	for i, n := range All {
		commits := 0
		for s := range n.qsc {
			if n.qsc[s].commit {
				commits++
				for _, nn := range All { // verify consensus
					if nn.qsc[s].conf.from != n.qsc[s].conf.from {
						t.Errorf("%v %v UNSAFE", i, s)
						for _, nnn := range All {
							nnn.testDump(t, s)
						}
					}
				}
			}
		}
		t.Logf("node %v committed %v of %v (%v%% success rate)",
			i, commits, len(n.qsc), (commits*100)/len(n.qsc))
	}
}

// Run QSC consensus for a variety of test cases.
func TestQSC(t *testing.T) {
	testRun(t, 1, 1, 10000, 0) // Trivial case: 1 of 1 consensus!
	testRun(t, 2, 2, 10000, 0) // Another trivial case: 2 of 2

	testRun(t, 2, 3, 10000, 0) // Standard f=1 case
	testRun(t, 3, 5, 10000, 0) // Standard f=2 case
	testRun(t, 4, 7, 1000, 0)  // Standard f=3 case
	testRun(t, 5, 9, 1000, 0)  // Standard f=4 case
	testRun(t, 11, 21, 100, 0) // Standard f=10 case

	testRun(t, 3, 3, 10000, 0) // Larger-than-minimum thresholds
	testRun(t, 6, 7, 1000, 0)
	testRun(t, 9, 10, 1000, 0)

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	testRun(t, 2, 3, 10000, 1) // Limit case: will never commit
	testRun(t, 2, 3, 10000, 2) // Extreme low-entropy: rarely commits
	testRun(t, 2, 3, 10000, 3) // A bit better bit still bad...
}
