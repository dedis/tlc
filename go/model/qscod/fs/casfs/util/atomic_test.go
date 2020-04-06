package util

import (
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"
)

func TestWriteFileOnce(t *testing.T) {

	filename := "testfile.tmp"
	var wg sync.WaitGroup
	writer := func(i int) {

		// Sleep a small random duration to jitter the test
		time.Sleep(time.Duration(rand.Int63n(int64(time.Microsecond))))
		println("thread",i,"writing")

		// Try to write the file
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
		println("thread",i,"read",len(b))
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

		println("\ntesting", n, "threads")
		for i := 1; i <= n; i++ {
			wg.Add(1)
			go writer(i)
		}
		wg.Wait()

		os.Remove(filename)
	}
}
