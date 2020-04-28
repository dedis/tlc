package simple

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"

	. "github.com/dedis/tlc/go/model/qscod"
)

// It sucks that Go doesn't allow packages to import public test code
// from other packages - otherwise most of the code below needn't be duplicated
// but could just be exported and reused from qscod/cli_test.go.

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

	// Use the simple Int63n for random number generation,
	// with values constrained to be lower than maxpri for testing.
	// A real deployment should use cryptographic randomness
	// and should preferably be high-entropy, close to the full 64 bits.
	rv := func() int64 { return rand.Int63n(int64(maxpri)) }

	// Our update function simply collects and consistency-checks
	// committed Heads until a designated time-step is reached.
	up := func(h Head) (prop string, err error) {
		//fmt.Printf("cli %v saw commit %v %q\n", self, h.Step, h.Data)

		// Consistency-check the history h known to be committed
		to.committed(t, h)

		// Stop once we reach the step limit
		if h.Step >= Step(maxstep) {
			return "", errors.New("Done")
		}

		prop = fmt.Sprintf("cli %v proposal %v", self, h.Step)
		return prop, nil
	}

	// Start the test client with appropriate parameters assuming
	// n=3f, tr=2f, tb=f, and ts=f+1, satisfying TLCB's constraints.
	c := Client{KV: kv, Tr: 2 * f, Ts: f + 1, Up: up, RV: rv}
	c.Run()

	wg.Done()
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, ncli, maxstep, maxpri int) {

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
