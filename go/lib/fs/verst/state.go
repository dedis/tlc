// Package verst implements a simple persistent versioned state abstraction
// in a directory on a standard POSIX-compatible file system.
//
// The abstraction that verst presents is essentially a key/value store,
// in which the keys are sequentially-increasing version numbers,
// and the values are opaque byte strings (which we represent as Go strings).
// The main operations verst provides are
// reading a particular (or the latest) version,
// and writing a new version as a successor to the latest version.
// The implementation ensures that new version writes are atomic:
// clients will never read partially-written values, for example.
// If several clients attempt to write the same new version concurrently,
// one will succeed while all the others will fail,
// and potentially need to retry with respect to the new latest version.
//
// The package is designed assuming that values are small,
// e.g., metadata rather than bulk data, appropriate for Go strings.
// and reading/writing all at once as atomic units.
// Bulk data should be handled by other means.
//
// The verst package uses simple atomic POSIX file system operations,
// with no locking, to manage concurrency in the underlying file system.
// It supports garbage-collection of old state versions
// by using atomic POSIX directory-manipulation operations.
// Barring bugs, it "should" not be possible to violate
// the guaranteed atomicity properties or corrupt the state store
// regardless of how many clients may be competing to access it
// or with what access patterns or delays.
// This atomicity is necessarily only as good as the underlying file system's
// guarantee of atomicity and consistency of the underlying operations:
// e.g., if the underlying file system can leave a rename operation
// half-completed after a badly-timed crash, the state could be corrupted.
//
// The design of verst guarantees progress, but not fairness:
// that is, by standard definitions it is lock-free but not wait free
// (https://en.wikipedia.org/wiki/Non-blocking_algorithm).
// Regardless of the amount of contention to write a new version, for example,
// verst guarantees that at least one client will be able to make progress.
// It makes no guarantee of a "fair" rotation among clients, however,
// or that some particularly slow or otherwise unlucky client will not starve.
//
// While this package currently lives in the tlc repository,
// it is not particularly specific to TLC and depends on nothing else in it,
// and hence might eventually be moved to a more generic home if appropriate.
//
// XXX describe the techniques in a bit more detail.
//
package verst

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	//	"errors"

	"github.com/bford/cofo/cbe"
	"github.com/dedis/tlc/go/lib/fs/atomic"
)

//const versPerGen = 100 // Number of versions between generation subdirectories
const versPerGen = 10 // Number of versions between generation subdirectories

const genFormat = "gen-%d" // Format for generation directory names
const verFormat = "ver-%d" // Format for register version file names

// A register version number is just a 64-bit integer.
type Version int64

// State holds cached state for a single verst versioned register.
type State struct {
	path    string  // Base pathname of directory containing register state
	genVer  Version // Version number of highest generation subdirectory
	genPath string  // Pathname to generation subdirectory
	ver     Version // Highest register version known to exist already
	val     string  // Cached register value for highest known version
	expVer  Version // Version number before which state is expired
}

// Initialize State to refer to a verst register at a given file system path.
// If create is true, create the designated directory if it doesn't exist.
// If excl is true, fail if the designated directory already exists.
func (st *State) Init(path string, create, excl bool) error {
	*st = State{path: path} // Set path and clear cached state

	// First check if the path already exists and is a directory.
	stat, err := os.Stat(path)
	switch {
	case err == nil && !stat.IsDir():
		return os.ErrExist // already exists, but not a directory

	case err == nil && !excl:
		return st.refresh() // exists: load our cache from it

	case err != nil && (!IsNotExist(err) || !create):
		return err // didn't exist and we can't create it
	}

	// Create and initialize the version state directory,
	// initially with a temporary name for atomicity.
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "." // Ensure dir is nonempty
	}
	tmpPath, err := ioutil.TempDir(dir, name+"-*.tmp")
	if err != nil {
		return err
	}
	defer func() { // Clean up on return if we can't move it into place
		os.RemoveAll(tmpPath)
	}()

	// Create an initial generation directory for state version 0
	genPath := filepath.Join(tmpPath, fmt.Sprintf(genFormat, 0))
	err = os.Mkdir(genPath, 0777)
	if err != nil {
		return err
	}

	// Create an initial state version 0 with the empty string as its value
	err = writeVerFile(genPath, fmt.Sprintf(verFormat, 0), "", "")
	if err != nil {
		return err
	}

	// Atomically move the temporary version state directory into place.
	err = os.Rename(tmpPath, path)
	if err != nil && (excl || !IsExist(err)) {
		return err
	}

	// Finally, load our cache from the state directory.
	return st.refresh()
}

