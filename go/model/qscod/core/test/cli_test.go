package test

import (
	"sync"
	"testing"

	. "github.com/dedis/tlc/go/model/qscod/core"
)

// Trivial intra-process key-value store implementation for testing
type testStore struct {
	mut sync.Mutex // synchronization for testStore state
	v   Value      // the latest value written
}

// WriteRead implements the Store interface with a simple intra-process map.
func (ts *testStore) WriteRead(v Value) Value {
	ts.mut.Lock()
	defer ts.mut.Unlock()

	// Write value v only if it's newer than the last value written.
	if v.P.Step > ts.v.P.Step {
		ts.v = v
	}

	// Then return whatever was last written, regardless.
	return ts.v
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, ncli, maxstep, maxpri int) {

	// Create a simple test key/value store representing each node
	kv := make([]Store, nnode)
	for i := range kv {
		kv[i] = &testStore{}
	}

	TestRun(t, kv, nfail, ncli, maxstep, maxpri)
}

// Test the Client with a trivial in-memory key/value Store implementation.
func TestClient(t *testing.T) {
	testRun(t, 1, 3, 1, 100000, 100) // Standard f=1 case
	testRun(t, 1, 3, 2, 100000, 100)
	testRun(t, 1, 3, 10, 100000, 100)
	testRun(t, 1, 3, 20, 100000, 100)
	testRun(t, 1, 3, 50, 100000, 100)
	testRun(t, 1, 3, 100, 100000, 100)

	testRun(t, 2, 6, 10, 10000, 100)  // Standard f=2 case
	testRun(t, 3, 9, 10, 10000, 100)  // Standard f=3 case
	testRun(t, 4, 12, 10, 10000, 100) // Standard f=4 case
	testRun(t, 5, 15, 10, 10000, 100) // Standard f=10 case

	// Test with low-entropy tickets: hurts commit rate, but still safe!
	testRun(t, 1, 3, 10, 10000, 2) // Extreme low-entropy: rarely commits
	testRun(t, 1, 3, 10, 10000, 3) // A bit better bit still bad...
}
