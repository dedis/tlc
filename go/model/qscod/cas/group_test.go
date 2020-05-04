package cas

import (
	"context"
	"fmt"
	"testing"

	"github.com/dedis/tlc/go/lib/cas"
	"github.com/dedis/tlc/go/lib/cas/test"
)

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, nclients, nthreads, naccesses int) {

	desc := fmt.Sprintf("F=%v,N=%v,Clients=%v,Threads=%v,Accesses=%v",
		nfail, nnode, nclients, nthreads, naccesses)
	t.Run(desc, func(t *testing.T) {

		// Create a cancelable context for the test run
		ctx, cancel := context.WithCancel(context.Background())

		// Create an in-memory CAS register representing each node
		members := make([]cas.Store, nnode)
		memhist := make([]test.History, nnode)
		for i := range members {
			members[i] = &cas.Register{}
		}

		// Create a consensus group Store for each simulated client
		clients := make([]cas.Store, nclients)
		for i := range clients {

			// Interpose checking wrappers on the CAS registers
			checkers := make([]cas.Store, nnode)
			for i := range checkers {
				checkers[i] = test.Checked(t, &memhist[i],
					members[i])
			}

			clients[i] = (&Group{}).Start(ctx, checkers, nfail)
		}

		// Run a standard torture-test across all the clients
		test.Stores(t, nthreads, naccesses, clients...)

		// Shut down all the clients by canceling the context
		cancel()
	})
}

// Test the Client with a trivial in-memory key/value Store implementation.
func TestClient(t *testing.T) {
	testRun(t, 1, 3, 1, 1, 1000) // Standard f=1 case
	testRun(t, 1, 3, 2, 1, 1000)
	testRun(t, 1, 3, 10, 1, 1000)
	testRun(t, 1, 3, 20, 1, 100)
	testRun(t, 1, 3, 50, 1, 10)
	testRun(t, 1, 3, 100, 1, 10)

	testRun(t, 2, 6, 10, 10, 000)  // Standard f=2 case
	testRun(t, 3, 9, 10, 10, 100)  // Standard f=3 case
	testRun(t, 4, 12, 10, 10, 100) // Standard f=4 case
	testRun(t, 5, 15, 10, 10, 100) // Standard f=10 case

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	testRun(t, 1, 3, 10, 10, 1000) // Extreme low-entropy: rarely commits
	testRun(t, 1, 3, 10, 10, 1000) // A bit better bit still bad...
}
