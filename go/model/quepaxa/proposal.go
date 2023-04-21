package quepaxa

import (
	"crypto/rand"
	"encoding/binary"
	"math"
)

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

// BasicProposal provides a basic proposal design
// that represents a reasonable "sweet spot" for most purposes.
//
// Proposals are randomly ranked using 31 bits of private randomness,
// drawn from the cryptographic random source for strong unpredictability,
// which might conceivably be needed to protect against a strong DoS attacker.
// Since 31-bit random ranks do not have high entropy,
// BasicProposal uses recorder numbers for breaking ties.
//
// BasicProposal contains a field D of parameterized type Data,
// containing any application-defined data associated with the proposal.
// This type may contain pointers or slices (e.g., referring to bulk data)
// provided the referenced data objects do not change during consensus.
// The BasicProposal does nothing with this data field other than copy it.
type BasicProposal[Data any] struct {
	R uint32 // Randomzed rank or priority
	N Node   // Replica for which proposal was created
	D Data   // Application-defined data
}

const basicProposalHighRank = math.MaxUint32

func (_ BasicProposal[D]) Nil() BasicProposal[D] {
	return BasicProposal[D]{}
}

func (p BasicProposal[D]) Best(o BasicProposal[D]) BasicProposal[D] {
	if o.R > p.R || (o.R == p.R && o.N > p.N) {
		return o
	}
	return p
}

func (p BasicProposal[D]) Rank(node Node, leader bool) BasicProposal[D] {

	// record the replica number that this proposal was produced for
	p.N = node

	if leader {
		// the leader always uses the reserved maximum rank
		p.R = basicProposalHighRank

	} else {
		// read 32 bits of randomness
		var b [4]byte
		_, err := rand.Read(b[:])
		if err != nil {
			panic("unable to read cryptographically random bits: " +
				err.Error())
		}

		// produce a 31-bit rank, avoiding the zero rank
		p.R = (binary.BigEndian.Uint32(b[:]) & 0x7fffffff) + 1
	}
	return p
}

func (p BasicProposal[D]) EqD(o BasicProposal[D]) bool {
	return p.R == o.R && p.N == o.N
}

var bp BasicProposal[struct{}]
var prop Proposer[BasicProposal[struct{}]]
