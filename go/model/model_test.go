package model

import (
	"fmt"
	"sync"
	"testing"
)

func (n *Node) run(wg *sync.WaitGroup) {

	// broadcast message for initial time step s=0
	n.advanceTLC(0) // broadcast message for initial time step

	// run the required number of time steps for the test
	for MaxSteps == 0 || n.Step < MaxSteps {
		msg := <-n.comm   // Receive a message
		n.receiveTLC(msg) // Process it
	}

	// signal that we're done
	wg.Done()
}

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
		wg := &sync.WaitGroup{}
		for _, n := range All { // Run the nodes on separate goroutines
			wg.Add(1)
			go n.run(wg)
		}
		wg.Wait()
		testResults(t) // Report test results
	})
}

// Dump the consensus state of node n in round s
func (n *Node) testDump(t *testing.T, s int) {
	r := &n.QSC[s]
	t.Errorf("%v %v conf %v %v %v re %v %v %v spoil %v %v %v", n.From, s,
		r.Conf.From, r.Conf.Tkt/len(All), r.Conf.Tkt%len(All),
		r.Reconf.From, r.Reconf.Tkt/len(All), r.Reconf.Tkt%len(All),
		r.Spoil.From, r.Spoil.Tkt/len(All), r.Spoil.Tkt%len(All))
}

// Globally sanity-check and summarize each node's observed results.
func testResults(t *testing.T) {
	for i, n := range All {
		commits := 0
		for s := range n.QSC {
			if n.QSC[s].Commit {
				commits++
				for _, nn := range All { // verify consensus
					if nn.QSC[s].Conf.From != n.QSC[s].Conf.From {
						t.Errorf("%v %v UNSAFE", i, s)
						for _, nnn := range All {
							nnn.testDump(t, s)
						}
					}
				}
			}
		}
		t.Logf("node %v committed %v of %v (%v%% success rate)",
			i, commits, len(n.QSC), (commits*100)/len(n.QSC))
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
