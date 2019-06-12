package failstop

import (
	"tlc"
	"time"
)

// Trivial internal realtime layer that merely timestamps incoming messages
// without providing any protection against invalid timestamps
// or badly out-of-sync clocks.
type realtimeLayer struct {
	stack *Stack
	lower tlc.Lower
	upper tlc.Upper
}

func (l *realtimeLayer) Init(stack *Stack, lower tlc.Lower, upper tlc.Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper
}

func (l *realtimeLayer) Send(msg tlc.Message) {
	s.lower.Send(msg)
}

func (l *realtimeLayer) Obsolete(min tlc.Step) {
	l.lower.Obsolete(min)
}

func (l *realtimeLayer) Recv(msg tlc.Message) {
	if msg.Timestamp().IsZero() {
		msg.SetTimestamp(l.stack.Now())
	}
	l.upper.Recv(msg)
}

