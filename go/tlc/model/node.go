package model

import (
	"time"
)


var Threshold int		// TLC and consensus threshold
var All []Node			// List of all nodes

var MaxSteps int
var MaxSleep time.Duration
var MaxTicket int32 = 100	// Amount of entropy in lottery tickets


type Type int			// Tyope of message
const (
	Raw Type = iota		// Raw unwitnessed message
	Ack			// Acknowledgment of a Raw message
	Wit			// Threshold-witnessed message
)

type Message struct {
	sender	int		// Which node sent this message

	// Threshold Logical Clocks (TLC) message state
	step	int		// Logical time step this message is for
	typ	Type		// Message type
	ticket	int32		// Genetic fitness ticket for this proposal
	saw	set		// Messages sender already saw in this step
	wit	set		// Threshold witnessed messages seen this step
	lastsaw	set		// Messages sender saw in last time step
	lastwit	set		// Threshold witnessed messages seen last step

	// Que Sera Consensus (QSC) message state
	pred	*Message	// Chosen predecessor of this proposal
	commit	bool		// True if proposal is permanently committed
}


type Node struct {
	comm	chan *Message	// Channel to send messages to this node
	tmpl	Message		// Template for messages we send
	acks	set		// Acknowledgments we've received in this step

	// This node's record of TLC history
	saw	[]set		// all broadcast messages seen in prior steps
	wit	[]set		// threshold witnessed messages in prior steps

	// This node's record of QSC consensus history
	choice	[]int		// Which node's message was chosen each round
	commit	[]bool		// Whether we observed successful commitment

	done	chan struct{}	// Run signals this when a node terminates
}

func (n *Node) Init(self int) {

	n.comm = make(chan *Message)
	n.tmpl = Message{sender: self, step: 0}
	n.done = make(chan struct{})
}

func (n *Node) Run(self int) {
	n.advanceTLC(0, nil, nil) // broadcast message for initial time step
	for MaxSteps == 0 || n.tmpl.step < MaxSteps {
		r := <-n.comm		// Receive a message
		n.receiveTLC(r)		// Process it
	}
	n.done <- struct{}{}	// signal that we're done
}


// Initialize and run the model for a given threshold and number of nodes.
func Run(threshold, nnodes int) {

	// Initialize the nodes
	Threshold = threshold
	All = make([]Node, nnodes)
	for i := range All {
		All[i].Init(i)
	}

	// Run all the nodes asynchronously on separate goroutines
	for i := range All {
		go All[i].Run(i)
	}

	// Wait for all the nodes to complete their execution
	for i := range All {
		<-All[i].done
	}
}

