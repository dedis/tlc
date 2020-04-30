// This package converts errors into time delays via random exponential backoff.
// It is designed to be extremely simple to use but robust and automatic.
//
package backoff

import (
	"context"
	"log"
	"math/rand"
	"time"
)

// Retry calls try() repeatedly until it returns without an error,
// with the default exponential backoff configuration.
//
// By default, Retry continues to try forever until it succeeds.
// The caller may pass a cancelable context in the ctx parameter, however,
// in case Retry will give up calling try when the context is cancelled.
// If the context was already cancelled on the call to Retry,
// then Retry returns ctx.Err() immediately without calling try.
//
func Retry(ctx context.Context, try func() error) error {
	return Config{}.Retry(ctx, try)
}

// Config represents configuration parameters for exponential backoff.
// To use, initialize a Config structure with the desired parameters
// and then call Config.Retry().
//
// Report, if non-nil, is a function called by Retry to report errors
// in an appropriate fashion specific to the application.
// If nil, Retry reports errors via log.Println by default.
// Report may also return a non-nil error to abort the Retry loop if it
// determines that the detected error is permanent and waiting will not help.
//
type Config struct {
	Report  func(error) error // Function to report errors
	MaxWait time.Duration     // Maximum backoff wait period

	mayGrow struct{} // Ensure Config remains extensible
}

func defaultReport(err error) error {
	log.Println(err.Error())
	return nil
}

// Retry calls try() repeatedly until it returns without an error,
// using exponential backoff configuration c.
func (c Config) Retry(ctx context.Context, try func() error) error {

	// Make sure we have a valid error reporter
	if c.Report == nil {
		c.Report = defaultReport
	}

	// Return immediately if ctx was already cancelled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	backoff := time.Duration(1) // minimum backoff duration
	for {
		before := time.Now()
		err := try()
		if err == nil { // success
			return nil
		}
		elapsed := time.Since(before)

		// Report the error as appropriate
		err = c.Report(err)
		if err != nil {
			return err // abort the retry loop
		}

		// Wait for an exponentially-growing random backoff period,
		// with the duration of each operation attempt as the minimum
		if backoff <= elapsed {
			backoff = elapsed
		}
		backoff += time.Duration(rand.Int63n(int64(backoff)))
		if c.MaxWait > 0 && backoff > c.MaxWait {
			backoff = c.MaxWait
		}

		// Wait for either the backoff timer or a cancel signal.
		t := time.NewTimer(backoff)
		select {
		case <-t.C: // Backoff timer expired
			continue

		case <-ctx.Done(): // Our context got cancelled
			t.Stop()
			return ctx.Err()
		}
	}
}
