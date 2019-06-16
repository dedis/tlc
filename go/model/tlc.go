package model

import (
	"math/rand"
)

// Create a copy of our message template for transmission.
// Also duplicates the slices within the template that are mutable.
func (n *Node) copyTemplate() *Message {
	msg := n.tmpl			// copy the message template
	msg.saw = msg.saw.copy(0)	// take snapshot of mutable saw set
	msg.wit = msg.wit.copy(0)	// take snapshot of mutable wit set
	return &msg
}

// Broadcast a copy of our current message template to all nodes
func (n *Node) broadcastTLC() *Message {
	msg := n.copyTemplate()
	for _, dest := range All {
		dest.comm <- msg
	}
	return msg
}

// Unicast an acknowledgment of a given proposal to its sender
func (n *Node) acknowledgeTLC(prop *Message) {
	msg := n.copyTemplate()
	msg.typ = Ack
	msg.prop = prop
	All[prop.from].comm <- msg
}

// Advance to a new time step.
func (n *Node) advanceTLC(step int) {

	// Initialize our message template for new time step
	n.tmpl.step = step	// Advance to new time step
	n.tmpl.typ = Prop	// Broadcast raw unwitnessed message initially
	n.tmpl.prop = nil	// Filled in below on raw proposal broadcast
	n.tmpl.ticket = rand.Int31n(MaxTicket)	// Choose a ticket
	n.tmpl.saw = n.tmpl.saw.copy(n.save)	// prune ancient history
	n.tmpl.wit = n.tmpl.wit.copy(n.save)

	n.acks = make(set)	// No acknowledgments received yet in this step
	n.wits = make(set)	// No threshold witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC(n.tmpl.saw, n.tmpl.wit)

	n.tmpl.prop = n.broadcastTLC()	// broadcast our raw proposal
}

// The network layer below calls this on receipt of a message from another node.
func (n *Node) receiveTLC(msg *Message) {

	// Process broadcast messages in causal order and only once each,
	// ignoring messages already processed or before recorded history.
	// This will catch us up at least to the same step as msg.
	if n.tmpl.saw.has(msg) || msg.step < n.save { return }
	for prior := range msg.saw {
		n.receiveTLC(prior) // First process causally prior messages
	}
	if n.tmpl.saw.has(msg) || msg.step < n.save {
		return		// discard messages already seen or obsolete
	}
	n.tmpl.saw.add(msg)	// record that we've seen this message

	// Now process this message according to type.
	switch msg.typ {
	case Prop: // A raw unwitnessed proposal broadcast.
		if msg.step == n.tmpl.step { // Acknowledge only in same step.
			n.acknowledgeTLC(msg)
		}

	case Ack: // An acknowledgment. Collect a threshold of acknowledgments.
		if msg.prop == n.tmpl.prop { // only if it acks our proposal
			n.acks.add(msg)
			if n.tmpl.typ == Prop && len(n.acks) >= Threshold {
				n.tmpl.typ = Wit // threshold-witnessed cert
				n.broadcastTLC()
			}
		}

	case Wit: // A threshold-witnessed cert. Collect a threshold of them.
		n.tmpl.wit.add(msg.prop) // collect all witnessed proposals
		if msg.step == n.tmpl.step {
			n.wits.add(msg.prop) // witnessed messages in this step
			if len(n.wits) >= Threshold {
				n.advanceTLC(n.tmpl.step + 1) // advance time
			}
		}
	}
}

