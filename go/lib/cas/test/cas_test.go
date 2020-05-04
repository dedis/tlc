package test

import (
	"testing"

	"github.com/dedis/tlc/go/lib/cas"
)

// Test the Client with a trivial in-memory key/value Store implementation.
func TestRegister(t *testing.T) {
	Stores(t, 100, 100000, &cas.Register{})
}
