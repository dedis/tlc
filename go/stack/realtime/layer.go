package realtime

import (
	"time"
	"tlc"
)

// Object encapsulating the functionality of this layer
type Layer struct {
	stack tlc.Stack
	lower tlc.Lower
	upper tlc.Upper

	min tlc.Step
	timers [][]tlc.Timer
}

func (l *Layer) Init(stack tlc.Stack, lower tlc.Lower, upper tlc.Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper
}

func (l *Layer) after(ts time.Time, step tlc.Step, deliver func()) {

	now := l.stack.Now()
	if ts.IsZero() || !ts.After(now) {

		// Deliver the message immediately.
		deliver()
		return
	}

	// Don't deliver the message immediately;
	// instead schedule a delivery callback for appropriate time.
	timer := l.stack.At(ts, deliver)

	// Keep the timer so we can stop stop and garbage-collect it
	// later if the associated logical time-step becomes obsolete
	// before the scheduled time arrives according to our clock.
	if step < l.min {
		l.stack.Warnf("tlc/realtime: asked to schedule message delivery for obsolete time-step")
		return
	}
	idx = step - l.min

	// Associate the timer with the appropriate time-step
	while len(l.timers) <= idx {
		l.timers = append(l.timers, nil)
	}
	l.timers[idx] = append(l.timers[idx], timer)
}

// The upper layer calls this method to broadcast a message.
func (l *Layer) Send(msg Message) {

	func deliver() {
		l.lower.Send(msg)
	}

	l.after(msg.Timestamp(), msg.Step(), deliver)
}

func (l *realtimeLayer) Obsolete(min tlc.Step) {

	// Cancel all timers associated with now-obsolete time-steps
	if min < l.min {
		l.stack.Warnf("tlc/realtime: non-monotonic obsoletion")
	}
	while min > l.min {
		if len(l.timers) > 0 {
			for _, timer := range l.timers[0] {
				timer.Stop()
			}
			l.timers[0] = nil
			l.timers = l.timers[1:]
		}
		l.min++
	}

	// Pass on the signal to lower layers
	l.lower.Obsolete(min)
}

// This method gets upcalled from the lower layer when a message is received.
func (l *Layer) Recv(msg Message) {

	func deliver() {
		l.upper.Recv(msg)
	}

	// Timestamp any received message that is not already timestamped.
	ts := msg.Timestamp()
	if ts.IsZero() {
		ts = l.stack.Now()
		msg.SetTimestamp(ts)
	}

	// Delay the received message if necessary until
	// the timestamp it contains has passed according to our wall clock.
	l.after(msg.Timestamp(), msg.Step(), deliver)
}

