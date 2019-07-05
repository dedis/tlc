package model

import (
	"math/rand"
)

// Best is a record representing either a best confirmed proposal,
// or a best potential spoiler competing with the best confirmed proposal.
type Best struct {
	from	int	// Node: i for best confirmed, n-i for best spoiler
	pri	int	// Priority based tkt & from (spoiler) or -from (conf)
}

// Find the Best of two records primarily according to highest ticket number,
// and secondarily according to highest from node number.
func (b *Best) merge(o *Best) {
	if (o.pri > b.pri) {
		*b = *o
	}
}

// Causally-combined QSC summary information for one consensus round
type Round struct {
	spoil	Best	// Best potential spoiler we've found so far
	conf	Best	// Best confirmed proposal we've found so far
	reconf	Best	// Best reconfirmed proposal we've found so far
}

func mergeQSC(b, o []Round) {
	for i := range o {
		b[i].spoil.merge(&o[i].spoil)
		b[i].conf.merge(&o[i].conf)
		b[i].reconf.merge(&o[i].reconf)
	}
}

// The TLC layer upcalls this method on advancing to a new time-step,
// with sets of proposals recently seen (saw) and threshold witnessed (wit).
func (n *Node) advanceQSC() {

	// Choose a fresh genetic fitness ticket for this proposal
	n.msg.tkt = 1 + int(rand.Int31n(MaxTicket))

	// Initialize consensus state for the round starting at step.
	// Find best spoiler, breaking ticket ties in favor of higher node
	bestSpoiler := Best{from: n.msg.from,
			    pri: n.msg.tkt * len(All) + n.msg.from}
	if len(n.msg.qsc) != n.msg.step+3 { panic("XXX") }
	n.msg.qsc = append(n.msg.qsc, Round{ spoil: bestSpoiler })

	// Decide if the just-completed consensus round successfully committed.
	r := &n.msg.qsc[n.msg.step]
	committed := (r.conf.from == r.reconf.from) &&
		     (r.conf.from == r.spoil.from)
	//println(n.msg.from, n.msg.step, "conf", r.conf.from,
	//		"reconf", r.reconf.from,
	//		"spoil", r.spoil.from)

	// Record the consensus results for this round (from s to s+3).
	n.choice = append(n.choice, r.conf.from)
	n.commit = append(n.commit, committed)
}

// TLC layer upcalls this to inform us that our proposal is threshold witnessed
func (n *Node) witnessedQSC() {

	// Our proposal is now confirmed in the consensus round just starting
	// Find best confirmed proposal, breaking ties in favor of lower node
	bestConfirmed := Best{from: n.msg.from,
			      pri: (n.msg.tkt + 1) * len(All) - n.msg.from}
	n.msg.qsc[n.msg.step+3].conf.merge(&bestConfirmed)

	// Find reconfirmed proposals for the consensus round that's in step 1
	n.msg.qsc[n.msg.step+2].reconf.merge(&n.msg.qsc[n.msg.step+2].conf)
}

