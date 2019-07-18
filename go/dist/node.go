package dist

import (
	"crypto/rand"
	"encoding/binary"
)

type Type int // Type of message
const (
	Raw Type = iota // Raw unwitnessed proposal
	Ack             // Acknowledgment of a proposal
	Wit             // Threshold witness confirmation of proposal
)

type Message struct {
	From int     // Which node sent this message
	Step int     // Logical time step this message is for
	Type Type    // Message type: Prop, Ack, or Wit
	Tkt  uint64  // Genetic fitness ticket for consensus
	QSC  []Round // QSC[s] holds QSC state for round ending at step s
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
	// Only the low 63 bits of the returned uint64 are used.
	//
	// This defaults to a function that generates lottery tickets
	// using the system's cryptographic random number generator.
	// The random source needs to be cryptographically strong to ensure
	// protection against sophisticated network DoS attackers.
	//
	// This function must not be changed once the Node is in operation.
	// All nodes must use the same nonnegative random number distribution.
	// Ticket collisions are not a problem as long as they are rare,
	// which is why 63 bits of entropy is sufficient.
	//
	Rand func() uint64
}

// Create and initialize a new Node with the specified group configuration.
func NewNode(self, threshold int, peers []Peer) (n *Node) {
	return &Node{
		Message: Message{From: self,
			QSC: make([]Round, 3)}, // "rounds" ending in steps 0-2
		peer: peers,
		Broadcast: func(msg *Message) { // Default broadcast function
			for _, dest := range peers {
				dest.Send(msg)
			}
		},
		Rand: func() uint64 { // Default random ticket generator
			var b [8]byte
			if _, err := rand.Read(b[:]); err != nil {
				panic(err)
			}
			return binary.BigEndian.Uint64(b[:])
		}}
}
