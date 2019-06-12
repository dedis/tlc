package logging

import (
	"tlc"
)


type event struct {
	seq Seq			// Local event sequence number
	step Step		// Associated logical time step
	msg tlc.Message		// Message received or produced at this event
}


// Interface representing a log-entry signing scheme we can use.
type Signer {

	// Sign a log entry.
	Sign(e *entry) []byte
}


// A logging.Layer instance represents a log containing
// all of the non-deterministic events affecting a node.
// Unlike other layers, this layer is designed to be instantiated
// multiple times in the implementations of each node:
// once representing the node itself and its own log,
// the others reflecting the logs of *other* nodes
// for simulation and Byzantine behavior protection via PeerReview.
type Layer struct {
	stack arch.Stack
	lower arch.Lower
	upper arch.Upper

	node int;		// Node number this log is for
	seq Seq			// First unassigned local event number
	minStep tlc.Step	// Minimum not-yet-obsolete TLC time-step
	log [][]*event		// Event log indexed by step and event number
}

func (l *Layer) Init(stack arch.Stack, lower arch.Lower, upper arch.Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper

	...
}

func (l *Layer) Recv(msg tlc.Message) {

	step := msg.Step()
	if step < l.minStep {
		return	// Discard messages from obsolete time-steps
	}
	XXX

	// Grow the step-indexed log slice as necessary
	stepIdx := l.seq - l.minStep
	while stepIdx >= len(l.log) {
		l.log = append(l.log, nil)
	}

	// Log the received message.
	e := event{l.seq, step, msg}
	l.log[stepIdx] = append(l.log[stepIdx], &e)

	l.seq++
}

func (l *Layer) Obsolete(min tlc.Step) {

	// Garbage collect log slices for time-steps earlier than minStep
	while l.minStep < min {
		if len(l.log) > 0 {
			l.log[0] = nil
			l.log = l.log[1:]
			l.minStep++
		} else {
			// catch up instantly
			l.minStep = min
		}
	}

	// Pass on the signal to lower layers
	l.lower.Obsolete(min)
}

