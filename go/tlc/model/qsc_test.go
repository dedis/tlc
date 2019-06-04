package tlc

import (
	"testing"
)


func setRand(b []byte) int {
	for {
		r := rand() % len(b)
		if b[r] == 0 {
			b[r] = 1
			return r
		}
	}
}


type qscTestMsg struct {
}

type qscTestView struct {
	step int
	recv []qscTestMsg		// new message(s) received in this view
	
}

type qscTestNode struct {
	node Node			// QSC node instance to be tested

	step int
	view []qscTestview		// log of this node's views

	nrcv int			// # witnessed messages received so far
	rcvd []qscTestMsg		// messages received from other nodes
}

func (tn *qscTestNode) init(threshold, nnodes int) {

	// Prepare and "broadcast" the step 0 message
	tn.Init(threshold, nnodes)

	tn.step = 0
	tn.initStep()
}

func (tn *qscTestNode) advanceToStep(toStep int, dos []byte) {

	while tn.step < toStep {

		// pull witnessed messages from other nodes in random order
		// until we reach the required threshold.
		hit := append(nil, ...dos)
		while tn.nrcv < tn.Node.threshold {

			// Pick any non-DoS'd node at random
			i = setRand(hit)
			if tn.rcvd[i] != nil {
				continue	// already received; pick again
			}

			...
		}

		v := qscTestView{}
		assert(len(tn.view) == tn.step)
		tn.view = append(tn.view, &v)
		tn.step++

		tn.initStep()
	}
}

func (tn *qscTestNode) initStep(step int) {

	// Clear the per-step state
	tn.nrcv = 0
	tn.rcvd = make([]qscTestMsg, tn.Node.nnodes)
}


func qscTestRun(t *testing.T, n, f, nrounds int) {

	t := n - f
	nsteps := nrounds * RoundSteps

	node := make([]testNode, 4)
	for i := 0; i < 4; i++ {
		node[i].init()
	}

	var dos []byte, ndos int
	dos_steps := 1
	for s := 0; s < nsteps; s++ {

		if --dos_steps == 0 { 

			// Pick a set of up to f nodes to
			// virtually "DoS attack" in upcoming steps
			ndos = f
			if rand() & 1 != 0 {
				// Half the time, attack the maximum of f nodes.
				// The other half the time, attack <f.
				ndos = rand() % f
			}
			dos = new(byte[], n)
			for j := 0; j < ndos; j++ {
				setRand(dos)
			}

			// Hold this DoS configuration a random period
			dos_steps = rand() % 20
		}

		// Now advance the un-attacked nodes to step s
		// and give them a chance to "broadcast" their messages
		done := append(nil, ...dos)
		for i := n - ndos; i > 0; i-- {
			j := setRand(done)
			node[i].advanceToStep(s, dos)
		}
	}
}

func TestQSC(t *testing.T) {
	qscTestRun(3, 1, 100)
}

