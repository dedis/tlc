package model


// Witnessed QSC requires three TLC time-steps per consensus round.
const RoundSteps = 3


// The TLC layer upcalls this method on advancing to a new time-step.
// The step parameter will always be monotonically increasing over time.
// We must fill in the QSC layer part of the message template
// for the new message this node broadcasts for this time step.
func (n *Node) advanceQSC() {

	// Calculate the starting step of the round that's just now completing.
	s := n.tmpl.step - RoundSteps
	if s < 0 {
		return	// Nothing to be done until the first round completes
	}

	// Find the best eligible proposal that was broadcast at s+0
	// and that is in our view by the end of the round at s+3.
	var bestProp *Message
	var bestTicket int32
	for m2 := range n.saw[s+2] { // step s+2 messages we saw by s+3
		if m2.step != s+2 { panic("wrong time step")  }
		for m1 := range m2.lastsaw { // s+1 messages we saw by s+2
			if m1.step != s+1 { panic("wrong time step")  }
			for m0 := range m1.lastwit { // s+0 witnessed msgs
				if m0.step != s { panic("wrong time step")  }
				if m0.ticket >= bestTicket {
					bestProp = m0
					bestTicket = m0.ticket
				}
			}
		}
	}

	// Determine if we can consider this proposal permanently committed.
	committed :=
		!n.spoiledQSC(s, bestProp, bestTicket) &&
		n.reconfirmedQSC(s, bestProp)

	// Record the consensus results for this round (from s to s+3).
	n.choice = append(n.choice, bestProp.sender)
	n.commit = append(n.commit, committed)

	// (Racy) global sanity-check our results against other nodes' choices
	if committed {
		for _, nn := range All {
			if len(nn.choice) > s && nn.choice[s] != bestProp.sender{
				panic("consensus safety violation!")
			}
		}
	}
}

// Return true if there's another proposal competitive with a candidate
// that we could have learned about by step s+2.
func (n *Node) spoiledQSC(s int, proposal *Message, ticket int32) bool {
	for m1 := range n.saw[s+1] {
		if m1.step != s+1 { panic("wrong time step") }
		for m0 := range m1.lastsaw {
			if m0.step != s { panic("wrong time step") }
			if m0 != proposal && m0.ticket >= ticket {
				return true	// proposal has competition!
			}
		}
	}
	return false
}

// Return true if given proposal was doubly confirmed (reconfirmed) by s+2.
func (n *Node) reconfirmedQSC(s int, prop *Message) bool {
	for m1 := range n.wit[s+1] {
		if m1.step != s+1 { panic("wrong time step") }
		if m1.lastwit.has(prop) {
			return true
		}
	}
	return false
}

