package qscod

import (
	"fmt"
	"sync"
	"testing"
	"math/rand"
)

// Trivial intra-process key-value store implementation for testing
type testStore struct {
	kv map[Step]Val
	mut sync.Mutex
}

// Object to record the common total order and verify it for consistency
type testOrder struct {
	hs []*Hist		// all history known to be committed so far
	mut sync.Mutex
}

func (to *testOrder) committed(t *testing.T, h *Hist) {
	if int(h.step/4) >= len(to.hs) {
		if h.pred != nil {
			to.committed(t, h.pred)	// first check h's predecessor
		}
		if int(h.step/4) != len(to.hs) {
			panic("out of sync")
		}
		to.hs = append(to.hs, h)
	}
	if to.hs[h.step/4] != h {
		t.Errorf("%v UNSAFE %v != %v", h.step/4, h.msg,
			to.hs[h.step/4].msg)
	}
}


func (ts *testStore) WriteRead(s Step, v Val) Val {
	ts.mut.Lock()
	if _, ok := ts.kv[s]; !ok {
		ts.kv[s] = v		// Write-once
	}
	v = ts.kv[s]			// Read
	ts.mut.Unlock()
	return v
}

func testCli(t *testing.T, self, nfail, ncom, maxTicket int,
		kv []Store, to *testOrder, wg *sync.WaitGroup) {
	c := &Client{}
	rv := func() int64 {
		return rand.Int63n(int64(maxTicket))
	}
	c.Start(2*nfail, nfail+1, kv, rv)
	for i := 0; i < ncom; i++ {
		h := c.Commit(fmt.Sprintf("cli %v commit %v", self, i))
		to.mut.Lock()
		to.committed(t, h)
		to.mut.Unlock()
	}
	c.Stop()
	wg.Done()
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, ncli, ncommits, maxTicket int) {
	desc := fmt.Sprintf("F=%v,N=%v,Clients=%v,Commits=%v,Tickets=%v",
		nfail, nnode, ncli, ncommits, maxTicket)
	t.Run(desc, func(t *testing.T) {

		// Create a test key/value store representing each node
		kv := make([]Store, nnode)
		for i := range kv {
			kv[i] = &testStore{kv: make(map[Step]Val)}
		}

		// Create a reference total order for safety checking
		to := &testOrder{}

		// Simulate the appropriate number of concurrent clients
		cli := make([]Client, ncli)
		wg := &sync.WaitGroup{}
		for i := range cli {
			wg.Add(1)
			go testCli(t, i, nfail, ncommits, maxTicket, kv, to, wg)
		}
		wg.Wait()

		// testResults(t, all) // Report test results
	})
}

func TestClient(t *testing.T) {
	//testRun(t, 0, 1, 1, 100000, 100) // Trivial case: 1 of 1 consensus!
	//testRun(t, 0, 1, 10, 100000, 100)
	//testRun(t, 0, 2, 1, 100000, 100) // Another trivial case: 2 of 2
	//testRun(t, 0, 2, 10, 100000, 100)

	testRun(t, 1, 3, 1, 1000, 100)  // Standard f=1 case
	testRun(t, 1, 3, 10, 1000, 100)
	testRun(t, 1, 3, 20, 1000, 100)
	testRun(t, 2, 6, 10, 1000, 100)  // Standard f=2 case
	testRun(t, 3, 9, 10, 1000, 100)   // Standard f=3 case
	testRun(t, 4, 12, 10, 1000, 100)   // Standard f=4 case
	testRun(t, 5, 15, 10, 1000, 100) // Standard f=10 case

	//testRun(t, 0, 3, 10, 1000, 100) // Less-than-maximum faulty nodes
	//testRun(t, 1, 7, 10, 1000, 100)
	//testRun(t, 2, 10, 10, 1000, 100)

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	//testRun(t, 1, 3, 10, 1000, 1) // Limit case: will never commit
	testRun(t, 1, 3, 10, 1000, 2) // Extreme low-entropy: rarely commits
	testRun(t, 1, 3, 10, 1000, 3) // A bit better bit still bad...
}

