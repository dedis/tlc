// Threshold time layer.
// Controls the advancement of time by collecting a threshold number of
// (optionally witnessed) broadcast messages from each time step.
package threstime

import (
	. "github.com/dedis/tlc/go/stack/arch"
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

	tmpl	Message		// Template for messages we send
	acks	set		// Acknowledgments we've received in this step
	wits	set		// Threshold witnessed messages seen this step
	expiry	int		// Earliest step for which we maintain history
	log	[][]logEntry	// Recent messages seen by nodes at each step
}

// Per-step info each node tracks and logs about all other nodes' histories
type logEntry struct {
	saw	set		// All nodes' messages the node had seen by then
	wit	set		// Threshold witnessed messages it had seen
}


// Initialize the threshold time layer state instance for a node.
func (l *Layer) Init(stack Stack, lower Lower, upper Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper

	l.tmpl = Message{From: n.self, Step: -1}
}


// Broadcast a copy of our current message template to all nodes
func (l *Layer) broadcast() *Message {

	//println(n.self, n.tmpl.Step, "broadcast", msg, "typ", msg.Typ)
	msg := n.tmpl
	n.broadcastCausal(&msg)
	return &msg
}

// Unicast an acknowledgment of a given proposal to its sender
func (l *Layer) acknowledge(prop *Message) {

	msg := n.tmpl
	msg.Typ = Ack
	msg.Prop = prop.Seq
	n.sendCausal(prop.From, &msg)
}

// Advance to a new time step.
func (l *Layer) advance(step int) {
	//println(n.self, step, "advanceTLC",
	//	"saw", len(n.saw[n.self]), "wit", len(n.wit[n.self]))

	// Initialize our message template for new time step
	n.tmpl.Step = step	// Advance to new time step
	n.tmpl.Typ = Prop	// Raw unwitnessed proposal message initially
	n.tmpl.Ticket = rand.Int31n(MaxTicket)	// Choose a ticket

	n.acks = make(set)	// No acknowledgments received yet in this step
	n.wits = make(set)	// No threshold witnessed messages received yet

	for i := range n.peer {	// Prune ancient saw and wit set history
		n.saw[i] = n.saw[i].copy(n.save)
		n.wit[i] = n.wit[i].copy(n.save)
	}

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	upper.Advance(n.saw[n.self], n.wit[n.self])

	prop := n.broadcast()		// broadcast our raw proposal
	n.tmpl.Prop = prop.Seq		// save proposal's sequence number
	n.acks.add(prop)		// automatically self-acknowledge  it
}

// Returns the set of broadcast messages that had been seen
// by a given node by a given recent time-step.
func (l *Layer) Saw(node Node, step Step) MessageSet {
}

// Returns the set of threshold witnessed proposals
// seen by a given node by a given recent time-step.
func (l *Layer) Wit(node Node, step Step) MessageSet {
}

// The lower layer upcalls this method to notify us of a received message.
// We rely on the causality layer below to ensure that messages are
// delivered to this method in causal order.
func (l *Layer) Receive(msg *Message) {

	// Process this message according to type.
	//println(n.self, n.tmpl.Step, "receivedTLC from", msg.From,
	//	"step", msg.Step, "typ", msg.Typ)
	switch msg.Type {
	case Prop: // A raw unwitnessed proposal broadcast.
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
		prop := l.log[msg.From][msg.Prop].msg
		if prop.Type != Prop { panic("doesn't refer to a proposal!") }
		if msg.Step == l.tmpl.Step {
			l.wits.add(prop) // witnessed messages in this step
			if len(l.wits) >= Threshold {

				// Log the saw and wit sets by time-step.

				// We've met the condition to advance time.
				l.advance(n.tmpl.Step + 1)
			}
		}
	}
}

