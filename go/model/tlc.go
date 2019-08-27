package model

// Create a copy of our message template for transmission.
// Sends QSC state only for the rounds still in our window.
func (n *Node) newMsg() *Message {
	msg := n.m                                         // copy template
	msg.QSC = append([]Round{}, n.m.QSC[n.m.Step:]...) // active QSC state
	return &msg
}

// Broadcast a copy of our current message template to all nodes
func (n *Node) broadcastTLC() {
	msg := n.newMsg()
	for i := 0; i < n.nnode; i++ {
		n.send(i, msg)
	}
}

// Advance to the next TLC time step.
//
// The client must invoke this function once after calling NewNode
// to launch the protocol and broadcast the message for TLC time-step zero.
// Thereafter, TLC advances time automatically based on network communication.
//
func (n *Node) Advance() {

	// Initialize message template with a proposal for the new time step
	n.m.Step++     // Advance to next time step
	n.m.Type = Raw // Broadcast raw proposal first
	n.acks = 0     // No acknowledgments received yet in this step
	n.wits = 0     // No threshold witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC()

	n.broadcastTLC() // broadcast our raw proposal
}

// Receive is called by the client or network layer on receipt of a Message from a peer.
// Any unmarshaling that may be required must have already been done.
//
// This function assumes that peer-to-peer connections are ordered and reliable,
// as they are when sent over Go channels or TCP/TLS connections.
// It also assumes that connection or peer failures are permanent:
// this implementation of QSC does not support restarting/resuming connections.
//
func (n *Node) Receive(msg *Message) {

	// Process only messages from the current or next time step.
	// We could accept and merge in information from older messages,
	// but it's perfectly safe and simpler just to ignore old messages.
	if msg.Step >= n.m.Step {

		// If msg is ahead of us, then virally catch up to it
		// Since we receive messages from a given peer in order,
		// a message we receive can be at most one step ahead of ours.
		if msg.Step > n.m.Step {
			n.Advance()
		}

		// Merge in received QSC state for rounds still in our pipeline
		mergeQSC(n.m.QSC[msg.Step:], msg.QSC)

		// Now process this message according to type.
		switch msg.Type {
		case Raw: // Acknowledge unwitnessed proposals.
			ack := n.newMsg()
			ack.Type = Ack
			n.send(msg.From, ack)

		case Ack: // Collect a threshold of acknowledgments.
			n.acks++
			if n.m.Type == Raw && n.acks >= n.thres {
				n.m.Type = Wit // Prop now threshold witnessed
				n.witnessedQSC()
				n.broadcastTLC()
			}

		case Wit: // Collect a threshold of threshold witnessed messages
			n.wits++ // witnessed messages in this step
			if n.wits >= n.thres {
				n.Advance() // tick the clock
			}
		}
	}
}
