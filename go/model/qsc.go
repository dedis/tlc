package model

// Best is a record representing either a best confirmed proposal,
// or a best potential spoiler competing with the best confirmed proposal,
// used in the Round struct.
//
// In each case, the only information we really need is
// the genetic fitness lottery ticket of the "best" proposal seen so far,
// and which node produced that proposal.
// This optimization works only in the non-Byzantine QSC consensus protocol,
// because Byzantine consensus requires that the lottery tickets be
// unknown and unbiasable to everyone until the consensus round completes.
//
// When we're collecting the best potential spoiler proposal -
// the proposal with the highest ticket regardless of whether it's confirmed -
// we must keep track of ticket collisions,
// in case one colliding proposal might "win" if not spoiled by the other.
// When we detect a spoiler collision, we simply set From to -1,
// an invalid node number that will be unequal to, and hence properly "spoil",
// a confirmed or reconfirmed proposal with the same ticket from any node.
//
type Best struct {
	From int    // Node the proposal is from (spoiler: -1 for tied tickets)
	Tkt  uint64 // Proposal's genetic fitness ticket
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

// Round encapsulates all the QSC state needed for one consensus round:
// the best potential "spoiler" proposal regardless of confirmation status,
// the best confirmed (witnessed) proposal we've seen so far in the round,
// and the best reconfirmed (double-witnessed) proposal we've seen so far.
// Finally, at the end of the round, we set Commit to true if
// the best confirmed proposal in Conf has definitely been committed.
type Round struct {
	Spoil  Best // Best potential spoiler(s) we've found so far
	Conf   Best // Best confirmed proposal we've found so far
	Reconf Best // Best reconfirmed proposal we've found so far
	Commit bool // Whether we confirm this round successfully committed
}

// Merge QSC round info from an incoming message into our round history
func mergeQSC(b, o []Round) {
	for i := range b {
		b[i].Spoil.merge(&o[i].Spoil, true)
		b[i].Conf.merge(&o[i].Conf, false)
		b[i].Reconf.merge(&o[i].Reconf, false)
	}
}

// The TLC layer upcalls this method on advancing to a new time-step,
// with sets of proposals recently seen (saw) and threshold witnessed (wit).
func (n *Node) advanceQSC() {

	// Choose a fresh genetic fitness ticket for this proposal
	n.m.Tkt = uint64(n.Rand()) | (1 << 63) // Ensure it's greater than zero

	// Initialize consensus state for the round starting at step.
	// Find best spoiler, breaking ticket ties in favor of higher node
	newRound := Round{Spoil: Best{From: n.m.From, Tkt: n.m.Tkt}}
	n.m.QSC = append(n.m.QSC, newRound)

	// Decide if the just-completed consensus round successfully committed.
	r := &n.m.QSC[n.m.Step]
	r.Commit = r.Conf.From == r.Reconf.From && r.Conf.From == r.Spoil.From
}

// TLC layer upcalls this to inform us that our proposal is threshold witnessed
func (n *Node) witnessedQSC() {

	// Our proposal is now confirmed in the consensus round just starting
	// Find best confirmed proposal, breaking ties in favor of lower node
	myBest := &Best{From: n.m.From, Tkt: n.m.Tkt}
	n.m.QSC[n.m.Step+3].Conf.merge(myBest, false)

	// Find reconfirmed proposals for the consensus round that's in step 1
	n.m.QSC[n.m.Step+2].Reconf.merge(&n.m.QSC[n.m.Step+2].Conf, false)
}
