// This package converts errors into time delays via random exponential backoff.
// It is designed to be extremely simple to use but robust and automatic.
//
package backoff

import (
	"time"
	"log"
	"math/rand"
)

// Retry calls try() repeatedly until it returns without an error,
// with the default exponential backoff configuration.
func Retry(try func() error) {
	Config{}.Retry(try)
}

// Config represents configuration parameters for exponential backoff.
// To use, initialize a Config structure with the desired parameters
// and then call Config.Retry().
//
// Report, if non-nil, is a function called by Retry to report errors
// in an appropriate fashion specific to the application.
// If nil, Retry reports errors via log.Println by default.
// Report may also call panic() to abort the Retry loop and unwind the stack
// if it determines that a detected error is permanent and waiting cannot help.
//
type Config struct {
	Report func(error)	// Function to report errors
	MaxWait time.Duration	// Maximum backoff wait period

	mayGrow struct{}	// Ensure Config remains extensible
}

// Retry calls try() repeatedly until it returns without an error,
// with exponential backoff configuration c.
func (c Config) Retry(try func() error) {

	if c.Report == nil {	// Default error reporter
		c.Report = func(err error) { log.Println(err.Error()) }
	}

	backoff := time.Duration(1)	// minimum backoff duration
	for {
		before := time.Now()
		err := try()
		if err == nil {		// success
			return
		}
		elapsed := time.Since(before)

		// Report the error as appropriate
		c.Report(err)

		// Wait for an exponentially-growing random backoff period,
		// with the duration of each operation attempt as the minimum
		if backoff <= elapsed {
			backoff = elapsed
		}
		backoff += time.Duration(rand.Int63n(int64(backoff)))
		if c.MaxWait > 0 && backoff > c.MaxWait {
			backoff = c.MaxWait
		}
		time.Sleep(backoff)
	}
}

