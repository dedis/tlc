package model

// Create a copy of our message template for transmission.
// Sends QSC state only for the rounds still in our window.
func (n *Node) newMsg() *Message {
	msg := n.Message                               // copy the message template
	msg.QSC = append([]Round{}, n.QSC[n.Step:]...) // active QSC state
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
// This function assumes that peer-to-peer connections are ordered and reliable.
func (n *Node) receiveTLC(msg *Message) {

	// Process only messages from the current or next time step.
	// Since we receive messages from a given peer in order,
	// a message we receive can be at most one step ahead of ours.
	// We could accept and merge in information from older messages,
	// but it's safe and simpler just to ignore old messages.
	if msg.Step >= n.Step {

		// If msg is ahead of us, then virally catch up to it
		if msg.Step > n.Step {
			n.advanceTLC(n.Step + 1)
		}

		// Merge in received QSC state for rounds still in our pipeline
		mergeQSC(n.QSC[msg.Step:], msg.QSC)

		// Now process this message according to type.
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
