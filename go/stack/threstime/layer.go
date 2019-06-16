// Threshold time layer.
// Controls the advancement of time by collecting a threshold number of
// (optionally witnessed) broadcast messages from each time step.
package threstime

import (
	"math/rand"

	. "github.com/dedis/tlc/go/stack/arch"
)


type Stack interface {
	Self() Node
}

type Lower interface {

	// Broadcast a message to all participating nodes.
	Broadcast(msg *Message)

	// Unicast a message to participant dest.
	Send(dest Node, msg *Message)

	// Return the message a given peer sent with a given sequence number.
	Message(peer Node, seq Seq) *Message
}

type Upper interface {

	// Notify the application of the advancement of logical time,
	// and get the application message to send at the new time-step.
	// Above the threshold time layer,
	// sends become upcalls rather than downcalls
	// because their timing is driven by TLC
	// and, in turn, network communication.
	Advance(step Step) (expire Step)

	// Deliver a (suitably witnessed) message newly-received from a peer.
//	Recv(msg arch.Message)
}

// Per-node state for the threshold time layer.
type Layer struct {
	stack	Stack
	lower	Lower
	upper	Upper

	tmpl	Message		// Template for messages we send
	acks	MessageSet	// Acknowledgments we've received in this step
	wits	MessageSet	// Threshold witnessed messages seen this step
	expiry	Step		// Earliest step for which we maintain history
	log	[][]logEntry	// Recent messages seen by nodes at each step
}

// Per-step info each node tracks and logs about all other nodes' histories
type logEntry struct {
	saw	MessageSet	// All nodes' messages the node had seen by then
	wit	MessageSet	// Threshold witnessed messages it had seen
}


// Initialize the threshold time layer state instance for a node.
func (l *Layer) Init(stack Stack, lower Lower, upper Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper

	l.tmpl = Message{From: stack.Self(), Step: -1}
}


// Broadcast a copy of our current message template to all nodes
func (l *Layer) broadcast() *Message {

	//println(n.self, n.tmpl.Step, "broadcast", msg, "typ", msg.Typ)
	msg := l.tmpl
	l.lower.Broadcast(&msg)
	return &msg
}

// Unicast an acknowledgment of a given proposal to its sender
func (l *Layer) acknowledge(prop *Message) {

	msg := l.tmpl
	msg.Type = Ack
	msg.Prop = prop.Seq
	l.lower.Send(prop.From, &msg)
}


var maxTicket int32 = 1000000	// XXX Amount of entropy in lottery tickets

// Advance to a new time step.
func (l *Layer) advance(step Step) {
	//println(n.self, step, "advanceTLC",
	//	"saw", len(n.saw[n.self]), "wit", len(n.wit[n.self]))

	// Initialize our message template for new time step
	l.tmpl.Step = step	// Advance to new time step
	l.tmpl.Type = Prop	// Raw unwitnessed proposal message initially
	l.tmpl.Ticket = Ticket(rand.Int31n(maxTicket))	// Choose a ticket

	l.acks = make(MessageSet) // No acknowledgments received yet this step
	l.wits = make(MessageSet) // No witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	newExp := l.upper.Advance(step)

	// Gradually garbage-collect old expired log entries.
	for l.expiry < newExp {
		l.log[0] = logEntry{nil, nil}
		l.log = l.log[1:]
	}

	prop := n.broadcast()		// broadcast our raw proposal
	n.tmpl.Prop = prop.Seq		// save proposal's sequence number
	n.acks.add(prop)		// automatically self-acknowledge  it
}

// Returns the set of broadcast messages that had been seen
// by a given node by a given recent time-step.
func (l *Layer) Saw(node Node, step Step) MessageSet {
	return l.log[node][step - l.expiry].saw
}

// Returns the set of threshold witnessed proposals
// seen by a given node by a given recent time-step.
func (l *Layer) Wit(node Node, step Step) MessageSet {
	return l.log[node][step - l.expiry].wit
}

// The lower layer upcalls this method to notify us of a received message.
// We rely on the causality layer below to ensure that messages are
// delivered to this method in causal order.
func (l *Layer) Receive(msg *Message, saw MessageSet) {

	// Process this message according to type.
	//println(n.self, n.tmpl.Step, "receivedTLC from", msg.From,
	//	"step", msg.Step, "typ", msg.Typ)
	switch msg.Type {
	case Prop: // A raw unwitnessed proposal broadcast.

		// Construct the set of threshold witnessed proposals
		// seen by the proposal's sender in its recent history.
		wit := make(MessageSet, len(saw))
		for m := range saw {
			if m.Type == Wit {
				prop := lower.Message(msg.From, msg.Prop)
				if prop.Type != Prop {
					panic("doesn't refer to a proposal!")
				}
				wit.Add(prop)
			}
		}

		// Record the set of [witnessed] messages this node had seen
		// by the time it advanced to this new time-step.
		if len(l.log[msg.From]) != msg.Step - l.expiry {
			panic("out of sync")
		}
		l.log[msg.From] = append(l.log[msg.From], logEntry{saw, wit})

		// If the proposal is in our current time step, acknowledge it.
		if msg.Step == l.tmpl.Step {
			//println(n.self, n.tmpl.Step, "ack", msg.From)
			l.acknowledge(msg)
		}

	case Ack: // An acknowledgment. Collect a threshold of acknowledgments.

		if msg.Prop == l.tmpl.Prop { // only if it acks our proposal
			l.acks.add(msg)
			//println(n.self, n.tmpl.Step,  "got ack", len(n.acks))
			if l.tmpl.Type == Prop && len(l.acks) >= Threshold {

				// Broadcast a threshold-witnesed certification
				l.tmpl.Type = Wit
				l.broadcast()
			}
		}

	case Wit: // A threshold-witnessed message. Collect a threshold of them.

		prop := lower.Message(msg.From, msg.Prop)
		if prop.Type != Prop { panic("doesn't refer to a proposal!") }
		// XXX don't panic; validate properly

		if msg.Step == l.tmpl.Step {
			l.wits.add(prop) // witnessed messages in this step
			if len(l.wits) >= Threshold {

				// We've met the condition to advance time.
				l.advance(n.tmpl.Step + 1)
			}
		}
	}
}

