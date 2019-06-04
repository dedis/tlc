package tlc

import (
	"math/big"
)


// Witnessed QSC requires three TLC time-steps per consensus round.
const RoundSteps = 3


type Msg struct {
	Ticket() big.Int	// Proposal's lottery ticket
	b []byte
}


type Result struct {
	Choice uint;		// Local choice of proposal to build on
	Commit bool;		// True if Choice is committed, not tentative
}

type View interface {
	Config() (t, n int)	// Number of nodes and progress threshold
	Ticket(s uint, i int)	// Get ticket of i's proposal from step s
	Witn(s uint, i int) int	// Number of nodes that witnessed i's proposal
}


// Per-node QSC state object
type Node struct {
	nnodes, threshold int
	step int
}

func (n *Node) Init(threshold, nnodes int) {
	n.nnodes = nnodes
	n.threshold  = threshold

	n.step = -1
	n.Step(0)
}

// TLC upcalls Step on receiving a new message from another node.
func (n *Node) Recv(msg Msg, view View) {
	// XXX
}


// TLC upcalls Step after receiving enough messages
// to advance to a new time-step.
// The step parameter will always be monotonically increasing over time,
// but may skip step numbers to  "catch up" after falling behind
func (n *Node) Step(step int, view View) {
	assert(step > n.step)
	n.step = step

	// XXX
}

func CalcResult(start uint, v View) (choice uint, committed bool) {
	t, n := v.Config()

	// First find the best proposal and best eligible proposal.
	bprop := -1
	beprop := -1
	var btkt, betkt big.Int
	for i := 0; i < n; i++ {
		ticket := v.Ticket(start, i)

		switch ticket.Cmp(bprop) {
		case 1:
			bprop = i
			btkt = ticket
		case 2:
			bprop = -1		// no best due to tie
		}

		if v.Witn(start, i) >= t {
			if ticket.Cmp(beprop) >= 0 {
				beprop = i
				betkt = ticket
			}
		}
	}
	assert beprop >= 0
	choice = beprop

	// Determine if the best proposal was fully committed
	committed = false
	if bprop != beprop {
		return				// spoiler
	}

	// See if any paparazzi gossipped the proposal's eligibility
	for i := 0; i < n; i++ {
		pv = v.Prior(start+1, i)
		if p != nil && pv.Witn(start, beprop) >= t &&
				v.Witn(start+1, i) >= t {
			committed = true
			break
		}
	}

	return
}

