// Threshold time layer.
// Controls the advancement of time by collecting a threshold number of
// (optionally witnessed) broadcast messages from each time step.
package threstime

import (
	"math/big"

	"tlc"
	"tlc/arch"
)


type Lower interface {
	Send(msg arch.Message)
}

type Upper interface {

	// Notify the application of the advancement of logical time,
	// and get the application message to send at the new time-step.
	// Above the threshold time layer,
	// sends become upcalls rather than downcalls
	// because their timing is driven by TLC
	// and, in turn, network communication.
	Step(step arch.Step) arch.Message

	// Deliver a (suitably witnessed) message newly-received from a peer.
	Recv(msg arch.Message)
}


type Layer struct {
	stack arch.Stack
	lower arch.Lower
	upper arch.Upper

	n, tm int
	step arch.step
	msg []arch.Message
	nmsg int

	view []View
}

func (l *Layer) Init(stack arch.Stack, lower arch.Lower, upper arch.Upper) {

	n, tm, _ := stack.Dim()
	l.n = n
	l.tm = tm

	l.stack = stack
	l.lower = lower
	l.upper = upper

	// Create the received-messages vector
	l.msg = make([]arch.Message, n)
	l.nmsg = 0

	// Launch the protocol by sending our message for timestamp 0.
	// Would it be useful to split this into a separate Start() function?
	l.step = 0
	l.send()
}

func (l *Layer) Recv(msg arch.Message) {

	step := msg.Step()
	if step < l.step {
		l.stack.Warnf("threstime: dropping obsolete message")
		return
	}
	if step > l.step {
		l.stack.Warnf("threstime: received message out of causal order")
		// or might this happen if we need to catch up after lagging?
		return
	}

	// Record the received message
	i = msg.Sender()
	if l.msg[i] != nil {
		l.stack.Warnf("threstime: duplicate message received")
		return
	}
	l.msg[i] = msg
	l.nmsg++

	// Pass the received message on up the stack.
	l.upper.Recv(msg)

	// Advance time when we meet the message threshold
	if (l.nmsg >= l.tm) {

		// Clear the received-messages record
		for i := range l.msg {
			l.msg[i] = nil
		}
		l.nmsg = 0

		// Increment step count and send the next time-step's broadcast
		l.step++
		l.send()
	}
}

func (l *Layer) send() {
	msg := l.upper.Send(l.step)
	l.lower.Send(msg)
}



// A View represents a view of threshold-clocked history history
// as recorded by a particular node at a particular time-step.
type View struct {
	step arch.Step
	peer int
	msg arch.Message
	seen *bitVec
	witn *bitVec
}

// Return the time-step number at which this View was established.
func (v *View) Step() arch.Step  {
	return v.step
}

// Return the participant number who established this View.
func (v *View) Peer() int {
	return v.peer
}

// Return the Message that participant broadcast at this time-step.
func (v *View) Message() arch.Message {
	return v.msg
}

// Return the bit-vector of nodes whose messages
// from the immediately prior time-step were seen in this view.
// The returned BitVec must not be modified.
// func (v *View) Seen() *BitVec

// Return true if node i's message from the last time-step is within this view.
func (v *View) Seen(i int) bool {
	return v.seen.Bit(i)
}

// Return the bit-vector of nodes whose messages
// from the immediately prior time-step were witness cosigned
// and used in advancing the logical clock to the current time-step.
// This is always a subset of the nodes returned by Seen().
// The returned BitVec must not be modified.
// func (v *View) Witnessed() *BitVec

// Return true if node i's message from the last time-step
// was threshold-witnessed and used in advancing time tothe current step
// within this view.
func (v *View) Witnessed(i  int) bool {
	return v.witn.Bit(i)
}

// If the specified prior time-step and node number's message
// is within the history defined by this View and not obsolete,
// then returns a View representing that prior event.
// Otherwise, returns nil.
// func (v *View) Prior(step arch.Step, node int)




// BitRec represents a bit vector, implemented via math/big.Int.
type bitVec struct {
	Int big.Int
}

func (v *bitVec) Bit(i int) bool {
	return v.Int.Bit(i) != 0
}

func (v bitVec) SetBit(i int, b bool) {
	if b {
		v.Int.SetBit(&v.Int, i, 1)
	} else {
		v.Int.SetBit(&v.Int, i, 0)
	}
}

