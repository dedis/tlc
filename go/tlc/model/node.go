package model

import (
	"time"
	"sync"
)


var Threshold int		// TLC and consensus threshold
var All []*Node			// List of all nodes

var MaxSteps int
var MaxSleep time.Duration
var MaxTicket int32 = 100	// Amount of entropy in lottery tickets


type Type int			// Tyope of message
const (
	Prop Type = iota	// Raw unwitnessed proposal
	Ack			// Acknowledgment of a proposal
	Wit			// Threshold witness confirmation of proposal
)

type Message struct {
	sender	int		// Which node sent this message
	step	int		// Logical time step this message is for
	typ	Type		// Message type
	prop	*Message	// Proposal this Ack or Wit is about
	ticket	int32		// Genetic fitness ticket for this proposal
	saw	set		// Recent messages the sender already saw
	wit	set		// Threshold witnessed messages the sender saw
}


type Node struct {
	comm	chan *Message	// Channel to send messages to this node
	tmpl	Message		// Template for messages we send
	save	int		// Earliest step for which we maintain history
	acks	set		// Acknowledgments we've received in this step
	wits	set		// Threshold witnessed messages seen this step

	// This node's record of QSC consensus history
	choice	[]*Message	// Best proposal this node chose each round
	commit	[]bool		// Whether we observed successful commitment
	commits	int		// Total number of rounds we observed success
	mutex	sync.Mutex	// Mutex protecting access to the above state

	done	chan struct{}	// Run signals this when a node terminates
}

func NewNode(self int) (n *Node) {
	n = &Node{}
	n.comm = make(chan *Message, 3 * len(All) * MaxSteps)
	n.tmpl = Message{sender: self, step: 0}
	n.done = make(chan struct{})
	return
}

func (n *Node) Run(self int) {
	n.advanceTLC(0) // broadcast message for initial time step
	for MaxSteps == 0 || n.tmpl.step < MaxSteps {
		msg := <-n.comm		// Receive a message
		n.receiveTLC(msg)	// Process it
	}
	n.done <- struct{}{}	// signal that we're done
}


// Initialize and run the model for a given threshold and number of nodes.
func Run(threshold, nnodes int) {

	println("Run config", threshold, "of", nnodes)

	// Initialize the nodes
	Threshold = threshold
	All = make([]*Node, nnodes)
	for i := range All {
		All[i] = NewNode(i)
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

