// Package store provides a file system key/value Store for QSCOD.
// It uses the cas package to implement versioned write-once and read,
// with garbage collection of old versions before the last known commit.
//
package store

import (
	"log"
	"math/rand"
	"time"

	. "github.com/dedis/tlc/go/model/qscod"
	"github.com/dedis/tlc/go/model/qscod/fs/util"
	"github.com/dedis/tlc/go/model/qscod/fs/verst"
)

// FileStore implements a QSCOD key/value store
// as a directory in a file system.
//
type FileStore struct {
	state  verst.State
	report func(err error)
}

// Initialize FileStore to use a directory at a given file system path.
// If create is true, create the designated directory if it doesn't exist.
// If excl is true, fail if the designated directory already exists.
func (fs *FileStore) Init(path string, create, excl bool) error {
	//fs.report = func(err error) { log.Println(err.Error()) }
	fs.report = func(err error) { log.Fatal(err.Error()) }
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
	fs.report = report
}

// Attempt to write the value v to a file associated with time-step step,
// then read back whichever value was successfully written first.
// Implements the qscod.Store interface.
//
func (fs *FileStore) WriteRead(step Step, v Value) (rv Value, rh Head) {

	// Don't try to write version 0; that's a virtual placeholder.
	if step == 0 {
		return v, Head{}
	}

	var backoff time.Duration
	for {
		before := time.Now()
		rv, rh, err := fs.tryWriteRead(step, v)
		if err == nil {
			return rv, rh // success
		}
		elapsed := time.Since(before)

		// Report the error as appropriate
		fs.report(err)

		// Wait for an exponentially-growing random backoff period,
		// with the duration of each operation attempt as the minimum
		if backoff <= elapsed {
			backoff = elapsed + 1
		}
		backoff += time.Duration(rand.Int63n(int64(backoff)))
		time.Sleep(backoff)
	}
}

func (fs *FileStore) tryWriteRead(step Step, v Value) (Value, Head, error) {
	ver := verst.Version(step)

	// Serialize the proposed value
	buf, err := util.EncodeValue(v)
	if err != nil {
		panic(err.Error())
	}

	// Try to write it to the versioned store -
	// but don't fret if someone else wrote it first.
	err = fs.state.WriteVersion(ver, string(buf))
	if err != nil && !verst.IsExist(err) {
		return Value{}, Head{}, err
	}

	// Now read back whatever value was successfully written.
	val, err := fs.state.ReadVersion(ver)
	switch {
	case err == nil: // success

		// Deserialize the value we read
		v, err = util.DecodeValue([]byte(val))
		if err != nil {
			return Value{}, Head{}, err
		}

		// Expire old versions before our last committed Head
		fs.state.Expire(verst.Version(v.C.Step))

		// Return the value v that we read
		return v, Head{}, err

	case verst.IsNotExist(err): // version doesn't exist

		// The requested version has probably been aged out,
		// so catch up to the most recent committed Head.
		println("tryWriteRead: catching up from", step)
		ver, val, err = fs.state.ReadLatest()
		if err != nil {
			return Value{}, Head{}, err
		}

		lv, err := util.DecodeValue([]byte(val))
		if err != nil {
			return Value{}, Head{}, err
		}

		// Return the passed v unmodified and the last committed Head
		return v, lv.C, err

	default: // some other error
		return Value{}, Head{}, err
	}
}
