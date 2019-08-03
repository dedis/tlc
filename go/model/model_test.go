package model

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func (n *Node) run(maxSteps int, peer []chan *Message, wg *sync.WaitGroup) {

	// broadcast message for initial time step s=0
	n.advanceTLC(0) // broadcast message for initial time step

	// run the required number of time steps for the test
	for n.Step < maxSteps {
		msg := <-peer[n.From] // Receive a message
		n.receiveTLC(msg)     // Process it
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
		all := make([]*Node, nnodes)
		peer := make([]chan *Message, nnodes)
		send := func(i int, m *Message) { peer[i] <- m }

		for i := range all { // Initialize all the nodes
			peer[i] = make(chan *Message, 3*nnodes*maxSteps)
			all[i] = NewNode(i, threshold, nnodes, send)
			if maxTicket > 0 {
				all[i].Rand = func() int64 {
					return rand.Int63n(int64(maxTicket))
				}
			}
		}
		wg := &sync.WaitGroup{}
		for _, n := range all { // Run the nodes on separate goroutines
			wg.Add(1)
			go n.run(maxSteps, peer, wg)
		}
		wg.Wait()
		testResults(t, all) // Report test results
	})
}

// Dump the consensus state of node n in round s
func (n *Node) testDump(t *testing.T, s, nnodes int) {
	r := &n.QSC[s]
	t.Errorf("%v %v conf %v %v re %v %v spoil %v %v",
		n.From, s, r.Conf.From, r.Conf.Tkt,
		r.Reconf.From, r.Reconf.Tkt, r.Spoil.From, r.Spoil.Tkt)
}

// Globally sanity-check and summarize each node's observed results.
func testResults(t *testing.T, all []*Node) {
	for i, n := range all {
		commits := 0
		for s := range n.QSC {
			if n.QSC[s].Commit {
				commits++
				for _, nn := range all { // verify consensus
					if nn.QSC[s].Conf.From != n.QSC[s].Conf.From {
						t.Errorf("%v %v UNSAFE", i, s)
						for _, nnn := range all {
							nnn.testDump(t, s, len(all))
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
