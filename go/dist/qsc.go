package dist

// RoundSteps is three because the witnessed QSC requires three TLC time-steps per consensus round.
const RoundSteps = 3

// The TLC layer upcalls this method on advancing to a new time-step,
// with sets of proposals seen (saw) and threshold witnessed (wit) recently.
func (n *Node) advanceQSC(saw, wit set) {
	//println(n.self, n.tmpl.Step, "advanceQSC saw", len(saw),
	//	"wit", len(wit))

	// Calculate the starting step of the round that's just now completing.
	s := n.tmpl.Step - RoundSteps
	if s < 0 {
		return // Nothing to be done until the first round completes
	}

	// Find the best eligible proposal that was broadcast at s+0
	// and that is in our view by the end of the round at s+3.
	var bestProp *Message
	var bestTicket int32
	for p := range wit {
		if p.Typ != Prop {
			panic("wit should contain only proposals")
		}
		if p.Step == s+0 && p.Ticket >= bestTicket {
			bestProp = p
			bestTicket = p.Ticket
		}
	}

	// Determine if we can consider this proposal permanently committed.
	spoiled := n.spoiledQSC(s, saw, bestProp, bestTicket)
	reconfirmed := n.reconfirmedQSC(s, wit, bestProp)
	committed := !spoiled && reconfirmed

	// Record the consensus results for this round (from s to s+3).
	n.choice = append(n.choice, choice{bestProp.From, committed})
	//println(n.self, n.tmpl.Step, "choice", bestProp.From, "spoiled", spoiled, "reconfirmed", reconfirmed, "committed", committed)

	// Don't bother saving history before the start of the next round.
	n.save = s + 1
}

// Return true if there's another proposal competitive with a given candidate.
func (n *Node) spoiledQSC(s int, saw set, prop *Message, ticket int32) bool {
	for p := range saw {
		if p.Step == s+0 && p.Typ == Prop && p != prop &&
			p.Ticket >= ticket {
			return true // victory spoiled by competition!
		}
	}
	return false
}

// Return true if given proposal was doubly confirmed (reconfirmed).
func (n *Node) reconfirmedQSC(s int, wit set, prop *Message) bool {
	for p := range wit { // search for a paparazzi witness at s+1
		if p.Step == s+1 && n.stepLog[p.From][s+1].wit.has(prop) {
			return true
		}
	}
	return false
}
