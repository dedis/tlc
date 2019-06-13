package failstop

import (
	. "github.com/dedis/tlc/go/stack/arch"
)

// Witnessed QSC requires three TLC time-steps per consensus round.
const roundSteps = 3


type Stack interface {
	Self() Node
}

type Lower interface {

	// Returns the set of broadcast messages that had been seen
	// by a given node by a given recent time-step.
	// The expire return value from Advance determines
	// how many time-steps in the past this information is kept.
	Saw(node Node, step Step) MessageSet

	// Returns the set of threshold witnessed proposals
	// seen by a given node by a given recent time-step.
	Wit(node Node, step Step) MessageSet
}

// Layer represents a per-node instance of QSC consensus protocol state,
// which contains this node's record of consensus history.
type Layer struct {
	stack Stack
	lower Lower
	upper Upper

	self	Node		// This participant's node number
	choice	[]Choice	// Best proposal this node chose each round
}

// A Choice is a record of one node's QSC decision in one time-step
type Choice struct {
	Best	Node		// Best proposal this node chose in this round
	Commit	bool		// Whether node observed successful commitment
}


type Upper interface {

	// We call this to inform the upper layer of logical time advancing.
	// Returns the earliest timestep from which the upper layer
	// still needs history information to be preserved,
	// or -1 if the upper layer doesn't need any history preserved.
	Advance(step Step) (expire Step)
}


// Initialize the consensus layer state instance for a node.
func (l *Layer) Init(stack Stack, lower Lower, upper Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper

	l.self = stack.Self()
	l.choice = nil
}

// The TLC layer below upcalls this method on advancing to a new time-step,
// with sets of proposals seen (saw) and threshold witnessed (wit) recently.
func (l *Layer) Advance(step Step) (expire Step) {

	saw := l.lower.Saw(l.self, step)
	wit := l.lower.Wit(l.self, step)
	//println(n.self, n.tmpl.Step, "advanceQSC saw", len(saw),
	//	"wit", len(wit))

	// Calculate the starting step of the round that's just now completing.
	s := step - roundSteps
	if s < 0 {
		return	// Nothing to be done until the first round completes
	}

	// Find the best eligible proposal that was broadcast at s+0
	// and that is in our view by the end of the round at s+3.
	var bestProp *Message
	var bestTicket Ticket
	for p := range wit {
		if p.Type != Prop { panic("wit should contain only proposals") }
		if p.Step == s+0 && p.Ticket >= bestTicket {
			bestProp = p
			bestTicket = p.Ticket
		}
	}

	// Determine if we can consider this proposal permanently committed.
	spoiled := l.spoiledQSC(s, saw, bestProp, bestTicket)
	reconfirmed := l.reconfirmedQSC(s, wit, bestProp)
	committed := !spoiled && reconfirmed

	// Record the consensus results for this round (from s to s+3).
	l.choice = append(l.choice, Choice{bestProp.From, committed})
	//println(l.self, step, "choice", bestProp.From, "spoiled", spoiled, "reconfirmed", reconfirmed, "committed", committed)

	// Inform the upper layer of time advancing and the consensus decision.
	expire = l.upper.Advance(step)

	// Considering upper-layer application's history storage requirements,
	// ensure that we save enough history for the next consensus round.
	if expire < 0 || expire > s+1 {
		expire = s+1
	}
	return expire
}

// Return true if there's another proposal competitive with a given candidate.
func (l *Layer) spoiledQSC(s Step, saw MessageSet, prop *Message,
			ticket Ticket) bool {

	for p := range saw {
		if p.Step == s+0 && p.Type == Prop && p != prop &&
				p.Ticket >= ticket {
			return true	// victory spoiled by competition!
		}
	}
	return false
}

// Return true if given proposal was doubly confirmed (reconfirmed).
func (l *Layer) reconfirmedQSC(s Step, wit MessageSet, prop *Message) bool {
	for p := range wit {	// search for a paparazzi witness at s+1
		if p.Step == s+1 && l.lower.Wit(p.From, s+1).Has(prop) {
			return true
		}
	}
	return false
}

