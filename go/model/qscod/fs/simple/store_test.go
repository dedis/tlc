package simple

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"

	. "github.com/dedis/tlc/go/model/qscod"
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
func testCli(t *testing.T, self, nfail, ncom, maxpri int,
	kv []Store, to *testOrder, wg *sync.WaitGroup) {

	rv := func() int64 { return rand.Int63n(int64(maxpri)) }

	// Commit ncom messages, and consistency-check each commitment.
	h := Head{}
	for i := 0; i < ncom; i++ {

		// Start the test client with appropriate parameters assuming
		// n=3f, tr=2f, tb=f, and ts=f+1, satisfying TLCB's constraints.
		pref := fmt.Sprintf("cli %v commit %v", self, i)
		h = Commit(2*nfail, nfail+1, kv, rv, pref, h)
		//println("thread", self, "got commit", h.Step, h.Data)

		to.committed(t, h) // consistency-check history h
	}
	wg.Done()
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, ncli, ncommits, maxpri int) {
	desc := fmt.Sprintf("F=%v,N=%v,Clients=%v,Commits=%v,Tickets=%v",
		nfail, nnode, ncli, ncommits, maxpri)
	t.Run(desc, func(t *testing.T) {

		// Create a test key/value store representing each node
		kv := make([]Store, nnode)
		for i := range kv {
			path := fmt.Sprintf("test-store-%d", i)
			ss := &FileStore{path}
			kv[i] = ss

			// Remove the test directory if one is left-over
			// from a previous test run.
			os.RemoveAll(path)

			// Create the test directory afresh.
			if err := os.Mkdir(path, 0744); err != nil {
				t.Fatal(err)
			}

			// Clean it up once the test is done.
			defer os.RemoveAll(path)
		}

		// Create a reference total order for safety checking
		to := &testOrder{}

		// Simulate the appropriate number of concurrent clients
		wg := &sync.WaitGroup{}
		for i := 0; i < ncli; i++ {
			wg.Add(1)
			go testCli(t, i, nfail, ncommits, maxpri, kv, to, wg)
		}
		wg.Wait()
	})
}

func TestSimpleStore(t *testing.T) {
	testRun(t, 1, 3, 1, 10, 100) // Standard f=1 case,
	testRun(t, 1, 3, 2, 10, 100) // varying number of clients
	testRun(t, 1, 3, 10, 3, 100)
	testRun(t, 1, 3, 20, 2, 100)
	testRun(t, 1, 3, 40, 2, 100)

	testRun(t, 2, 6, 10, 5, 100)  // Standard f=2 case
	testRun(t, 3, 9, 10, 3, 100)  // Standard f=3 case
	testRun(t, 4, 12, 10, 2, 100) // Standard f=4 case
	testRun(t, 5, 15, 10, 2, 100) // Standard f=10 case
}