// Refresh our cached state in attempt to "catch up" to the
// latest register version on the file system.
// Of course the file system may be a constantly-moving target
// so the refreshed state could be stale again immediately on return.
func (st *State) refresh() error {

	// First find the highest-numbered state generation subdirectory
	genver, genname, _, err := scan(st.path, genFormat, 0)
	if err != nil {
		return err
	}

	// Then find the highest-numbered register version in that subdirectory
	genpath := filepath.Join(st.path, genname)
	regver, regname, _, err := scan(genpath, verFormat, 0)
	if err != nil {
		return err
	}

	// Read that highest register version file
	val, _, err := readVerFile(genpath, regname)
	if err != nil {
		return err
	}

	st.genVer = genver
	st.genPath = genpath

	st.ver = regver
	st.val = val

	return nil
}

// Scan a directory for highest-numbered file or subdirectory matching format.
// If upTo > 0, returns the highest-numbered version no higher than upTo.
func scan(path, format string, upTo Version) (
	maxver Version, maxname string, names []string, err error) {

	// Scan the verst directory for the highest-numbered subdirectory.
	dir, err := os.Open(path)
	if err != nil {
		return 0, "", nil, err
	}
	info, err := dir.Readdir(0)
	if err != nil {
		return 0, "", nil, err
	}

	// Find the highest-numbered generation subdirectory
	maxver = -1
	for i := range info {
		name := info[i].Name()

		// Scan the version number embedded in the name, if any,
		// and confirm that the filename exactly matches the format.
		var ver Version
		n, err := fmt.Sscanf(name, format, &ver)
		if n < 1 || err != nil || name != fmt.Sprintf(format, ver) {
			continue
		}

		// Find the highest extant version number
		// (no greater than upTo, if upTo is nonzero)
		if ver > maxver && (upTo == 0 || ver <= upTo) {
			maxver, maxname = ver, name
		}

		// If upTo is nonzero, collect all the matching names.
		if upTo > 0 && ver <= upTo {
			names = append(names, name)
		}
	}
	if maxver < 0 { // No highest version!? oops
		return 0, "", nil, os.ErrNotExist
	}
	return
}

// Read and parse the register version file at regpath.
func readVerFile(genPath, verName string) (val, nextGen string, err error) {

	regPath := filepath.Join(genPath, verName)
	b, err := ioutil.ReadFile(regPath)
	if err != nil {
		return "", "", err
	}

	// The encoded value is always first and not optional
	rb, b, err := cbe.Decode(b)
	if err != nil {
		println("corrupt verst version file " + regPath)
		return "", "", err
	}

	// The encoded next-generation directory name is optional
	nxg, b, err := cbe.Decode(b)
	// (ignore decoding errors)

	return string(rb), string(nxg), nil
}

// Read the latest version of the stored state,
// returning both the highest version number (key) and associated value.
// Of course a new version might be written at any time,
// so the caller must assume this information could become stale immediately.
func (st *State) ReadLatest() (ver Version, val string, err error) {

	if err := st.refresh(); err != nil {
		return 0, "", err
	}
	return st.ver, st.val, nil
}

// Read a specific version of the stored state,
// returning the associated value if possible.
// Returns ErrNotExist if the specified version does not exist,
// either because it has never been written or because it has been expired.
func (st *State) ReadVersion(ver Version) (val string, err error) {

	// In the common case of reading back the last-written version,
	// just return its value from our cache.
	if ver == st.ver {
		return st.val, nil
	}

	// Find and read the appropriate version file
	val, err = st.readUncached(ver)
	if err != nil {
		return "", err
	}

	// Update our cached state as appropriate.
	if ver > st.ver {
		st.ver = ver
		st.val = val
	}

	return val, nil
}

func (st *State) readUncached(ver Version) (val string, err error) {

	// Optimize for sequential reads of the "next" version
	verName := fmt.Sprintf(verFormat, ver)
	if ver >= st.genVer {
		val, _, err := readVerFile(st.genPath, verName)
		if err == nil {
			return val, nil // success
		}
		if !IsNotExist(err) {
			return "", err // error other than non-existent
		}
	}

	// Fallback: scan for the generation containing requested version.
	//println("readUncached: fallback at", ver)
	genVer, genName, _, err := scan(st.path, genFormat, ver)
	if err != nil {
		return "", err
	}
	//println("readUncached: found", ver, "in gen", genVer)

	// The requested version should be in directory genName if it exists.
	genPath := filepath.Join(st.path, genName)
	val, _, err = readVerFile(genPath, verName)
	if err != nil {
		return "", err
	}

	// Update our cached generation state
	if ver >= st.ver {
		println("moving to generation", genVer, "at ver", ver)
		st.genVer = genVer
		st.genPath = genPath
	}

	return val, err
}

