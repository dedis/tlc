// This package provides low-level utility operations
// needed by casfs.
package fs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// This code solves a different problem from, but is partly inspired by:
// https://github.com/google/renameio
// https://github.com/natefinch/atomic

// WriteFileOnce attempts to write data to filename atomically, only once,
// failing with ErrExist if someone else already wrote a file at filename.
//
// Ensures that no one ever sees a zero-length or incomplete file
// at the target filename, by writing data to a temporary file first,
// synchronizing it to stable storage, then atomically linking it into place.
//
func WriteFileOnce(filename string, data []byte, perm os.FileMode) error {

	// Create a temporary file in the target directory,
	// mainly to ensure that it's on the same volume for hard linking.
	dir, name := filepath.Split(filename)
	pattern := fmt.Sprintf("%s-*.tmp", name)
	tmpfile, err := ioutil.TempFile(dir, pattern)
	if err != nil {
		return err
	}

	// Make sure it gets closed and removed regardless of outcome.
	tmpname := tmpfile.Name()
	defer func() {
		tmpfile.Close()
		os.Remove(tmpname)
	}()

	// Write the data to the temporary file.
	n, err := tmpfile.Write(data)
	if err != nil {
		return err
	}
	if n < len(data) {
		return errors.New("short write")
	}

	// Set the correct file permissions
	if err := tmpfile.Chmod(perm); err != nil {
		return err
	}

	// Force the newly-written data to stable storage.
	// For background on this see commends for CloseAtomicallyReplace
	// at https://github.com/google/renameio/blob/master/tempfile.go
	//
	if err := tmpfile.Sync(); err != nil {
		return err
	}

	if err := tmpfile.Close(); err != nil {
		return err
	}

	// Atomically hard-link the temporary file into the target filename.
	// Unlike os.Rename, this fails if target filename already exists.
	if err := os.Link(tmpname, filename); err != nil {
		return err
	}

	return nil
}
