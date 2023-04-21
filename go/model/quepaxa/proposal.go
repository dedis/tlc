package quepaxa

// The Proposal interface defines constraints for a concrete proposal type P.
//
// The Rank method must return the same proposal with rank set appropriately:
// - to the maximum rank High if leader is set (this replica is the leader)
// - to a freshly-chosen random rank between 1 and High-1 otherwise
//
// In addition, if proposal ranks are low-entropy so there is a chance of ties,
// and P is using replica numbers for tiebreaking,
// then the Rank function also sets the replica number in the proposal.
type Proposal[P any] interface {
	Nil() P                           // the nil proposal
	Best(other P) P                   // best of this and other
	Rank(replica Node, leader bool) P // randomly rank proposal
	EqD(other P) bool                 // equality for deciding
}

//type Proposal[D any] struct {
//	R Rank			// random rank to prioritize this proposal
//	I Node			// identity of initial proposing replica
//	D Data			// decision data associated with proposal
//}

//func (x Proposal[D]).Cmp(y Proposal[D]) bool {
//	switch {
//	case x.R < y.R || (x.R == y.R && x.I < y.I):
//		return -1
//	case x.R > y.R || (x.R == y.R && x.I > y.I):
//		return 1
//	default:
//		return 0
//	}
//}
