// Package test implements shareable code for testing instantiations
// of the cas.Store check-and-set storage interface.
package test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dedis/tlc/go/lib/cas"
)

// History records a history of cas.Store version/value observations,
// typically made across concurrent goroutines or even distributed nodes,
// and checks all these observations for consistency.
//
type History struct {
	vs  []string   // all history known to be committed so far
	mut sync.Mutex // mutex protecting this reference order
}

// Observe records a version/value pair that was observed via a cas.Store,
// checks it for consistency against all prior recorded version/value pairs,
// and reports any errors via testing context t.
//
func (to *History) Observe(t *testing.T, ver int64, val string) {
	to.mut.Lock()
	defer to.mut.Unlock()

	// Ensure history slice is long enough
	for ver >= int64(len(to.vs)) {
		to.vs = append(to.vs, "")
	}

	// Check commit consistency across all concurrent clients
	switch {
	case to.vs[ver] == "":
		to.vs[ver] = val
	case to.vs[ver] != val:
		t.Errorf("History inconsistency at %v:\nold: %+v\nnew: %+v",
			ver, to.vs[ver], val)
	}
}

// Checked wraps the provided CAS store with a consistency-checker
// that records all requested and observed accesses against history h,
// reporting any inconsistency errors discovered via testing context t.
//
// The wrapper also consistency-checks the caller's accesses to the Store,
// ensuring that the provided lastVer is indeed the last version retrieved.
// This means that when checking a Store that is shared across goroutines,
// each goroutine should have its own Checked wrapper around that Store.
//
func Checked(t *testing.T, h *History, store cas.Store) cas.Store {
	return &checkedStore{t, h, store, 0}
}

type checkedStore struct {
	t  *testing.T
	h  *History
	s  cas.Store
	lv int64
}

func (cs *checkedStore) CheckAndSet(
	ctx context.Context, lastVer int64, reqVal string) (
	actualVer int64, actualVal string, err error) {

	if lastVer != cs.lv {
		cs.t.Errorf("Checked CAS store passed wrong last version")
	}

	actualVer, actualVal, err = cs.s.CheckAndSet(ctx, lastVer, reqVal)
	cs.h.Observe(cs.t, actualVer, actualVal)

	cs.lv = actualVer
	return actualVer, actualVal, err
}

// Stores torture-tests one or more cas.Store interfaces
// that are all supposed to represent the same consistent underlying state.
// The test is driven by nthreads goroutines per Store interface,
// each of which performs naccesses CAS operations on its interface.
//
func Stores(t *testing.T, nthreads, naccesses int, store ...cas.Store) {

	bg := context.Background()
	wg := sync.WaitGroup{}
	h := &History{}

	tester := func(i, j int) {
		cs := Checked(t, h, store[i])
		var lastVer int64
		for k := 0; k < naccesses; k++ {
			reqVal := fmt.Sprintf("store %v thread %v access %v",
				i, j, k)
			lastVer, _, _ = cs.CheckAndSet(bg, lastVer, reqVal)
		}
		wg.Done()
	}

	// Launch a set of goroutines for each Store interface.
	// To maximize cross-store concurrency,
	// launch the first thread per store, then the second per store, etc.
	for j := 0; j < nthreads; j++ {
		for i := range store {
			wg.Add(1)
			go tester(i, j)
		}
	}

	// Wait for all tester goroutines to complete
}
