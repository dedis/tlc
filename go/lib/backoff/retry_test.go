package backoff

import (
	"testing"
	"errors"
	"fmt"
)

func TestRetry(t *testing.T) {

	n := 0
	try := func() error {
		n++
		if n < 30 {
			return errors.New(fmt.Sprintf("test error %d", n))
		}
		return nil
	}
	Retry(try)
}

