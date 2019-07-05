package model

import (
	"fmt"
	"testing"
)

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, threshold, nnodes, maxSteps, maxTicket int) {
	if maxTicket == 0 { // Default to moderate-entropy tickets
		maxTicket = 10 * nnodes
	}
	desc := fmt.Sprintf("T=%v,N=%v,Steps=%v,Tickets=%v",
		threshold, nnodes, maxSteps, maxTicket)
	t.Run(desc, func(t *testing.T) {
		Threshold = threshold
		All = make([]*Node, nnodes)
		MaxSteps = maxSteps
		MaxTicket = int32(maxTicket)

		for i := range All { // Initialize all the nodes
			All[i] = newNode(i)
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
	t.Errorf("%v %v conf %v (%v) reconf %v (%v) spoil %v (%v)", n.from, s,
		n.qsc[s].conf.from, n.qsc[s].conf.pri / len(All),
		n.qsc[s].reconf.from, n.qsc[s].reconf.pri / len(All),
		n.qsc[s].spoil.from, n.qsc[s].spoil.pri / len(All))
}

// Globally sanity-check and summarize each node's observed results.
func testResults(t *testing.T) {
	for i, n := range All {
		commits := 0
		if n.from != i { panic("XXX") }
		for s, committed := range n.commit {
			if n.choice[s] != n.qsc[s].conf.from { panic("XXX") }
			if committed {
				commits++
				for _, nn := range All { // verify consensus
					if nn.choice[s] != n.choice[s] {
						t.Errorf("%v %v UNSAFE COMMIT",
							i, s)
						for _, nnn := range All {
							nnn.testDump(t, s)
						}
					}
				}
			}
		}
		t.Logf("node %v committed %v of %v (%v%% success rate)",
			i, commits, len(n.commit), (commits*100)/len(n.commit))
	}
}

// Run QSC consensus for a variety of test cases.
func TestQSC(t *testing.T) {
	testRun(t, 1, 1, 10000, 0) // Trivial case: 1 of 1 consensus!
	testRun(t, 2, 2, 10000, 0) // Another trivial case: 2 of 2

	testRun(t, 2, 3, 10000, 0) // Standard f=1 case
	testRun(t, 3, 5, 1000, 0)  // Standard f=2 case
	testRun(t, 4, 7, 1000, 0)  // Standard f=3 case
	testRun(t, 5, 9, 1000, 0)  // Standard f=4 case
	testRun(t, 11, 21, 20, 0)  // Standard f=10 case

	testRun(t, 3, 3, 1000, 0) // Larger-than-minimum thresholds
	testRun(t, 6, 7, 1000, 0)
	testRun(t, 9, 10, 100, 0)

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	testRun(t, 2, 3, 10000, 1) // Limit case: will never commit
	testRun(t, 2, 3, 10000, 2) // Extreme low-entropy: rarely commits
	testRun(t, 2, 3, 10000, 3) // A bit better bit still bad...
}
