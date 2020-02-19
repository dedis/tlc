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

func (ts *testStore) WriteRead(s Step, v Val) Val {
	ts.mut.Lock()
	if _, ok := ts.kv[s]; !ok {
		ts.kv[s] = v
	}
	v = ts.kv[s]
	ts.mut.Unlock()
	return v
}

func testCli(t *testing.T, self, f, ncom, maxTicket int,
		kv []Store, wg *sync.WaitGroup) {
	c := &Client{}
	rv := func() int64 {
		return rand.Int63n(int64(maxTicket))
	}
	c.Start(2*f, f+1, kv, rv)
	for i := 0; i < ncom; i++ {
		c.Commit(fmt.Sprintf("cli %v commit %v", self, i))
	}
	c.Stop()
	wg.Done()
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, ncli, ncommits, maxTicket int) {
	if maxTicket == 0 { // Default to moderate-entropy tickets
		maxTicket = 10 * nnode
	}
	desc := fmt.Sprintf("F=%v,N=%v,Commits=%v,Tickets=%v",
		nfail, nnode, ncommits, maxTicket)
	t.Run(desc, func(t *testing.T) {

		// Create a test key/value store representing each node
		kv := make([]Store, nnode)
		for i := range kv {
			kv[i] = &testStore{kv: make(map[Step]Val)}
		}

		// Simulate the appropriate number of concurrent clients
		cli := make([]Client, ncli)
		wg := &sync.WaitGroup{}
		for i := range cli {
			wg.Add(1)
			go testCli(t, i, nfail, ncommits, maxTicket, kv, wg)
		}
		wg.Wait()

		// testResults(t, all) // Report test results
	})
}

func TestClient(t *testing.T) {
	testRun(t, 0, 1, 1, 100000, 0) // Trivial case: 1 of 1 consensus!
	testRun(t, 0, 1, 10, 100000, 0)
	testRun(t, 0, 2, 1, 100000, 0) // Another trivial case: 2 of 2
	testRun(t, 0, 2, 10, 100000, 0)

	testRun(t, 1, 3, 1, 100000, 0)  // Standard f=1 case
	testRun(t, 1, 3, 10, 100000, 0)
	testRun(t, 1, 3, 100, 100000, 0)
	testRun(t, 2, 6, 10, 100000, 0)  // Standard f=2 case
	testRun(t, 3, 9, 10, 10000, 0)   // Standard f=3 case
	testRun(t, 4, 12, 10, 10000, 0)   // Standard f=4 case
	testRun(t, 5, 15, 10, 10000, 0) // Standard f=10 case

	testRun(t, 0, 3, 10, 100000, 0) // Less-than-maximum faulty nodes
	testRun(t, 1, 7, 10, 10000, 0)
	testRun(t, 2, 10, 10, 10000, 0)

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	testRun(t, 1, 3, 10, 100000, 1) // Limit case: will never commit
	testRun(t, 1, 3, 10, 100000, 2) // Extreme low-entropy: rarely commits
	testRun(t, 1, 3, 10, 100000, 3) // A bit better bit still bad...
}

