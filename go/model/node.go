package model

var Threshold int // TLC and consensus threshold
var All []*Node   // List of all nodes

var MaxSteps int          // Max number of consensus rounds to run
var MaxTicket int32 = 100 // Amount of entropy in lottery tickets

type Type int // Type of message
const (
	Raw Type = iota  // Raw unwitnessed proposal
	Ack              // Acknowledgment of a proposal
	Wit              // Threshold witness confirmation of proposal
)

type Message struct {
	from   int      // Which node sent this message
	step   int      // Logical time step this message is for
	typ    Type     // Message type: Prop, Ack, or Wit
	prop   int      // Node whose proposal this Ack or Wit is about
	tkt    int	// Genetic fitness ticket for consensus
	qsc    []Round	// qsc[s] is consensus state for round ending at step s
}

type Node struct {
	Message       	     // Template for messages we send
	comm   chan *Message // Channel to send messages to this node
	save   int           // Earliest step for which we maintain history
	acks   set           // Acknowledgments we've received in this step
	wits   set           // Threshold witnessed messages seen this step
	choice []int         // Best proposal this node chose each round
	commit []bool        // Whether we observed successful commitment
	done   chan struct{} // Run signals this when a node terminates
}

func newNode(self int) (n *Node) {
	n = &Node{}
	n.from = self
	n.prop = self
	n.qsc = make([]Round, 3) // for fake "rounds" ending in steps 0-2
	n.comm = make(chan *Message, 3*len(All)*MaxSteps)
	n.done = make(chan struct{})
	return
}

func (n *Node) run() {
	n.advanceTLC(0) // broadcast message for initial time step

	for MaxSteps == 0 || n.step < MaxSteps {
		msg := <-n.comm   // Receive a message
		n.receiveTLC(msg) // Process it
	}
	n.done <- struct{}{} // signal that we're done
}
