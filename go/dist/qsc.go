package dist

// Best is a record representing either a best confirmed proposal,
// or a best potential spoiler competing with the best confirmed proposal.
type Best struct {
	from int   // Node the proposal is from (spoiler: -1 for tied tickets)
	tkt  int64 // Proposal's genetic fitness ticket
}

// Find the Best of two records primarily according to highest ticket number.
// For spoilers, detect and record ticket collisions with invalid node number.
func (b *Best) merge(o *Best, spoiler bool) {
	if o.tkt > b.tkt {
		*b = *o // strictly better ticket
	} else if o.tkt == b.tkt && o.from != b.from && spoiler {
		b.from = -1 // record ticket collision
	}
}

// Causally-combined QSC summary information for one consensus round
type Round struct {
	spoil  Best // Best potential spoiler(s) we've found so far
	conf   Best // Best confirmed proposal we've found so far
	reconf Best // Best reconfirmed proposal we've found so far
	commit bool // Whether we confirm this round successfully committed
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
	n.tkt = n.Rand()

	// Initialize consensus state for the round starting at step.
	n.qsc = append(n.qsc, Round{spoil: Best{from: n.from, tkt: n.tkt},
		conf: Best{tkt: -1}, reconf: Best{tkt: -1}})

	// Decide if the just-completed consensus round successfully committed.
	r := &n.qsc[n.step]
	r.commit = r.conf.from == r.reconf.from && r.conf.from == r.spoil.from
}

// TLC layer upcalls this to inform us that our proposal is threshold witnessed
func (n *Node) witnessedQSC() {

	// Our proposal is now confirmed in the consensus round just starting
	// Find best confirmed proposal, breaking ties in favor of lower node
	n.qsc[n.step+3].conf.merge(&Best{from: n.from, tkt: n.tkt}, false)

	// Find reconfirmed proposals for the consensus round that's in step 1
	n.qsc[n.step+2].reconf.merge(&n.qsc[n.step+2].conf, false)
}
