package model

// Create a copy of our message template for transmission.
// Also duplicates the slices within the template that are mutable.
func (n *Node) newMsg() *Message {
	msg := n.Message                      // copy the message template
	msg.QSC = append([]Round{}, n.QSC...) // copy QSC state slice
	return &msg
}

// Broadcast a copy of our current message template to all nodes
func (n *Node) broadcastTLC() {
	msg := n.newMsg()
	for i := 0; i < n.nnode; i++ {
		n.send(i, msg)
	}
}

// Advance to a new time step.
func (n *Node) advanceTLC(step int) {

	// Initialize message template with a proposal for the new time step
	n.Step = step // Advance to new time step
	n.Type = Raw  // Broadcast raw proposal first
	n.acks = 0    // No acknowledgments received yet in this step
	n.wits = 0    // No threshold witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC()

	n.broadcastTLC() // broadcast our raw proposal
}

// The network layer below calls this on receipt of a message from another node.
func (n *Node) receiveTLC(msg *Message) {

	for msg.Step > n.Step { // msg is ahead: virally catch up to it
		n.advanceTLC(n.Step + 1)
	}

	// Merge in received QSC state for rounds still in our pipeline
	if msg.Step+3 > n.Step {
		mergeQSC(n.QSC[n.Step+1:], msg.QSC[n.Step+1:])
	}

	// Now process this message according to type, but only in same step.
	if msg.Step == n.Step {
		switch msg.Type {
		case Raw: // Acknowledge unwitnessed proposals.
			ack := n.newMsg()
			ack.Type = Ack
			n.send(msg.From, ack)

		case Ack: // Collect a threshold of acknowledgments.
			n.acks++
			if n.Type == Raw && n.acks >= n.thres {
				n.Type = Wit // Prop now threshold witnessed
				n.witnessedQSC()
				n.broadcastTLC()
			}

		case Wit: // Collect a threshold of threshold witnessed messages
			n.wits++ // witnessed messages in this step
			if n.wits >= n.thres {
				n.advanceTLC(n.Step + 1) // tick the clock
			}
		}
	}
}
