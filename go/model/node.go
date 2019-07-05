package model

var Threshold int // TLC and consensus threshold
var All []*Node   // List of all nodes

var MaxSteps int          // Max number of consensus rounds to run
var MaxTicket int32 = 100 // Amount of entropy in lottery tickets

type Type int // Type of message
const (
	Prop Type = iota // Raw unwitnessed proposal
	Ack              // Acknowledgment of a proposal
	Wit              // Threshold witness confirmation of proposal
)

type Message struct {
	from   int      // Which node sent this message
	step   int      // Logical time step this message is for
	typ    Type     // Message type: Prop, Ack, or Wit
	prop   *Message // Proposal this Ack or Wit is about
	ticket int32    // Genetic fitness ticket for this proposal
	saw    set      // Recent messages the sender already saw
	wit    set      // Threshold witnessed messages the sender saw
}

type Node struct {
	comm   chan *Message // Channel to send messages to this node
	tmpl   Message       // Template for messages we send
	save   int           // Earliest step for which we maintain history
	acks   set           // Acknowledgments we've received in this step
	wits   set           // Threshold witnessed messages seen this step
	choice []*Message    // Best proposal this node chose each round
	commit []bool        // Whether we observed successful commitment
	done   chan struct{} // Run signals this when a node terminates
}

func newNode(self int) (n *Node) {
	n = &Node{}
	n.comm = make(chan *Message, 3*len(All)*MaxSteps)
	n.tmpl = Message{from: self, step: 0}
	n.done = make(chan struct{})
	return
}

func (n *Node) run() {
	n.advanceTLC(0) // broadcast message for initial time step
	for MaxSteps == 0 || n.tmpl.step < MaxSteps {
		msg := <-n.comm   // Receive a message
		n.receiveTLC(msg) // Process it
	}
	n.done <- struct{}{} // signal that we're done
}
