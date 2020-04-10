// This package provides a simple file system key/value Store for QSCOD,
// with no support for garbage collection.
// It is intended only for education, testing, and experimentation,
// and not for any production use.
//
package simple

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/dedis/tlc/go/model/qscod"
	"github.com/dedis/tlc/go/model/qscod/fs/util"
)

// FileStore implements a simple QSCOD key/value store
// as a directory in a file system.
// The caller must create the directory designated by Path.
//
type FileStore struct {
	Path string // Directory to contain files representing key/value state
}

// Attempt to write the value v to a file associated with time-step step,
// then read back whichever value was successfully written first.
//
// This implementation simply panics if any file system error occurs.
// A more robust approach suited to asynchronous consensus would be
// to log the error then retry in an exponential-backoff loop.
//
func (fs *FileStore) WriteRead(step Step, v Value) (rv Value, rh Head) {

	// Serialize the proposed value
	buf, err := util.EncodeValue(v)
	if err != nil {
		panic(err.Error())
	}

	// Try to write the file, ignoring already-exists errors
	name := fmt.Sprintf("ver-%d", step)
	path := filepath.Join(fs.Path, name)
	err = util.WriteFileOnce(path, buf, 0666)
	if err != nil && !os.IsExist(err) {
		panic(err.Error())
	}

	// Read back whatever file was successfully written first there
	rbuf, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}

	// Deserialize the value read
	rv, err = util.DecodeValue(rbuf)
	if err != nil {
		panic(err.Error())
	}

	return rv, rh
}

// QSCOD calls Committed to inform us that history comh is committed,
// so we can garbage-collect entries before it in the key/value store.
// But this Store does not implement garbage-collection.
//
func (fs *FileStore) Committed(comh Head) {
	// do nothing - no garbage collection
}
