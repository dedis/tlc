package dist

import (
	"math/rand"
)

var Threshold int // TLC and consensus threshold
var All []*Node   // List of all nodes

var MaxSteps int          // Max number of consensus rounds to run

type Type int // Type of message
const (
	Raw Type = iota // Raw unwitnessed proposal
	Ack             // Acknowledgment of a proposal
	Wit             // Threshold witness confirmation of proposal
)

type Message struct {
	from int     // Which node sent this message
	step int     // Logical time step this message is for
	typ  Type    // Message type: Prop, Ack, or Wit
	tkt  int64   // Genetic fitness ticket for consensus
	qsc  []Round // qsc[s] is consensus state for round ending at step s
}

type Node struct {
	Message               // Template for messages we send

	// Consensus uses the Rand function to choose "genetic fitness"
	// lottery tickets for each node's proposal in each round.
	//
	// This defaults to the system's math.rand.Int63() generator.
	// Cryptographic random numbers should be used instead
	// if strong, intelligent network adversaries are anticipated.
	//
	// All nodes must use the same random number distribution.
	// Ticket collisions are not a problem as long as they are rare,
	// which is why 64 bits of entropy is sufficient.
	//
	Rand	func() int64

	comm    chan *Message // Channel to send messages to this node
	acks    int           // # acknowledgments we've received in this step
	wits    int           // # threshold witnessed messages seen this step
	done    chan struct{} // Run signals this when a node terminates
}

func newNode(self int) (n *Node) {
	return &Node{
		Message: Message{from: self,
			qsc: make([]Round, 3)}, // "rounds" ending in steps 0-2
		Rand: rand.Int63,	// Default random ticket generator
		comm: make(chan *Message, 3*len(All)*MaxSteps),
		done: make(chan struct{})}
}

func (n *Node) run() {
	n.advanceTLC(0) // broadcast message for initial time step

	for MaxSteps == 0 || n.step < MaxSteps {
		msg := <-n.comm   // Receive a message
		n.receiveTLC(msg) // Process it
	}
	n.done <- struct{}{} // signal that we're done
}
