package model

import "math/rand"

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
	QSC  []Round // qsc[s] is consensus state for round ending at step s
}

type Node struct {
	Message // Template for messages we send

	thres int             // TLC message and witness thresholds
	peer  []chan *Message // Channels to send messages to peers

	acks int // # acknowledgments we've received in this step
	wits int // # threshold witnessed messages seen this step

	// Consensus uses the Rand function to choose "genetic fitness"
	// lottery tickets for each node's proposal in each round.
	// Only the low 63 bits of the returned int64 are used.
	//
	// This defaults to using the system's math/rand.Int63().
	// To tolerate sophisticated network denial-of-service attackers,
	// a full implementation should use cryptographic randomness
	// and hide the tickets from the network using encryption (e.g., TLS).
	//
	// This function must not be changed once the Node is in operation.
	// All nodes must use the same nonnegative random number distribution.
	// Ticket collisions are not a problem as long as they are rare,
	// which is why 63 bits of entropy is sufficient.
	//
	Rand func() int64
}

// Create and initialize a new Node with the specified group configuration.
func NewNode(self, threshold int, peer []chan *Message) (n *Node) {
	return &Node{
		Message: Message{From: self,
			QSC: make([]Round, 3)}, // "rounds" ending in steps 0-2
		thres: threshold,
		peer:  peer,
		Rand:  rand.Int63}
}
