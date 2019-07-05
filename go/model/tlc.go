package model

// Create a copy of our message template for transmission.
// Also duplicates the slices within the template that are mutable.
func (n *Node) newMsg() *Message {
	msg := n.msg             // copy the message template
	msg.qsc = append([]Round{}, n.msg.qsc...) // copy QSC state slice
	return &msg
}

// Broadcast a copy of our current message template to all nodes
func (n *Node) broadcastTLC() {
	msg := n.newMsg()
	for _, dest := range All {
		dest.comm <- msg
	}
}

// Unicast an acknowledgment of a given proposal to its sender
func (n *Node) acknowledgeTLC(prop *Message) {
	msg := n.newMsg()
	msg.typ = Ack
	msg.prop = prop.from // XXX
	All[prop.from].comm <- msg
}

// Advance to a new time step.
func (n *Node) advanceTLC(step int) {

	// Initialize message template with a proposal for the new time step
	n.msg.step = step                     // Advance to new time step
	n.msg.typ = Raw                       // Broadcast raw proposal first

	n.acks = make(set) // No acknowledgments received yet in this step
	n.wits = make(set) // No threshold witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC()

	n.broadcastTLC() // broadcast our raw proposal
}

// The network layer below calls this on receipt of a message from another node.
func (n *Node) receiveTLC(msg *Message) {

	if msg.step > n.msg.step {	// msg is ahead: virally catch up to it
		if len(n.msg.qsc) != n.msg.step+3+1 { panic("XXX") }
		for msg.step > n.msg.step {
			n.advanceTLC(n.msg.step + 1)
		}
	}
	if msg.step + 3 <= n.msg.step {	// msg is too far behind to be useful
		return
	}

	// Merge in received QSC state for rounds still in our pipeline
	mergeQSC(n.msg.qsc[n.msg.step:], msg.qsc[n.msg.step:])

	// Now process this message according to type, but only in same step.
	if msg.step == n.msg.step {
		switch msg.typ {
		case Raw: // A raw unwitnessed proposal broadcast.
			n.acknowledgeTLC(msg)

		case Ack: // Collect a threshold of acknowledgments.
			if msg.prop != n.msg.from { panic("XXX") }
			if n.acks.has(msg) { panic("XXX") }
			n.acks.add(msg)
			if n.msg.typ == Raw && len(n.acks) >= Threshold {
				n.msg.typ = Wit // Prop now threshold witnessed
				n.witnessedQSC()
				n.broadcastTLC()
			}

		case Wit: // Collect a threshold of threshold witnessed messages
			if n.wits.has(msg) { panic("XXX") }
			n.wits.add(msg) // witnessed messages in this step
			if len(n.wits) >= Threshold {
				n.advanceTLC(n.msg.step + 1) // tick the clock
			}
		}
	}
}
