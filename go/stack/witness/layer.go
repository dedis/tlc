// Threshold witnessing (and optionally witness cosigning) layer.
package witness

import (
	"tlc"
	"tlc/arch"
)


type Lower interface {
	Send(msg arch.Message)
}

type Upper interface {

	// Deliver a (suitably witnessed) message newly-received from a peer.
	Recv(msg arch.Message)
}


type Layer struct {
	stack arch.Stack
	lower arch.Lower
	upper arch.Upper

	n, tw int
}

func (l *Layer) Init(stack arch.Stack, lower arch.Lower, upper arch.Upper) {

	n, _, tw := stack.Dim()
	l.n = n
	l.tw = tw

	l.stack = stack
	l.lower = lower
	l.upper = upper

	// Create the received-messages vector
	l.msg = make([]arch.Message, n)
	l.nmsg = 0
}

func (l *Layer) Recv(msg arch.Message) {

	if (bare unwitnessed message) {
		if (obsolete timestep) {
			reply with catch-up info
		}

		witness the message and reply with cosig

	} else if witness response to our message {

		collect
		if (tw threshold met) {
			attach certification to message
			l.lower.Send(msg)
		}

	} else if (threshold witnessing certification) {

		attach certification to message

		l.upper.Recv(msg)
	}
}

