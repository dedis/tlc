package fs

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"

	. "github.com/dedis/tlc/go/model/qscod"
)

// Encode a Value for transmission.
// Currently uses GOB encoding for simplicity,
// but we should change that to something not Go-specific..
func encodeValue(v Value) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode a Value from its serialized format.
func decodeValue(b []byte) (v Value, err error) {
	r := bytes.NewReader(b)
	dec := gob.NewDecoder(r)
	err = dec.Decode(&v)
	return
}

// Simple file system key/value Store for QSCOD,
// with no support for garbage collection.
//
// Intended only for education, testing, and experimentation,
// not for any production use.
//
type simpleStore struct {
	dirPath string // directory containing per-step files
	comh    Hist   // latest history known to be committed
}

func (ss *simpleStore) Init(path string) error {
	ss.dirPath = path
	return os.Mkdir(path, 0744)
}

func (ss *simpleStore) WriteRead(step Step, v Value) (rv Value, rh Hist) {

	// Serialize the proposed value
	buf, err := encodeValue(v)
	if err != nil {
		panic(err.Error()) // XXX
	}

	// Try to write the file, ignoring already-exists errors
	name := fmt.Sprintf("ver-%d", step)
	path := filepath.Join(ss.dirPath, name)
	err = WriteFileOnce(path, buf, 0666)
	if err != nil && !os.IsExist(err) {
		panic(err.Error()) // XXX
	}

	// Read back whatever file was successfully written first there
	rbuf, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err.Error()) // XXX
	}

	// Deserialize the value read
	rv, err = decodeValue(rbuf)
	if err != nil {
		panic(err.Error()) // XXX
	}

	return rv, rh
}

func (ss *simpleStore) Committed(comh Hist) {
	// do nothing - no garbage collection
}

// Object to record the common total order and verify it for consistency
type testOrder struct {
	hs  []Hist     // all history known to be committed so far
	mut sync.Mutex // mutex protecting this reference order
}

// When a client reports a history h has been committed,
// record that in the testOrder and check it for global consistency.
func (to *testOrder) committed(t *testing.T, h Hist) {
	to.mut.Lock()
	defer to.mut.Unlock()

	// Ensure history slice is long enough
	for h.Step >= Step(len(to.hs)) {
		to.hs = append(to.hs, Hist{})
	}

	// Check commit consistency across all concurrent clients
	switch {
	case to.hs[h.Step] == Hist{}:
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
	h := Hist{}
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

func testPath(node int) string {
	return fmt.Sprintf("test-store-%d", node)
}

//  Run a consensus test case with the specified parameters.
func testRun(t *testing.T, nfail, nnode, ncli, ncommits, maxpri int) {
	desc := fmt.Sprintf("F=%v,N=%v,Clients=%v,Commits=%v,Tickets=%v",
		nfail, nnode, ncli, ncommits, maxpri)
	t.Run(desc, func(t *testing.T) {

		// Create a test key/value store representing each node
		kv := make([]Store, nnode)
		for i := range kv {
			ss := &simpleStore{}
			path := testPath(i)
			os.RemoveAll(path)
			if err := ss.Init(path); err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(path)
			kv[i] = ss
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
