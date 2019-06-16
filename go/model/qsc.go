package model

// The TLC layer upcalls this method on advancing to a new time-step,
// with sets of proposals recently seen (saw) and threshold witnessed (wit).
func (n *Node) advanceQSC(saw, wit set) {

	// Calculate the starting step of the round that's just now completing.
	s := n.tmpl.step - 3	// Three steps per round
	if s < 0 {
		return	// Nothing to be done until the first round completes
	}

	// Find the best eligible proposal that was broadcast at s+0
	// and that is in our view by the end of the round at s+3.
	var bestProp *Message
	var bestTicket int32
	for p := range wit {
		if p.step == s+0 && p.ticket >= bestTicket {
			bestProp = p
			bestTicket = p.ticket
		}
	}

	// Determine if we can consider this proposal permanently committed.
	committed := !n.spoiledQSC(s, saw, bestProp, bestTicket) && 
		     n.reconfirmedQSC(s, wit, bestProp)

	// Record the consensus results for this round (from s to s+3).
	n.choice = append(n.choice, bestProp)
	n.commit = append(n.commit, committed)

	// Don't bother saving history before the start of the next round.
	n.save = s+1
}

// Return true if there's another proposal competitive with a given candidate.
func (n *Node) spoiledQSC(s int, saw set, prop *Message, ticket int32) bool {
	for p := range saw {
		if p.step == s+0 && p.typ == Prop && p != prop &&
				p.ticket >= ticket {
			return true	// victory spoiled by competition!
		}
	}
	return false
}

// Return true if given proposal was doubly confirmed (reconfirmed).
func (n *Node) reconfirmedQSC(s int, wit set, prop *Message) bool {
	for p := range wit {	// search for a paparazzi witness at s+1
		if p.step == s+1 && p.wit.has(prop) {
			return true
		}
	}
	return false
}