// Write version ver with associated value val if ver is not yet written.
// The caller may skip version numbers, e.g., to catch up a delayed store,
// but must never try to (re-)write older versions up to the last written.
//
func (st *State) WriteVersion(ver Version, val string) (err error) {

	if ver <= st.ver {
		return ErrExist
	}
	verName := fmt.Sprintf(verFormat, ver)

	// Should this register version start a new generation?
	tmpGenName := ""
	if ver%versPerGen == 0 {

		// Prepare the new generation in a temporary directory first
		pattern := fmt.Sprintf(genFormat+"-*.tmp", ver)
		tmpPath, err := ioutil.TempDir(st.path, pattern)
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(tmpPath)
		}()
		tmpGenName = filepath.Base(tmpPath)

		// Write the new register version in the new directory (too)
		err = writeVerFile(tmpPath, verName, val, tmpGenName)
		if err != nil {
			return err
		}
	}

	// Write version into the (old) generation directory
	err = writeVerFile(st.genPath, verName, val, tmpGenName)
	if err != nil && !IsExist(err) {
		return err
	}

	// Read back whatever register version file actually got written,
	// which might be from someone else's write that won over ours.
	val, tmpGenName, err = readVerFile(st.genPath, verName)
	if err != nil {
		return err
	}

	// If the (actual) new version indicates a new generation directory,
	// try to move the temporary directory into its place.
	// It's harmless if multiple writers attempt this redundantly:
	// it fails if either the old temporary directory no longer exists
	// or if a directory with the new name already exists.
	if tmpGenName != "" {
		oldGenPath := filepath.Join(st.path, tmpGenName)
		newGenPath := filepath.Join(st.path,
			fmt.Sprintf(genFormat, ver))
		err := os.Rename(oldGenPath, newGenPath)
		if err != nil && !IsExist(err) && !IsNotExist(err) {
			return err
		}

		// It's a good time to expire old generations when feasible
		st.expireOld()

		// Update our cached generation state
		st.genVer = ver
		st.genPath = newGenPath
	}

	// Update our cached version state
	st.ver = ver
	st.val = val
	return nil
}

func writeVerFile(genPath, verName, val, nextGen string) error {

	// Encode the new register version file
	b := cbe.Encode(nil, []byte(val))
	b = cbe.Encode(b, []byte(nextGen))

	// Write it atomically
	verPath := filepath.Join(genPath, verName)
	if err := atomic.WriteFileOnce(verPath, b, 0644); err != nil {
		return err
	}

	return nil
}

// Expire indicates that state versions earlier than before may be deleted.
// It does not necessarily delete these older versions immediately, however.
// Attempts either to read or to write expired versions will fail.
//
func (st *State) Expire(before Version) {
	if st.expVer < before {
		st.expVer = before
	}
}

// Actually try to delete expired versions.
// We do this only about once per generation for efficiency.
func (st *State) expireOld() {

	// Find all existing generation directories up to version 'before'
	maxVer, maxName, names, err := scan(st.path, genFormat, st.expVer)
	if err != nil || len(names) == 0 {
		return // ignore errors, e.g., no expired generations
	}
	if maxVer < 0 || maxVer > st.expVer {
		println("expireOld oops", len(names), maxVer, st.expVer)
		panic("shouldn't happen")
	}

	// Delete all generation directories before maxVer,
	// since those can only contain versions strictly before maxVer.
	for _, genName := range names {
		if genName != maxName {
			genPath := filepath.Join(st.path, genName)
			atomicRemoveAll(genPath)
		}
	}
}

// Atomically remove the directory at path,
// ensuring that no one sees inconsistent states within it,
// by renaming it before starting to delete its contents.
func atomicRemoveAll(path string) error {

	tmpPath := fmt.Sprintf("%s.old", path)
	if err := os.Rename(path, tmpPath); err != nil {
		return err
	}

	return os.RemoveAll(tmpPath)
}

// State.Write returns an error matching this predicate
// when the version the caller asked to write already exists.
func IsExist(err error) bool {
	return os.IsExist(err)
}

// State.Read returns an error matching this predicat
// when the version the caller asked to read does not exist.
func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

var ErrExist = os.ErrExist
var ErrNotExist = os.ErrNotExist
