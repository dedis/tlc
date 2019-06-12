package vectime

import (
	"tlc"
	"tlc/arch"
	"crypto/hash"
)


// A viewstamp is a concatenation of hashes of log heads of each node,
// indexed by participant number and corresponding to a vector timestamp.
// The gossip protocol uses viewstamps to detect any divergence
// between two peers' views of all participants' logs and detect equivocation.
// To lay blame, the peers need to exchange viewstamps
// to obtain the multiple-signed log heads of the equivocating node(s).
type viewstamp []byte


type Lower interface {
	arch.Lower
	Seq() stack.Seq		// Our most recent unassigned sequence number
	Peer(i int) LowerPeer	// Find per-peer state for a given node
}

// Our required interface to per-peer state managed by the lower layer.
type LowerPeer interface {
}


type Layer struct {
	stack arch.Stack
	lower Lower
	upper arch.Upper

	node int		// Node number this instance is for
	peer []Peer		// Per-peer state for all peers we track
}

type Peer struct {
	lower LowerPeer
}

func (l *Layer) Init(stack arch.Stack, lower arch.Lower, upper arch.Upper) {
	l.stack = stack
	l.lower = lower
	l.upper = upper

	...
}

func (l *Layer) Send(msg arch.Message) {

	hashLen := l.stack.HashLen()

	// Attach a vector timestamp to the message.
	// Also save a corresponding viewstamp for peer-to-peer cross-checking.
	n := len(l.peer)
	v := make(Vec, n)
	vs := make([]byte, 0, n * hashLen)
	for i := range l.peer {
		v[i] = l.peer.Seq()
		vs = append(vs, XXX)
	}
	XXX stash vs

	// include the hash of the viewstamp with the vector timestamp
	h := l.stack.Hash()
	h.Write(vs)
	vsh := h.Sum(nil)

	msg.SetVec(v, vsh)

	// Pass on the message to the next lower layer
	l.lower.Send(msg)
}

func (l *Layer) Recv(msg arch.Message) {

	if vsm := msg.ViewstampMessage(); vsm != nil {
		l.recvViewstampMessage(msg, vsm)
		return
	}

	// Hold and reorder messages received out-of-order?
	// Or just check in-order?

	// Once we've obtained all precursers to a received message,
	// check viewstamp consistency.
	// If inconsistent, block further message processing from this peer?
	// and send peer request for its viewstamp on this vector.

	l.upper.Recv(msg)
}

func (l *Layer) recvViewstampMessage(msg stack.Message, vsm arch.ViewstampMessage) {

	// Compare peer's viewstamp for this vector with ours.
	for i, seq := range vsm.View {

		Ensure Seq is within our window for peer i.

		Check peer's log hash for that seq with ours.

		...
	}

	// Check if this discharges an outstanding ViewReqMessage request?
	// Do we actually need to keep track of outstanding requests?
	// Or if the peer is dishonest and never answers our request,
	// can we just ensure the transaction goes safely
	// into a 'blocked' state that will eventually be closed
	// due to lack of progress?
}

