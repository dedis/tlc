package qscod

import "fmt"
import "math/rand"
import "sync"
import "testing"

// Trivial intra-process key-value store implementation for testing
type testStore struct {
	mut  sync.Mutex   // synchronization for testStore state
	kv   map[Step]Val // the key/value map
	comh *Hist        // latest history known to be committed
}

// WriteRead implements the Store interface with a simple intra-process map.
func (ts *testStore) WriteRead(s Step, v Val) (Val, *Hist) {
	ts.mut.Lock()
	defer ts.mut.Unlock()
	if ts.comh != nil && s < ts.comh.step { // step s is too old?
		return v, ts.comh
	}
	if _, ok := ts.kv[s]; !ok { // no client wrote a value yet for s?
		ts.kv[s] = v // write-once
	}
	v = ts.kv[s] // Read the winning value in any case
	return v, nil
}

func (ts *testStore) Committed(comh *Hist) {
	if ts.comh == nil || ts.comh.step < comh.step {
		ts.comh = comh
	}
}

// Object to record the common total order and verify it for consistency
type testOrder struct {
	hs  []*Hist    // all history known to be committed so far
	mut sync.Mutex // mutex protecting this reference order
}

// When a client reports a history h has been committed,
// record that in the testOrder and check it for global consistency.
func (to *testOrder) committed(t *testing.T, h *Hist) {
	if int(h.step/4) >= len(to.hs) { // one new proposal every 4 steps
		if h.pred != nil {
			to.committed(t, h.pred) // first check h's predecessor
		}
		to.hs = append(to.hs, h)
	}
	if to.hs[h.step/4] != h {
		t.Errorf("%v UNSAFE %v != %v", h.step/4, h.msg,
			to.hs[h.step/4].msg)
	}
}

// testCli creates a test client with particular configuration parameters.
func testCli(t *testing.T, self, nfail, ncom, maxpri int,
	kv []Store, to *testOrder, wg *sync.WaitGroup) {

	rv := func() int64 { return rand.Int63n(int64(maxpri)) }

	// Commit ncom messages, and consistency-check each commitment.
	h := &Hist{} // Placeholder initial history
	for i := 0; i < ncom; i++ {

		// Start the test client with appropriate parameters assuming
		// n=3f, tr=2f, tb=f, and ts=f+1, satisfying TLCB's constraints.
		pref := fmt.Sprintf("cli %v commit %v", self, i)
		h = Commit(2*nfail, nfail+1, kv, rv, pref, h)
		//println("thread", self, "got commit", h.step, h.msg)

		to.mut.Lock()
		to.committed(t, h) // consistency-check history h
		to.mut.Unlock()
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
			kv[i] = &testStore{kv: make(map[Step]Val)}
		}

		// Create a reference total order for safety checking
		to := &testOrder{}

		// Simulate the appropriate number of concurrent clients
		cli := make([]client, ncli)
		wg := &sync.WaitGroup{}
		for i := range cli {
			wg.Add(1)
			go testCli(t, i, nfail, ncommits, maxpri, kv, to, wg)
		}
		wg.Wait()
	})
}

func TestClient(t *testing.T) {
	testRun(t, 1, 3, 1, 1000, 100) // Standard f=1 case
	testRun(t, 1, 3, 2, 1000, 100)
	testRun(t, 1, 3, 10, 1000, 100)
	testRun(t, 1, 3, 20, 1000, 100)
	testRun(t, 1, 3, 50, 1000, 100)
	testRun(t, 1, 3, 100, 1000, 100)

	testRun(t, 2, 6, 10, 1000, 100)  // Standard f=2 case
	testRun(t, 3, 9, 10, 1000, 100)  // Standard f=3 case
	testRun(t, 4, 12, 10, 1000, 100) // Standard f=4 case
	testRun(t, 5, 15, 10, 1000, 100) // Standard f=10 case

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	testRun(t, 1, 3, 10, 1000, 2) // Extreme low-entropy: rarely commits
	testRun(t, 1, 3, 10, 1000, 3) // A bit better bit still bad...
}
