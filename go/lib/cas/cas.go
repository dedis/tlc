// Package cas defines a simple versioned check-and-set (CAS) state interface.
// It defines a generic access interface called Store,
// and a simple in-memory CAS register called Register.
//
// This CAS abstraction is functionally equivalent to classic compare-and-swap,
// but slightly more efficient and robust because it compares version numbers,
// which are always small and guaranteed to increase with each state change,
// rather than comparing actual state contents, which are arbitrary strings.
//
package cas

import (
	"context"
	"errors"
	"sync"
)

// Store defines a CAS storage abstraction via a single CheckAndSet method.
//
// CheckAndSet writes a new version of the state containing value reqVal,
// provided the state has not changed since prior version lastVer.
// CheckAndSet then reads and returns the latest state version and value.
// The version check and conditional write are guaranteed to be atomic,
// ensuring that the caller can avoid undetected state loss due to races.
//
// When CheckAndSet succeeds in writing the caller's proposed value reqVal,
// it returns actualVer > lastVer, actualVal == reqVal, and err == nil.
// The Store assigns new version numbers, which must be increasing
// but need not be consecutive.
//
// If the stored state had already advanced past version number lastVer,
// CheckAndSet returns actualVer > lastVer, actualVal == the state value
// associated with actualVer, and err == Changed.
//
// If CheckAndSet returns any error other than Changed, then it may return
// actualVer == 0 and actualVal == "" to indicate the state couldn't be read.
//
// Version numbers are 64-bit integers, and values are arbitrary Go strings.
// Value strings may contain binary data; the Store treats them as opaque.
// While values in principle have no particular length limit, in practice
// Store implementations may expect them to be "reasonably small", i.e.,
// efficient for storing metadata but not necessarily for bulk data storage.
//
// CheckAndSet takes a Context parameter so that long-running implementations,
// particularly those accessing remote storage in a distributed system,
// can respond to cancellation requests and timeouts appropriately.
//
type Store interface {
	CheckAndSet(ctx context.Context, lastVer int64, reqVal string) (
		actualVer int64, actualVal string, err error)
}

// Register implements a simple local-memory CAS register.
// It is thread-safe and ready for use on instantiation.
type Register struct {
	mut sync.Mutex // for synchronizing accesses
	ver int64      // the latest version number
	val string     // the latest value written
}

// CheckAndSet implements the Store interface for the CAS register.
func (r *Register) CheckAndSet(
	ctx context.Context, lastVer int64, reqVal string) (
	actualVer int64, actualVal string, err error) {

	r.mut.Lock()
	defer r.mut.Unlock()

	// If the version doesn't match, just return the latest version.
	if r.ver != lastVer {
		return r.ver, r.val, Changed
	}

	// Write and return the new version
	r.ver, r.val = lastVer+1, reqVal
	return r.ver, r.val, nil
}

// CheckAndSet returns Changed when the stored value was changed
// by someone else since the last version the caller indicated.
var Changed = errors.New("Version changed")
