// This package provides a simple file system key/value Store for QSCOD,
// with no support for garbage collection.
// It is intended only for education, testing, and experimentation,
// and not for any production use.
//
package simple

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/dedis/tlc/go/lib/backoff"
	"github.com/dedis/tlc/go/lib/fs/atomic"
	. "github.com/dedis/tlc/go/model/qscod/core"
	"github.com/dedis/tlc/go/model/qscod/encoding"
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
func (fs *FileStore) WriteRead(v Value) (rv Value) {

	try := func() (err error) {

		// Serialize the proposed value
		buf, err := encoding.EncodeValue(v)
		if err != nil {
			return err
		}

		// Try to write the file, ignoring already-exists errors
		name := fmt.Sprintf("ver-%d", v.P.Step)
		path := filepath.Join(fs.Path, name)
		err = atomic.WriteFileOnce(path, buf, 0666)
		if err != nil && !os.IsExist(err) {
			return err
		}

		// Read back whatever file was successfully written first there
		rbuf, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		// Deserialize the value read
		rv, err = encoding.DecodeValue(rbuf)
		if err != nil {
			return err
		}

		return nil
	}

	backoff.Retry(context.Background(), try)
	return rv
}

// QSCOD calls Committed to inform us that history comh is committed,
// so we can garbage-collect entries before it in the key/value store.
// But this Store does not implement garbage-collection.
//
func (fs *FileStore) Committed(comh Head) {
	// do nothing - no garbage collection
}
