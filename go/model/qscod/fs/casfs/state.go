// Package casfs uses simple atomic POSIX file system operations,
// with no locking, to create simple versioned registers supporting
// atomic compare-and-set (CAS) functionality.
// It supports garbage-collection of old register versions
// by using atomic POSIX directory-manipulation operations.
//
// XXX describe in more detail
//
package casfs

import (
	"os"
	"fmt"
	"path/filepath"
	"io/ioutil"
	"errors"

	"github.com/bford/cofo/cbe"
	"github.com/dedis/tlc/go/model/qscod/fs/util"
)


const versPerGen = 100	// Number of versions between generation subdirectories

const genFormat = "gen-%d"	// Format for generation directory names
const verFormat = "ver-%d"	// Format for register version file names


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
func (st *State) Init(path string, create, excl bool) error {
	*st = State{path: path}	// Set path and clear cached state

	// First check if the path already exists and is a directory.
	stat, err := os.Stat(path)
	if err != nil && (!os.IsExist(err) || !create) {
		return err
	}
	if err == nil && !stat.IsDir() {
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
	val, nextGen, err := readVerFile(genpath, regname)
	if err != nil {
		return err
	}

	st.genVer = genver
	st.genPath = genpath

	st.ver = regver
	st.val = val
	st.nxg = nextGen

	return nil
}

// Scan a directory for highest-numbered file or subdirectory matching format.
func scan(path, format string, upTo Version) (
	maxver Version, maxname string, names []string, err error) {

	// Scan the casfs directory for the highest-numbered subdirectory.
	dir, err := os.Open(path)
	if err != nil {
		return 0, "", nil, err
	}
	info, err := dir.Readdir(0)
	if err != nil {
		return 0, "", nil, err
	}

	// Find the highest-numbered generation subdirectory
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
		if upTo > 0 {
			names = append(names, name)
		}
	}
	if maxver == 0 {	// No highest version!? oops
		return 0, "", nil, os.ErrNotExist
	}
	return
}

// Read and parse the register version file at regpath.
func readVerFile(genPath, verName string) (state, nextGen string, err error) {

	regPath := filepath.Join(genPath, verName)
	b, err := ioutil.ReadFile(regPath)
	if err != nil {
		return "", "", err
	}

	// The encoded value is always first and not optional
	val, b, err := cbe.Decode(b)
	if err != nil {
		println("corrupt casfs version file " + regPath)
		return "", "", err
	}

	// The encoded next-generation directory name is optional
	nxg, b, err := cbe.Decode(b)
	// (ignore decoding errors)

	return string(val), string(nxg), nil
}

// Read the current casfs register state and version number.
// Of course this may change at any time on the underlying file system,
// so the caller must assume it might be stale immediately.
func (st *State) Read() (val string, ver Version, err error) {

	if err := st.refresh(); err != nil {
		return "", 0, err
	}
	return st.val, st.ver, nil
}

// Write newVal atomically only if the current value still equals oldVal.
//
// XXX maybe don't need? don't export for now
func (st *State) writeIfEqual(newVal string, oldVal string) (err error) {

	// Reject the write if it doesn't even match our cached state
	if oldVal != st.val {
		return ErrChanged
	}

	return st.writeVer(newVal)
}

// Write newVal atomically only if the current version number is still oldVer.
func (st *State) Write(newVal string, oldVer Version) (err error) {

	// Reject the write if it doesn't even match our cached state
	if oldVer != st.ver {
		return ErrChanged
	}

	return st.writeVer(newVal)
}

// Write newVal to the next register version after the current cached version
func (st *State) writeVer(newVal string) (err error) {

	newVer := st.ver + 1
	newVerName := fmt.Sprintf(verFormat, newVer)

	// Should this register version start a new generation?
	tmpGenName := ""
	if newVer % versPerGen == 0 {
		// Prepare the new generation in a temporary directory first
		pattern := fmt.Sprintf(genFormat + "-tmp*", newVer)
		tmpPath, err := ioutil.TempDir(st.path, pattern)
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(tmpPath)
		}()
		tmpGenName = filepath.Base(tmpPath)

		// Write the new register version in the new directory (too)
		err = writeVerFile(tmpPath, newVerName, newVal, tmpGenName)
		if err != nil {
			return err
		}
	}

	// Write version into the (old) generation directory
	err = writeVerFile(st.genPath, newVerName, newVal, tmpGenName)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Read back whatever register version file actually got written,
	// which might be from someone else's write that won over ours.
	newVal, tmpGenName, err = readVerFile(st.genPath, newVerName)
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
				fmt.Sprintf(genFormat, newVer))
		err := os.Rename(oldGenPath, newGenPath)
		if !os.IsExist(err) && !os.IsNotExist(err) {
			return err
		}
	}

	// Update our cached state
	st.ver = newVer
	st.val = newVal
	st.nxg = tmpGenName
	return nil
}

func writeVerFile(genPath, verName, val, nextGen string) error {

	// Encode the new register version file
	b := cbe.Encode(nil, []byte(val))
	b = cbe.Encode(b, []byte(nextGen))

	// Write it atomically
	verPath := filepath.Join(genPath, verName)
	if err := util.WriteFileOnce(verPath, b, 0644); err != nil {
		return err
	}

	return nil
}

// Expire deletes file system state for versions older than before.
// Attempts either to read or to write expired versions will fail.
//
func (st *State) Expire(before Version) (err error) {

	// Find all existing generation directories up to version 'before'
	maxver, maxname, names, err := scan(st.path, genFormat, before)
	if err != nil {
		return err
	}
	if len(names) == 0 || maxver <= 0 || maxver > before {
		panic("shouldn't happen")
	}

	// Delete all generation directories before 'highest',
	// since those can only contain versions strictly before 'highest'.
	for _, name := range names {
		if name != maxname {
			genPath := filepath.Join(st.path, name)
			e := atomicRemoveAll(genPath)
			if e != nil && err == nil {
				err = e
			}
		}
	}

	return err
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


// State.Write returns ErrChanged if the casfs register
// has changed from the specified state value or version, respectively. 
var ErrChanged = errors.New("casfs register changed")

