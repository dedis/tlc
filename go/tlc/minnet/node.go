package minnet

import (
	"time"
	"sync"
)


var Threshold int		// TLC and consensus threshold
var All []*Node			// List of all nodes

var MaxSteps int
var MaxSleep time.Duration
var MaxTicket int32 = 100	// Amount of entropy in lottery tickets


type Type int			// Type of message
const (
	Prop Type = iota	// Raw unwitnessed proposal
	Ack			// Acknowledgment of a proposal
	Wit			// Threshold witness confirmation of proposal
	Done			// Internal message indicating time to quit
)

type msgId struct {
	from	int		// Sending node number
	seq	int		// Message index in sender's log
}

type Message struct {
	from	int		// Which node originally sent this message
	seq	int		// Node-local sequence number for vector time
	step	int		// Logical time step this message is for
	typ	Type		// Message type
	vec	vec		// Vector clock update from sender node
	prop	*Message	// Proposal this Ack or Wit is about
	ticket	int32		// Genetic fitness ticket for this proposal
	saw	set		// Recent messages the sender already saw
	wit	set		// Threshold witnessed messages the sender saw
}

type Node struct {
	self	int		// This node's participant number
	comm	[]chan *Message	// Channels to send messages to this node
	recv	chan *Message	// Node-internal message receive channel
	mutex	sync.Mutex	// Mutex protecting node's protocol stack

	tmpl	Message		// Template for messages we send
	save	int		// Earliest step for which we maintain history
	acks	set		// Acknowledgments we've received in this step
	wits	set		// Threshold witnessed messages seen this step

	mat	[]vec		// Node's current matrix clock
	log	[][]*Message	// Record of all nodes' message histories
	oom	[][]*Message	// Out-of-order messages not yet delivered

	// This node's record of QSC consensus history
	choice	[]*Message	// Best proposal this node chose each round
	commit	[]bool		// Whether we observed successful commitment

	done	chan struct{}	// Run signals this when a node terminates
}

func NewNode(self int) (n *Node) {
	n = &Node{}
	n.initGossip(self)

	n.tmpl = Message{from: self, step: 0}

	n.done = make(chan struct{})
	return
}

// Initialize and run the model for a given threshold and number of nodes.
func Run(threshold, nnodes int) {

	//println("Run config", threshold, "of", nnodes)

	// Initialize the nodes
	Threshold = threshold
	All = make([]*Node, nnodes)
	for i := range All {
		All[i] = NewNode(i)
	}

	// Run all the nodes asynchronously on separate goroutines
	for i, n := range All {
		go n.runGossip(i)
	}

	// Wait for all the nodes to complete their execution
	for _, n := range All {
		<-n.done
	}
}

