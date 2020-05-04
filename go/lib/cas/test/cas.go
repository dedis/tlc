// Package test implements shareable code for testing instantiations
// of the cas.Store check-and-set storage interface.
package test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/dedis/tlc/go/lib/cas"
)

// History records a history of cas.Store version/value observations,
// typically made across concurrent goroutines or even distributed nodes,
// and checks all these observations for consistency.
//
type History struct {
	hist map[int64]string // version-value map defining observed history
	mut  sync.Mutex       // mutex protecting this reference order
}

// Observe records an old/new value pair that was observed via a cas.Store,
// checks it for consistency against all prior recorded old/new value pairs,
// and reports any errors via testing context t.
//
func (to *History) Observe(t *testing.T, version int64, value string) {
	to.mut.Lock()
	defer to.mut.Unlock()

	// Create the successor map if it doesn't already exist
	if to.hist == nil {
		to.hist = make(map[int64]string)
	}

	// If there is any recorded successor to old, it had better be new.
	if old, exist := to.hist[version]; exist && old != value {
		t.Errorf("\nInconsistency:\n ver %v\n old %q\n new %q\n",
			version, old, value)
	}

	// Record the new successor
	to.hist[version] = value
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
	return &checkedStore{t: t, h: h, s: store}
}

type checkedStore struct {
	t *testing.T // Testing context
	h *History   // History we're using for consistency-checking
	s cas.Store  // Underlying compare-and-set Store

	lver int64  // Last version number read from the underlying Store
	lval string // Last value read from the underlying Store

	rver int64 // Our fudged informational version numbers for testing
}

func (cs *checkedStore) CompareAndSet(ctx context.Context, old, new string) (
	version int64, actual string, err error) {

	// Sanity-check the arguments we're passed
	if old != cs.lval {
		cs.t.Errorf("CompareAndSet: wrong old value %q != %q",
			old, cs.lval)
	}
	if new == "" {
		cs.t.Errorf("CompareAndSet: new value empty")
	}
	if new == old {
		cs.t.Errorf("CompareAndSet: new value identical to old")
	}

	// Try to change old to new atomically.
	version, actual, err = cs.s.CompareAndSet(ctx, old, new)

	// Sanity-check the Store-assigned version numbers
	if version < cs.lver {
		cs.t.Errorf("CompareAndSet: Store version number decreased")
	}
	if version == cs.lver && actual != cs.lval {
		cs.t.Errorf("CompareAndSet: Store version failed to increase")
	}

	// Record and consistency-check all version/value pairs we observe.
	cs.h.Observe(cs.t, version, actual)

	// Produce our own informational version numbers to return
	// that increase a bit unpredictability for testing purposes.
	if version > cs.lver {
		cs.rver++
	}
	cs.rver += rand.Int63n(3)

	// Update our cached record of the underlying Store's last state
	cs.lver, cs.lval = version, actual

	// Return the actual new value regardless.
	return cs.rver, actual, err
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
		old, err := "", error(nil)
		for k := 0; k < naccesses; k++ {
			new := fmt.Sprintf("store %v thread %v access %v",
				i, j, k)
			//println("tester", i, j, "access", k)
			_, old, err = cs.CompareAndSet(bg, old, new)
			if err != nil {
				t.Error("CompareAndSet: " + err.Error())
			}
		}
		//println("tester", i, j, "done")
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
	wg.Wait()
}
