// Package cas defines a simple compare-and-set (CAS) state interface.
// It defines a generic access interface called Store,
// and a simple in-memory CAS register called Register.
//
package cas

import (
	"context"
	"sync"
)

// Store defines a CAS storage abstraction via a single CompareAndSet method.
//
// CompareAndSet writes a proposed new value to the state,
// provided the state still has the specified old value.
// The compare and conditional write are guaranteed to be atomic,
// ensuring that the caller can avoid undetected state loss due to races.
// CompareAndSet then reads and returns the latest actual state value.
//
// State values are arbitrary opaque Go strings, and may contain binary data.
// While values in principle have no particular length limit, in practice
// Store implementations may expect them to be "reasonably small", i.e.,
// efficient for storing metadata but not necessarily for bulk data storage.
//
// The Store assigns a version number to each value CompareAndSet returns.
// Version numbers must be monotonic but need not be assigned consecutively.
// The version number must increase when the stored value changes,
// and may increase at other times even when the value hasn't changed.
// The caller may simply ignore the version numbers CompareAndSet returns,
// or may use them for consistency-checking and debugging:
// see the Checked wrapper function in the test subpackage for example.
// Version numbers do not impose a burden on Store interface implementations,
// in part because it's easy to adapt a non-versioned underlying CAS interface
// with a simple wrapper that attaches a version number to each proposed value.
//
// CompareAndSet takes a Context parameter so that long-running implementations,
// particularly those accessing remote storage in a distributed system,
// can respond to cancellation requests and timeouts appropriately.
// For robust asynchronous operation, CompareAndSet should return err != nil
// only when its context is cancelled or when it encounters an error
// that it detects to be permanent and unrecoverable for sure.
// On encountering errors that may be temporary (e.g., due to network outages),
// it is better for the Store to keep trying until success or cancellation,
// using the lib/backoff package for example.
//
type Store interface {
	CompareAndSet(ctx context.Context, old, new string) (
		version int64, actual string, err error)
}

// Register implements a simple local-memory CAS register.
// It is thread-safe and ready for use on instantiation.
type Register struct {
	mut sync.Mutex // for synchronizing accesses
	ver int64      // version number of the latest value
	val string     // the latest value written
}

// CompareAndSet implements the Store interface for the CAS register.
func (r *Register) CompareAndSet(ctx context.Context, old, new string) (
	version int64, actual string, err error) {

	r.mut.Lock()
	defer r.mut.Unlock()

	// Update the value only if the current value is as expected.
	if r.val == old {
		r.ver, r.val = r.ver+1, new
	}

	// Return the actual new value, changed or not.
	return r.ver, r.val, nil
}

