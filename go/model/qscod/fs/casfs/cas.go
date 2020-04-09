// Package casfs uses simple atomic POSIX file system operations,
// with no locking, to create simple versioned registers supporting
// atomic compare-and-set (CAS) functionality.
package casfs

import (
	"os"
	"fmt"
	"path/filepath"
	"io/ioutil"
	"errors"

	"github.com/bford/cofo/cbe"
)


const versPerGen = 100	// Number of versions between generation subdirectories


// A register version number is just a 64-bit integer.
type Version int64

// State holds cached state for a single casfs versioned register.
type State struct {
	path string	// Base pathname of directory containing register state

	genVer Version	// Version number of highest generation subdirectory
	genPath string	// Pathname to generation subdirectory

	ver Version	// Highest register version known to exist already
	val string	// Cached register value for highest known version
	nxg string	// Cached next-directory string for highest version
}

// Initialize State to refer to a casfs register at a given file system path.
// If create is true, create the designated directory if it doesn't exist.
// If excl is true, fail if the designated directory already exists.
func (s *State) Init(path string, create, excl bool) error {
	*s = State{path: string}	// Set path and clear cached state

	// First check if the path already exists and is a directory.
	st, err := os.Stat(path)
	if err != nil && (!os.IsExist(err) || !create) {
		return err
	}
	if err == nil && !st.IsDir() {
		return os.ErrExist
	}

	// Create and initialize the register directory if appropriate.
	// This Mkdir could potentially race with someone else's,
	// so ignore already-exists errors unless excl is true.
	if err != nil && create {
		err = os.Mkdir(path, 0777)
		if err != nil && (!os.IsExist(err) || excl) {
			return err
		}

		// XXX create a subdirectory for version 1
	}

	return nil
}

// Refresh our cached state in attempt to "catch up" to the
// latest register version on the file system.
// Of course the file system may be a constantly-moving target
// so the refreshed state could be stale again immediately on return.
func (s *State) refresh() error {

	// First find the highest-numbered state generation subdirectory
	genver, genname, err := scan(s.path, "gen-%d")
	if err != nil {
		return err
	}

	// Then find the highest-numbered register version in that subdirectory
	genpath := filepath.Join(s.path, genname)
	regver, regname, err := scan(genpath, "ver-%d")
	if err != nil {
		return err
	}

	// Read that highest register version file
	regpath := filepath.Join(genpath, regname)
	val, nextGen, err := readVer(regpath)
	if err != nil {
		return err
	}

	s.genVer = genver
	s.genPath = genpath

	s.ver = regver
	s.val = val
	s.nxg = nextGen

	return nil
}

// Scan a directory for highest-numbered file or subdirectory matching format.
func scan(path, format string) (maxver Version, maxname string, err error) {

	// Scan the casfs directory for the highest-numbered subdirectory.
	dir, err := os.Open(s.path)
	if err != nil {
		return 0, "", err
	}
	info, err := os.Readdir(0)
	if err != nil {
		return 0, "", err
	}

	// Find the highest-numbered generation subdirectory
	var ver, maxver Version
	var maxname string
	for i := range info {
		name := info[i].Name()
		n, err := fmt.Sscanf(name, format, &ver)
		if n < 1 || err != nil || ver <= maxver {
			continue
		}

		// Confirm that the filename exactly matches the format
		if name == fmt.Sprintf(format, ver) {
			maxver, maxname = ver, name
		}
	}
	if maxver == 0 {	// No highest version!? oops
		return 0, "", os.ErrNotExist
	}
	return maxver, maxname, nil
}

// Read and parse the register version file at regpath.
func readVer(regPath string) (state, nextGen string, err error) {

	b, err := ioutil.ReadFile(regPath)
	if err != nil {
		return err
	}

	// The encoded value is always first and not optional
	val, b, err := cbe.Decode(b)
	if err != nil {
		println("corrupt casfs version file " + regPath)
		return err
	}

	// The encoded next-generation directory name is optional
	nxg, b, err := cbe.Decode(b)
	// (ignore decoding errors)

	return string(val), string(nxg), nil
}

// Read the current casfs register state and version number.
// Of course this may change at any time on the underlying file system,
// so the caller must assume it might be stale immediately.
func Read() (val string, ver Version, err error) {

	s.refresh()
}

// Write newVal atomically only if the current value still equals oldVal.
func (s *State) WriteIfEqual(newVal string, oldVal string) (err error) {

	// Reject the write if it doesn't even match our cached state
	if oldVal != s.val {
		return ErrChanged
	}

	return writeVer(newVal)
}

// Write newVal atomically only if the version number is still oldVer.
func WriteIfVersion(newVal string, oldVer Version) (err error) {

	// Reject the write if it doesn't even match our cached state
	if oldVer != s.ver {
		return ErrChanged
	}

	return writeVer(newVal)
}

// Write newVal to the next register version after the current cached version
func (s *State) writeVer(newVal string) (err error) {

	newVer = s.ver + 1
	newVerName = io.Sprintf("ver-%d", newVer)

	// Should this register version start a new generation?
	tmpGenName := ""
	if newVer % versPerGen == 0 {
		// Prepare the new generation in a temporary directory first
		pattern := fmt.Sprintf("gen-%d-tmp*", newVer)
		tmpPath, err := ioutil.TempDir(s.path, pattern)
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(tmpPath)
		}()
		tmpGenName = filepath.Base(tmpPath)

		// Write the new register version in the new directory (too)
		tmpVerName := filepath.Join(tmpPath, newVerName)
		err := writeVerFile(tmpPath, newVerName, newVal, tmpGenName)
		if err != nil {
			return err
		}
	}

	// Write version into the (old) generation directory
	err := writeVerFile(s.genPath, newVerName, newVal, tmpGenName)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Read back whatever register version file actually got written,
	// which might be from someone else's write that won over ours.
	newVal, tmpGenName, err = readVerFile(s.genPath, newVerName)
	if err != nil {
		return err
	}

	// If the (actual) new version indicates a new generation directory,
	// try to move the temporary directory into its place.
	// It's harmless if multiple writers attempt this redundantly:
	// it fails if either the old temporary directory no longer exists
	// or if a directory with the new name already exists.
	if tmpGenName != "" {
		oldGenPath = filepath.Join(s.path, tmpGenName)
		newGenPath = filepath.Join(s.path, fmt.Sprintf("gen-%d", newVer))
		err := os.Rename(oldGenPath, newGenPath)
		if !os.IsExist(err) && !os.IsNotExist(err) {
			return err
		}
	}

	// Update our cached state
	s.ver = newVer
	s.val = newVal
	s.nxg = tmpGenName
	return nil
}

func (s *State) writeVerFile(dirPath, verName, val, nextGen string) error {

	// Encode the new register version file
	b := cbe.Encode(nil, val)
	b = cbe.Encode(b, []byte(nextGen))

	// Write it atomically
	verPath = filepath.Join(dirPath, verName)
	if err := util.WriteFileOnce(verPath, b, 0644); err != nil {
		return err
	}

	return nil
}


// WriteIfEqual and WriteIfVersion return ErrChanged if the casfs register
// has changed from the specified state value or version, respectively. 
var ErrChanged = errors.New("casfs register changed")

