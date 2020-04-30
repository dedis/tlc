// Package test contains shareable code for testing instantiations of QSCOD.
package test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	. "github.com/dedis/tlc/go/model/qscod/core"
)

// Object to record the common total order and verify it for consistency
type testOrder struct {
	hs  []Head     // all history known to be committed so far
	mut sync.Mutex // mutex protecting this reference order
}

// When a client reports a history h has been committed,
// record that in the testOrder and check it for global consistency.
func (to *testOrder) committed(t *testing.T, h Head) {
	to.mut.Lock()
	defer to.mut.Unlock()

	// Ensure history slice is long enough
	for h.Step >= Step(len(to.hs)) {
		to.hs = append(to.hs, Head{})
	}

	// Check commit consistency across all concurrent clients
	switch {
	case to.hs[h.Step] == Head{}:
		to.hs[h.Step] = h
	case to.hs[h.Step] != h:
		t.Errorf("UNSAFE at %v:\n%+v\n%+v", h.Step, h, to.hs[h.Step])
	}
}

// testCli creates a test client with particular configuration parameters.
func testCli(t *testing.T, self, f, maxstep, maxpri int,
	kv []Store, to *testOrder, wg *sync.WaitGroup) {

	// Create a cancelable context for the test run
	ctx, cancel := context.WithCancel(context.Background())

	// Use the simple Int63n for random number generation,
	// with values constrained to be lower than maxpri for testing.
	// A real deployment should use cryptographic randomness
	// and should preferably be high-entropy, close to the full 64 bits.
	rv := func() int64 { return rand.Int63n(int64(maxpri)) }

	// Our proposal function simply collects and consistency-checks
	// committed Heads until a designated time-step is reached.
	pr := func(L, C Head) string {
		//fmt.Printf("cli %v saw commit %v %q\n", self, C.Step, C.Data)

		// Consistency-check the history h known to be committed
		to.committed(t, C)

		// Stop once we reach the step limit
		if C.Step >= Step(maxstep) {
			cancel()
		}

		return fmt.Sprintf("cli %v proposal %v", self, C.Step)
	}

	// Start the test client with appropriate parameters assuming
	// n=3f, tr=2f, tb=f, and ts=f+1, satisfying TLCB's constraints.
	c := Client{KV: kv, Tr: 2 * f, Ts: f + 1, Pr: pr, RV: rv}
	c.Run(ctx)

	wg.Done()
}

// Run a consensus test case on a given set of Store interfaces
// and with the specified group configuration and test parameters.
func TestRun(t *testing.T, kv []Store, nfail, ncli, maxstep, maxpri int) {

	// Create a reference total order for safety checking
	to := &testOrder{}

	desc := fmt.Sprintf("F=%v,N=%v,Clients=%v,Commits=%v,Tickets=%v",
		nfail, len(kv), ncli, maxstep, maxpri)
	t.Run(desc, func(t *testing.T) {

		// Simulate the appropriate number of concurrent clients
		wg := &sync.WaitGroup{}
		for i := 0; i < ncli; i++ {
			wg.Add(1)
			go testCli(t, i, nfail, maxstep, maxpri, kv, to, wg)
		}
		wg.Wait()
	})
}
