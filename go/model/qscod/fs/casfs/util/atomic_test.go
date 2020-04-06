package util

import (
	"os"
	"sync"
	"testing"
	"io/ioutil"
)

func TestWriteFileOnce(t *testing.T) {

	filename := "testfile.tmp"
	var wg sync.WaitGroup
	writer := func(i int) {
		b := make([]byte, i) // create i-length file filled with i's
		for j := range b {
			b[j] = byte(i)
		}
		err := WriteFileOnce(filename, b, 0644)
		if err != nil && !os.IsExist(err) {
			t.Error("WriteFileOnce:", err)
		}

		// Now try to read the file that got written
		b, err = ioutil.ReadFile(filename)
		if err != nil {
			t.Error("ReadFile", err)
		}

		// Check that what we read back is valid
		i = len(b)
		if i == 0 {
			t.Error("zero-length file shouldn't be possible")
		}
		for j := range b {
			if b[j] != byte(i) {
				t.Error("read file has wrong byte at", j)
			}
		}

		wg.Done()
	}

	// Test with increasing numbers of threads
	for n := 1; n <= 128; n *= 2 {

		t.Log(n, "threads")
		for i := 1; i <= n; i++ {
			wg.Add(1)
			go writer(i)
		}
		wg.Wait()
	}

	os.Remove(filename)
}
