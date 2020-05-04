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
	hist []string   // all history known to be committed so far
	mut  sync.Mutex // mutex protecting this reference order
}

// When a client reports a history h has been committed,
// record that in the testOrder and check it for global consistency.
func (to *testOrder) committed(t *testing.T, step int64, prop string) {
	to.mut.Lock()
	defer to.mut.Unlock()

	// Ensure history slice is long enough
	for step >= int64(len(to.hist)) {
		to.hist = append(to.hist, "")
	}

	// Check commit consistency across all concurrent clients
	switch {
	case to.hist[step] == "":
		to.hist[step] = prop
	case to.hist[step] != prop:
		t.Errorf("Inconsistency at %v:\n old %q\n new %q",
			step, to.hist[step], prop)
	}
}

// testCli creates a test client with particular configuration parameters.
func testCli(t *testing.T, self, f, maxstep, maxpri int,
	kv []Store, to *testOrder, wg *sync.WaitGroup) {

	// Create a cancelable context for the test run
	ctx, cancel := context.WithCancel(context.Background())

	// Our proposal function simply collects and consistency-checks
	// committed Heads until a designated time-step is reached.
	pr := func(step int64, cur string, com bool) (string, int64) {
		//fmt.Printf("cli %v saw commit %v %q\n", self, C.Step, C.Data)

		// Consistency-check the history h known to be committed
		if com {
			to.committed(t, step, cur)
		}

		// Stop once we reach the step limit
		if step >= int64(maxstep) {
			cancel()
		}

		// Use the simple Int63n for random number generation,
		// with values constrained to be lower than maxpri for testing.
		// A real deployment should use cryptographic randomness
		// and should preferably be high-entropy,
		// close to the full 64 bits.
		pri := rand.Int63n(int64(maxpri))

		return fmt.Sprintf("cli %v proposal %v", self, step), pri
	}

	// Start the test client with appropriate parameters assuming
	// n=3f, tr=2f, tb=f, and ts=f+1, satisfying TLCB's constraints.
	c := Client{KV: kv, Tr: 2 * f, Ts: f + 1, Pr: pr}
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
