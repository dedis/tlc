// Package cas implements a versioned check-and-set (CAS) state abstraction
// in a directory on a standard POSIX-compatible file system.
//
// See the tlc/go/lib/cas package for general information
// on this CAS state abstraction.
//
// This implementation is just a simple wrapper around the verst package,
// which provides a slightly-more-general versioned state abstraction.
// To implement CAS, in essence, we simply expire old versions immediately
// as soon as any new version is written.
//
package cas

import (
	"context"

	"github.com/dedis/tlc/go/lib/fs/verst"
)

// FileStore holds cached state for a single compare-and-set register.
type FileStore struct {
	vs verst.State // underlying versioned state
}

// Initialize FileStore to refer to a CAS register at a given file system path.
// If create is true, create the designated directory if it doesn't exist.
// If excl is true, fail if the designated directory already exists.
func (st *FileStore) Init(path string, create, excl bool) error {
	return st.vs.Init(path, create, excl)
}

// CheckAndSet conditionally writes a new version to the stored state,
// then reads and returns the actual current state version and content.
//
// The write attempt succeeds only if the proposed version is strictly larger
// than the latest version that has been written so far,
// and otherwise silently does nothing without producing an error.
// CheckAndSet returns a non-nil error only if an unexpected error occurred,
// other than a simple race between multiple writers.
//
func (st *FileStore) CheckAndSet(ctx context.Context, ver int64, val string) (
	actualVer int64, actualVal string, err error) {

	// Try to write the new version to the underlying versioned store -
	// but don't fret if someone else wrote it or if it has expired.
	err = st.vs.WriteVersion(ver, val)
	if err != nil && !verst.IsExist(err) && !verst.IsNotExist(err) {
		return 0, "", err
	}

	// Now read back whatever value was successfully written.
	val, err = st.vs.ReadVersion(ver)
	if err != nil && verst.IsNotExist(err) {

		// The requested version has probably been aged out,
		// so catch up to the most recent committed value.
		ver, val, err = st.vs.ReadLatest()
	}
	if err != nil {
		return 0, "", err
	}

	// Expire all versions before this latest one
	st.vs.Expire(ver)

	// Return the actual version and value that we read
	return ver, val, err
}
