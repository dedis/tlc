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
	from int // Node: i for best confirmed, n-i for best spoiler
	tkt  int // Priority based tkt & from (spoiler) or -from (conf)
}

// Find the Best of two records primarily according to highest ticket number,
// and secondarily according to highest from node number.
func (b *Best) merge(o *Best, spoiler bool) {
	if o.tkt > b.tkt {
		*b = *o // strictly better ticket
	} else if o.tkt == b.tkt && o.from != b.from && spoiler {
		b.from = -1 // record ticket collision
	}
}

// Causally-combined QSC summary information for one consensus round
type Round struct {
	spoil  Best // Best potential spoiler we've found so far
	conf   Best // Best confirmed proposal we've found so far
	reconf Best // Best reconfirmed proposal we've found so far
}

// Merge QSC round info from an incoming message into our round history
func mergeQSC(b, o []Round) {
	for i := range o {
		b[i].spoil.merge(&o[i].spoil, true)
		b[i].conf.merge(&o[i].conf, false)
		b[i].reconf.merge(&o[i].reconf, false)
	}
}

// The TLC layer upcalls this method on advancing to a new time-step,
// with sets of proposals recently seen (saw) and threshold witnessed (wit).
func (n *Node) advanceQSC() {

	// Choose a fresh genetic fitness ticket for this proposal
	n.tkt = 1 + int(rand.Int31n(MaxTicket))

	// Initialize consensus state for the round starting at step.
	// Find best spoiler, breaking ticket ties in favor of higher node
	n.qsc = append(n.qsc, Round{spoil: Best{from: n.from, tkt: n.tkt}})

	// Decide if the just-completed consensus round successfully committed.
	r := &n.qsc[n.step]
	committed := r.conf.from == r.reconf.from &&
		r.conf.from == r.spoil.from

	// Record the consensus results for this round (from s to s+3).
	n.choice = append(n.choice, r.conf.from)
	n.commit = append(n.commit, committed)
}

// TLC layer upcalls this to inform us that our proposal is threshold witnessed
func (n *Node) witnessedQSC() {

	// Our proposal is now confirmed in the consensus round just starting
	// Find best confirmed proposal, breaking ties in favor of lower node
	n.qsc[n.step+3].conf.merge(&Best{from: n.from, tkt: n.tkt}, false)

	// Find reconfirmed proposals for the consensus round that's in step 1
	n.qsc[n.step+2].reconf.merge(&n.qsc[n.step+2].conf, false)
}
