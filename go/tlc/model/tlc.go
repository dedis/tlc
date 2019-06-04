package model

import (
	"math/rand"
)

// Create a copy of our message template for transmission.
// Also duplicates the slices within the template that are mutable.
func (n *Node) copyTemplate() *Message {

	msg := n.tmpl			// copy the message template
	msg.saw = msg.saw.copy()	// take snapshot of mutable set
	msg.wit = msg.wit.copy()	// take snapshot of mutable set
	return &msg
}

// Broadcast a copy of our current message template to all nodes
func (n *Node) broadcast() {

	msg := n.copyTemplate()
	for _, dest := range All {
		dest.comm <- msg
	}
}

// Unicast an acknowledgment to a given destination node
func (n *Node) acknowledge(dest int) {

	msg := n.copyTemplate()
	msg.typ = Ack
	All[dest].comm <- msg
}

// Advance to a new time step.
func (n *Node) advanceTLC(step int, lastsaw, lastwit set) {

	// Initialize our message template for new time step
	n.tmpl.step = step	// Advance to new time step
	n.tmpl.typ = Raw	// Raw unwitnessed message initially
	n.tmpl.ticket = rand.Int31n(MaxTicket) // Choose a ticket
	n.tmpl.saw = make(set)	// No messages ssen yet this step
	n.tmpl.wit = make(set)	// No threshold witnessed messages yet
	n.tmpl.lastsaw = lastsaw
	n.tmpl.lastwit = lastwit

	n.acks = make(set)	// No acknowledgments received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC()

	n.broadcast()		// broadcast our raw unwitnessed message
}

func (n *Node) receiveTLC(msg *Message) {

	// First handle messages for time steps other than the one we expect
	if msg.step < n.tmpl.step {
		return	// just ignore obsolete messages from old time steps
	} else if msg.step > n.tmpl.step {
		// Recursively receiving the messages that enabled msg's sender
		// to advance to this newer time step should catch us up to it.
		n.receivePriorTLC(msg.lastsaw)
		if n.tmpl.step != msg.step { panic("failed to catch up!") }
	}

	// Process messages in this step in causal order and only once each.
	if n.tmpl.saw.has(msg) {
		return	// ignore messages from this step we already processed
	}
	n.receivePriorTLC(msg.saw)	// handle prior messages from this step

	// Now process this message according to type.
	switch msg.typ {
	case Raw: // A raw unwitnessed message. Just record and acknowledge it.
		n.tmpl.saw.add(msg)
		n.acknowledge(msg.sender)

	case Ack: // An acknowledgment. Collect a threshold of acknowledgments.
		n.acks.add(msg)
		if n.tmpl.typ == Raw && len(n.acks) >= Threshold {

			// Broadcast our newly threshold-witnesed message
			n.tmpl.typ = Wit
			n.broadcast()
		}

	case Wit: // A threshold-witnessed message. Collect a threshold of them.
		n.tmpl.saw.add(msg)
		n.tmpl.wit.add(msg)
		if len(n.tmpl.wit) >= Threshold {

			// We've met the threshold condition to advance time.
			n.saw = append(n.saw, n.tmpl.saw)
			n.wit = append(n.wit, n.tmpl.wit)
			n.advanceTLC(n.tmpl.step + 1, n.tmpl.saw, n.tmpl.wit)
		}
	}
}

// Recursively receive all prior broadcast messages in a set.
func (n *Node) receivePriorTLC(msgset set) {
	for prior := range msgset {
		n.receiveTLC(prior)
	}
}

