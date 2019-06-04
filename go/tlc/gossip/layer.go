package gossip

import (
	"tlc"
	"tlc/arch"
)


type Layer struct {
	stack arch.Stack
	lower arch.Lower
	upper arch.Upper

	mat []stack.Vec		// Our current matrix clock
}

func (l *Layer) Init(stack arch.Stack, lower arch.Lower, upper arch.Upper) {

	l.stack = stack
	l.lower = lower
	l.upper = upper

	N, _, _ := l.stack.Dim()
	l.mat := make(Mat, N)
}

func (l *Layer) Send(msg arch.Message) {

	// Pass on the message to the next lower layer
	l.lower.Send(msg)
}

func (l *Layer) Recv(msg arch.Message) {

	// Received a vector time notification? If so, update our matrix clock.
	if v := msg.Vec(); v != nil {
		mv := l.mat[msg.Sender()]
		mv.Max(mv, v)
	}

	XXX record which immediate peer we received the message from

	XXX Schedule sending (perhaps delayed) vector time updates to peers

	l.upper.Recv(msg)
}


