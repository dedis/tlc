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
	g         *Group     // group this store is associated with
	lvals     string     // last value we observed in the underlying Store
	lval      core.Value // deserialized last value
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
		//println("WriteRead cancelled")
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
	for val.S > cs.lval.S {

		// Write the serialized value to the underlying CAS interface
		_, avals, err := cs.CompareAndSet(cs.g.ctx, cs.lvals, vals)
		if err != nil {
			println("CompareAndSet error", err.Error())
			return core.Value{}, err
		}

		// Deserialize the actual value we read back
		aval, err := encoding.DecodeValue([]byte(avals))
		if err != nil {
			println("decoding error", err.Error())
			return core.Value{}, err
		}

		//		println("tryWriteRead step",
		//			cs.lval.S, "w", val.S, "->", aval.S,
		//			"casver", cs.lver, "->", aver)

		if aval.S <= cs.lval.S {
			panic("CAS failed to advance TLC step!")
		}

		// Update our record of the underlying CAS version and value
		//println("update from step", cs.lval.S, "to step", aval.S)
		cs.lvals, cs.lval = avals, aval
	}

	//println("cs returning newer step", cs.lval.S)
	return cs.lval, nil
}
