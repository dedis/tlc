package dist

import (
	"math/rand"
)

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

// Peer is the interface to a peer node in the consensus group.
type Peer interface {
	Send(msg *Message) // Transmit a Message to this peer
}

type Node struct {
	Message // Template for messages we send

	thres int    // TLC message and witness thresholds
	peer  []Peer // List of peers in node number order

	acks int // # acknowledgments we've received in this step
	wits int // # threshold witnessed messages seen this step

	// Optional configuration parameters with defaults below:

	// This function broadcasts a Message to all Peers including ourself.
	// By default, this simply calls peer.Send on each peer individually,
	// but this default may be changed if efficient broadcast is available.
	//
	Broadcast func(msg *Message)

	// Consensus uses the Rand function to choose "genetic fitness"
	// lottery tickets for each node's proposal in each round.
	//
	// This defaults to the system's math.rand.Int63() generator.
	// Cryptographic random numbers should be used instead
	// if strong, intelligent network adversaries are anticipated.
	//
	// This function must not be changed once the Node is in operation.
	// All nodes must use the same nonnegative random number distribution.
	// Ticket collisions are not a problem as long as they are rare,
	// which is why 64 bits of entropy is sufficient.
	//
	Rand func() int64
}

// Create and initialize a new Node with the specified group configuration.
func NewNode(self, threshold int, peers []Peer) (n *Node) {
	return &Node{
		Message: Message{from: self,
			qsc: make([]Round, 3)}, // "rounds" ending in steps 0-2
		peer: peers,
		Broadcast: func(msg *Message) {
			for _, dest := range peers {
				dest.Send(msg)
			}
		},
		Rand: rand.Int63} // Default random ticket generator
}
