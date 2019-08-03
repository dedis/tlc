// This package implements a simple pedagogic model of TLC and QSC.
// It uses no cryptography and supports only failstop, non-Byzantine consensus,
// but should be usable in scenarios that would typically employ Paxos or Raft.
//
// To read about the principles underlying TLC and QSC, please refer to:
// https://arxiv.org/abs/1907.07010
// For a high-level overview of the different implementations of TLC/QSC
// in different languages that live in this repository, please see:
// https://github.com/dedis/tlc/
//
// Configuring and launching consensus groups
//
// To use this implementation of QSC,
// a user of this package must first configure and launch
// a threshold group of nodes.
// This package handles only the core consensus logic,
// leaving matters such as node configuration, network names, connections,
// and wire-format marshaling and unmarshaling to the client of this package.
//
// The client using this package
// must assign each node a unique number from 0 through nnode-1,
// e.g., by configuring the group with a well-known ordering of its members.
// Only node numbers are important to this package; it is oblivious to names.
//
// When each node in the consensus group starts,
// the client calls NewNode to initialize the node's TLC and QSC state.
// The client may then change optional Node configuration parameters,
// such as Node.Rand, before actually commencing protocol message processing.
// The client then calls Node.Advance to launch TLC and the consensus protocol,
// advance to time-step zero, and broadcast a proposal for this time-step.
// Thereafter, the protocol self-clocks asynchronously using TLC
// based on network communication.
//
// Consensus protocol operation and results
//
// This package implements QSC in pipelined fashion, which means that
// a sliding window of three concurrent QSC rounds is active at any time.
// At the start of any given time step s when Advance broadcasts a Raw message,
// this event initiates a new consensus round starting at s and ending at s+3,
// and (in the steady state) completes a consensus round that started at s-3.
// Each Message a node broadcasts includes QSC state from four rounds:
// Message.QSC[0] holds the results of the consensus round just completed,
// while QSC[1] through QSC[3] hold the state of the three still-active rounds,
// with QSC[3] being the newest round just launched.
//
// If Message.QSC[0].Commit is true in the Raw message commencing a time-step,
// then this node saw the round ending at step Message.Step as fully committed.
// In this case, all nodes will have agreed on the same proposal in that round,
// which is the proposal made by node number Message.QSC[0].Conf.From.
// If the client was waiting for a particular transaction to be ordered
// or definitely committed/aborted according to the client's transaction rules,
// then seeing that Message.QSC[0].Commit is true means that the client may
// resolve the status of transactions proposed up to Message.Step-3.
// Other nodes might not have observed this same round as committed, however,
// so the client must not assume that other nodes also necessarily be aware
// that this consensus round successfully committed.
//
// If Message.QSC[0].Commit is false, the round may or may not have converged:
// this node simply cannot determine conclusively whether the round converged.
// Other nodes might have chosen different "best confirmed" proposals,
// as indicated in their respective QSC[0].Conf.From broadcasts for this step.
// Alternatively, the round may in fact have converged,
// and other nodes might observe that fact, even though this node did not.
//
// Message transmission, marshaling
//
// This package invokes the send function provided to NewNode to send messages,
// leaving any wire-format marshaling required to the provided function.
// This allows the client complete control over the desired wire format,
// and to include other information beyond the fields defined in Message,
// such as any semantic content on which the client wishes to achieve consensus.
// On receipt of a message from another node,
// the client must unmarshal it as appropriate
// and invoke Node.Receive with the unmarshalled Message.
//
// Concurrency control
//
// The consensus protocol logic in this package is not thread safe:
// it must be run in a single goroutine,
// or else the client must implement appropriate locking.
//
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
