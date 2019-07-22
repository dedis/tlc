// This package implements a simple pedagogic model of TLC and QSC.
// It uses no cryptography or real network communication,
// using only Go channels and asynchronous goroutines to drive consensus.
package model

import (
	"math/rand"
)

// Best is a record representing either a best confirmed proposal,
// or a best potential spoiler competing with the best confirmed proposal.
type Best struct {
	From int // Node the proposal is from (spoiler: -1 for tied tickets)
	Tkt  int // Proposal's genetic fitness ticket
}

// Find the Best of two records primarily according to highest ticket number.
// For spoilers, detect and record ticket collisions with invalid node number.
func (b *Best) merge(o *Best, spoiler bool) {
	if o.Tkt > b.Tkt {
		*b = *o // strictly better ticket
	} else if o.Tkt == b.Tkt && o.From != b.From && spoiler {
		b.From = -1 // record ticket collision
	}
}

// Causally-combined QSC summary information for one consensus round
type Round struct {
	Spoil  Best // Best potential spoiler(s) we've found so far
	Conf   Best // Best confirmed proposal we've found so far
	Reconf Best // Best reconfirmed proposal we've found so far
	Commit bool // Whether we confirm this round successfully committed
}

// Merge QSC round info from an incoming message into our round history
func mergeQSC(b, o []Round) {
	for i := range o {
		b[i].Spoil.merge(&o[i].Spoil, true)
		b[i].Conf.merge(&o[i].Conf, false)
		b[i].Reconf.merge(&o[i].Reconf, false)
	}
}

// The TLC layer upcalls this method on advancing to a new time-step,
// with sets of proposals recently seen (saw) and threshold witnessed (wit).
func (n *Node) advanceQSC() {

	// Choose a fresh genetic fitness ticket for this proposal
	n.Tkt = 1 + int(rand.Int31n(MaxTicket))

	// Initialize consensus state for the round starting at step.
	// Find best spoiler, breaking ticket ties in favor of higher node
	n.QSC = append(n.QSC, Round{Spoil: Best{From: n.From, Tkt: n.Tkt}})

	// Decide if the just-completed consensus round successfully committed.
	r := &n.QSC[n.Step]
	r.Commit = r.Conf.From == r.Reconf.From && r.Conf.From == r.Spoil.From
}

// TLC layer upcalls this to inform us that our proposal is threshold witnessed
func (n *Node) witnessedQSC() {

	// Our proposal is now confirmed in the consensus round just starting
	// Find best confirmed proposal, breaking ties in favor of lower node
	n.QSC[n.Step+3].Conf.merge(&Best{From: n.From, Tkt: n.Tkt}, false)

	// Find reconfirmed proposals for the consensus round that's in step 1
	n.QSC[n.Step+2].Reconf.merge(&n.QSC[n.Step+2].Conf, false)
}
