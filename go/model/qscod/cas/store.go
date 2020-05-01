package cas

import (
	"github.com/dedis/tlc/go/lib/backoff"
	"github.com/dedis/tlc/go/lib/cas"
	"github.com/dedis/tlc/go/model/qscod/core"
	"github.com/dedis/tlc/go/model/qscod/encoding"
)

// coreStore implements QSCOD core's native Store interface
// based on a cas.Store interface.
type coreStore struct {
	cas.Store            // underlying CAS state store
	g         *group     // group this store is associated with
	lver      int64      // last underlying CAS version we wrote and read back
	lval      core.Value // last value we read with lver
}

func (cs *coreStore) WriteRead(v core.Value) (rv core.Value) {

	try := func() (err error) {
		rv, err = cs.tryWriteRead(v)
		return err
	}

	// Try to perform the atomic operation until it succeeds
	// or until the group's context gets cancelled.
	err := backoff.Retry(cs.g.ctx, try)
	if err != nil && cs.g.ctx.Err() != nil {

		// The group's context got cancelled,
		// so just silently return nil Values
		// until the consensus worker threads catch up and terminate.
		return core.Value{}
	}
	if err != nil {
		panic("backoff.Retry inexplicably gave up: " + err.Error())
	}
	return rv
}

func (cs *coreStore) tryWriteRead(val core.Value) (core.Value, error) {

	// Serialize the proposed value
	valb, err := encoding.EncodeValue(val)
	if err != nil {
		println("encoding error", err.Error())
		return core.Value{}, err
	}
	vals := string(valb)

	// Try to set the underlying CAS register to the proposed value
	// only as long as doing so would strictly increase its TLC step
	for val.P.Step > cs.lval.P.Step {

		// Write the serialized value to the underlying CAS interface
		aver, avals, err := cs.CheckAndSet(cs.g.ctx, cs.lver, vals)
		if err != nil && err != cas.Changed {
			println("CheckAndSet error", err.Error())
			return core.Value{}, err
		}

		// Deserialize the actual value we read back
		aval, err := encoding.DecodeValue([]byte(avals))
		if err != nil {
			println("decoding error", err.Error())
			return core.Value{}, err
		}

		//		println("tryWriteRead step",
		//			cs.lval.P.Step, "w", val.P.Step, "->", aval.P.Step,
		//			"casver", cs.lver, "->", aver)

		if aval.P.Step <= cs.lval.P.Step {
			panic("CAS failed to advance TLC step!")
		}

		// Update our record of the underlying CAS version and value
		cs.lver, cs.lval = aver, aval
	}

	return cs.lval, nil
}
