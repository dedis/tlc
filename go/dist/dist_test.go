package dist

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

type testNode struct {
	n *Node         // Node state
	c chan *Message // Channel to communicate with this testNode
}

func (tn *testNode) Send(msg *Message) {
	tn.c <- msg
}

func (tn *testNode) run(maxSteps int, wg *sync.WaitGroup) {

	// broadcast message for initial time step s=0
	tn.n.advanceTLC(0)

	// run the required number of time steps for the test
	for tn.n.step < maxSteps {
		msg := <-tn.c        // Receive a msg
		tn.n.receiveTLC(msg) // Process it
	}

	// signal that we're done
	wg.Done()
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, threshold, nnodes, maxSteps int, maxTicket int64) {
	desc := fmt.Sprintf("T=%v,N=%v,Steps=%v,Tickets=%v",
		threshold, nnodes, maxSteps, maxTicket)
	t.Run(desc, func(t *testing.T) {

		// Initialize all the nodes
		tn := make([]testNode, nnodes)
		peer := make([]Peer, nnodes)
		for i := range tn {
			tn[i].n = NewNode(i, threshold, peer)
			tn[i].c = make(chan *Message, 3*nnodes*maxSteps)
			if maxTicket > 0 {
				tn[i].n.Rand = func() uint64 {
					return uint64(rand.Int63n(maxTicket))
				}
			}
			peer[i] = &tn[i]
		}

		// Run the nodes on separate goroutines
		wg := &sync.WaitGroup{}
		for i := range tn {
			wg.Add(1)
			go tn[i].run(maxSteps, wg)
		}
		wg.Wait()
		testResults(t, tn) // Report test results
	})
}

// Dump the consensus state of node n in round s
func (tn *testNode) testDump(t *testing.T, s, nn int) {
	r := &tn.n.qsc[s]
	t.Errorf("%v %v conf %v %v %v re %v %v %v spoil %v %v %v", tn.n.from, s,
		r.conf.from, int(r.conf.tkt)/nn, int(r.conf.tkt)%nn,
		r.reconf.from, int(r.reconf.tkt)/nn, int(r.reconf.tkt)%nn,
		r.spoil.from, int(r.spoil.tkt)/nn, int(r.spoil.tkt)%nn)
}

// Globally sanity-check and summarize each node's observed results.
func testResults(t *testing.T, tn []testNode) {
	for i, ti := range tn {
		commits := 0
		for s := range ti.n.qsc {
			if ti.n.qsc[s].commit {
				commits++
				for _, tj := range tn { // verify consensus
					if tj.n.qsc[s].conf.from != ti.n.qsc[s].conf.from {
						t.Errorf("%v %v UNSAFE", i, s)
						for _, tk := range tn {
							tk.testDump(t, s, len(tn))
						}
					}
				}
			}
		}
		t.Logf("node %v committed %v of %v (%v%% success rate)",
			i, commits, len(ti.n.qsc), (commits*100)/len(ti.n.qsc))
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
