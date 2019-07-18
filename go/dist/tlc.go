package dist

// Create a copy of our message template for transmission.
// Also duplicates the slices within the template that are mutable.
func (n *Node) newMsg() *Message {
	msg := n.Message                      // copy the message template
	msg.qsc = append([]Round{}, n.qsc...) // copy QSC state slice
	return &msg
}

// Advance to a new time step.
func (n *Node) advanceTLC(step int) {

	// Initialize message template with a proposal for the new time step
	n.step = step // Advance to new time step
	n.typ = Raw   // Broadcast raw proposal first
	n.acks = 0    // No acknowledgments received yet in this step
	n.wits = 0    // No threshold witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC()

	n.Broadcast(n.newMsg()) // broadcast our raw proposal
}

// The network layer below calls this on receipt of a message from another node.
func (n *Node) receiveTLC(msg *Message) {

	for msg.step > n.step { // msg is ahead: virally catch up to it
		mergeQSC(n.qsc[n.step:], msg.qsc[n.step:n.step+3+1])
		n.advanceTLC(n.step + 1)
	}

	// Merge in received QSC state for rounds still in our pipeline.
	// But don't merge QSC state for the round that just ended this step.
	if msg.step+3 > n.step {
		mergeQSC(n.qsc[n.step+1:], msg.qsc[n.step+1:])
	}

	// Now process this message according to type, but only in same step.
	if msg.step == n.step {
		switch msg.typ {
		case Raw: // Acknowledge unwitnessed proposals.
			ack := n.newMsg()
			ack.typ = Ack
			n.peer[msg.from].Send(ack)

		case Ack: // Collect a threshold of acknowledgments.
			n.acks++
			if n.typ == Raw && n.acks >= n.thres {
				n.typ = Wit // Prop now threshold witnessed
				n.witnessedQSC()
				n.Broadcast(n.newMsg())
			}

		case Wit: // Collect a threshold of threshold witnessed messages
			n.wits++ // witnessed messages in this step
			if n.wits >= n.thres {
				n.advanceTLC(n.step + 1) // tick the clock
			}
		}
	}
}
