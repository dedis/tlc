package simple

import (
	"fmt"
	"os"
	"testing"

	. "github.com/dedis/tlc/go/model/qscod/core"
	. "github.com/dedis/tlc/go/model/qscod/core/test"
)

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

	TestRun(t, kv, nfail, ncli, maxstep, maxpri)
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
