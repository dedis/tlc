package backoff

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
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
	Retry(context.Background(), try)
}

func TestTimeout(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	try := func() error {
		return errors.New("haha, never going to succeed")
	}
	if err := Retry(ctx, try); err != context.DeadlineExceeded {
		t.Errorf("got wrong error from Retry: %v", err.Error())
	}

	// Now test with an already-cancelled context
	try = func() error {
		panic("shouldn't get here!")
	}
	if err := Retry(ctx, try); err != context.DeadlineExceeded {
		t.Errorf("got wrong error from Retry: %v", err.Error())
	}

	// for good measure
	cancel()
}
