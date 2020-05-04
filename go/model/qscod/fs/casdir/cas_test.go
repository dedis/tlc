// Package casdir tests CAS-based QSCOD over a set of file system CAS stores.
package casdir

import (
	"context"
	"fmt"
	"os"
	"testing"

	. "github.com/dedis/tlc/go/lib/cas"
	"github.com/dedis/tlc/go/lib/cas/test"
	"github.com/dedis/tlc/go/lib/fs/casdir"
	. "github.com/dedis/tlc/go/model/qscod/cas"
)

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, nclients, nthreads, naccesses int) {

	// Create a test key/value store representing each node
	dirs := make([]string, nnode)
	for i := range dirs {
		dirs[i] = fmt.Sprintf("test-store-%d", i)

		// Remove the test directory if one is left-over
		// from a previous test run.
		os.RemoveAll(dirs[i])

		// Create the test directory afresh.
		fs := &casdir.Store{}
		if err := fs.Init(dirs[i], true, true); err != nil {
			t.Fatal(err)
		}

		// Clean it up once the test is done.
		defer os.RemoveAll(dirs[i])
	}

	desc := fmt.Sprintf("F=%v,N=%v,Clients=%v,Threads=%v,Accesses=%v",
		nfail, nnode, nclients, nthreads, naccesses)
	t.Run(desc, func(t *testing.T) {

		// Create a context and cancel it at the end of the test
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create simulated clients to access the consensus group
		clients := make([]Store, nclients)
		for i := range clients {

			// Create a set of Store objects for each client
			members := make([]Store, nnode)
			for j := range members {
				fs := &casdir.Store{}
				err := fs.Init(dirs[j], false, false)
				if err != nil {
					t.Fatal(err)
				}
				members[j] = fs
			}

			clients[i] = (&Group{}).Start(ctx, members, nfail)
		}

		// Run a standard torture test across all the clients
		test.Stores(t, nthreads, naccesses, clients...)
	})
}

func TestConsensus(t *testing.T) {
	testRun(t, 1, 3, 1, 10, 10) // Standard f=1 case,
	testRun(t, 1, 3, 2, 10, 10) // varying number of clients
	testRun(t, 1, 3, 10, 10, 10)
	testRun(t, 1, 3, 20, 10, 10)

	testRun(t, 2, 6, 10, 10, 10) // Standard f=2 case
	testRun(t, 3, 9, 10, 10, 10) // Standard f=3 case

	// Note: when nnode * nclients gets to be around 120-ish,
	// we start running into default max-open-file limits.
}
