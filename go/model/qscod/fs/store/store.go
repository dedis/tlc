// Package store provides a file system key/value Store for QSCOD.
// It uses the cas package to implement versioned write-once and read,
// with garbage collection of old versions before the last known commit.
//
package store

import (
	. "github.com/dedis/tlc/go/model/qscod"
	"github.com/dedis/tlc/go/model/qscod/fs/backoff"
	"github.com/dedis/tlc/go/model/qscod/fs/util"
	"github.com/dedis/tlc/go/model/qscod/fs/verst"
)

// FileStore implements a QSCOD key/value store
// as a directory in a file system.
//
type FileStore struct {
	state verst.State
	bc    backoff.Config
}

// Initialize FileStore to use a directory at a given file system path.
// If create is true, create the designated directory if it doesn't exist.
// If excl is true, fail if the designated directory already exists.
func (fs *FileStore) Init(path string, create, excl bool) error {
	return fs.state.Init(path, create, excl)
}

// SetReport sets the function used to report errors that occur
// while attempting to access the key/value store on the file system.
// The default report function writes the error via the standard log.
//
// Since we don't know in general which errors may be transitory
// and which are permanent failures, especially on remote file systems,
// FileStore assumes all errors may be transitory, just reports them,
// and keeps trying the access after a random exponential backoff.
// A custom report function can panic or terminate the process
// if it determines an error to be permanent and fatal, however.
//
func (fs *FileStore) SetReport(report func(error)) {
	fs.bc.Report = report
}

// Attempt to write the value v to a file associated with time-step step,
// then read back whichever value was successfully written first.
// Implements the qscod.Store interface.
//
func (fs *FileStore) WriteRead(v Value) (rv Value) {

	// Don't try to write version 0; that's a virtual placeholder.
	if v.P.Step == 0 {
		return v
	}

	try := func() (err error) {
		rv, err = fs.tryWriteRead(v)
		return err
	}

	fs.bc.Retry(try)
	return rv
}

func (fs *FileStore) tryWriteRead(val Value) (Value, error) {
	ver := verst.Version(val.P.Step)

	// Serialize the proposed value
	valb, err := util.EncodeValue(val)
	if err != nil {
		return Value{}, err
	}
	vals := string(valb)

	// Try to write it to the versioned store -
	// but don't fret if someone else wrote it or if it has expired.
	err = fs.state.WriteVersion(ver, vals)
	if err != nil && !verst.IsExist(err) && !verst.IsNotExist(err) {
		return Value{}, err
	}

	// Now read back whatever value was successfully written.
	vals, err = fs.state.ReadVersion(ver)
	if err != nil && verst.IsNotExist(err) {

		// The requested version has probably been aged out,
		// so catch up to the most recent committed Head.
		_, vals, err = fs.state.ReadLatest()
	}
	if err != nil {
		return Value{}, err
	}

	// Deserialize the value we read
	val, err = util.DecodeValue([]byte(vals))
	if err != nil {
		return Value{}, err
	}

	// Expire all versions before this latest one
	fs.state.Expire(verst.Version(val.P.Step))

	// Return the value v that we read
	return val, err
}

func (fs *FileStore) tryLatest() (Head, error) {

	// Read the latest state value from the file system
	ver, val, err := fs.state.ReadLatest()
	if err != nil {
		return Head{}, err
	}

	// Decode it into a Value
	v, err := util.DecodeValue([]byte(val))
	if err != nil && ver > 0 {
		return Head{}, err
	}

	// Return the last committed Head recorded in the latest Value
	//println("LastCommit returning", v.C.Step)
	return v.C, nil
}
