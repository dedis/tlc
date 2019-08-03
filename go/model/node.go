package model

import "math/rand"

// Type represents the type of a QSC message: either Raw, Ack, or Wit.
//
// At the start of each time step a node broadcasts a Raw message,
// which proposes a block for the consensus round starting at this step
// and solicits witness acknowledgments to the proposal.
//
// Nodes that receive a Raw message during the same time step
// reply with a unicast Ack message to Raw message's sender,
// acknowledging that they have seen the sender's proposal
// and merged in its QSC state.
//
// Once a node has received a threshold of Ack messages to its Raw proposal,
// the node broadcasts a Wit message to announce that its proposal is witnessed.
// Nodes wait to collect a threshold of Wit messages as their condition
// to advance to the next time step and broadcast their next Raw message.
//
type Type int

const (
	Raw Type = iota // Raw unwitnessed QSC proposal
	Ack             // Acknowledgment of a proposal
	Wit             // Threshold witness confirmation of a proposal
)

// Message contains the information nodes must pass in messages
// both to run the TLC clocking protocol and achieve QSC consensus.
//
// This implementation of QSC performs no message marshaling or unmarshalling;
// the client using it must handle message wire-format serialization.
// However, the Message struct is defined so as to be compatible with
// standard Go encoders such as encoding/gob or encoding/json.
// The client may also marshal/unmarshal its own larger message struct
// containing a superset of the information here,
// such as to attach semantic content in some form to consensus proposals.
type Message struct {
	From int     // Node number of node that sent this message
	Step int     // Logical time step this message is for
	Type Type    // Message type: Prop, Ack, or Wit
	Tkt  uint64  // Genetic fitness ticket for consensus
	QSC  []Round // QSC consensus state for rounds ending at Step or later
}

// Node contains per-node state and configuration for TLC and QSC.
// Use NewNode to create and properly initialize an instance
// with the mandatory configuration parameters.
// Public fields in this struct are optional configuration settings,
// which NewNode initializes to defaults but the caller may change
// after calling NewNode but before commencing protocol execution.
//
// Consensus uses the configurable Rand function to choose "genetic fitness"
// lottery tickets for each node's proposal in each round.
// Only the low 63 bits of the returned int64 are used.
// This defaults to using the system's math/rand.Int63().
// To tolerate sophisticated network denial-of-service attackers,
// a full implementation should use cryptographic randomness
// and hide the tickets from the network using encryption (e.g., TLS).
//
// The Rand function must not be changed once the Node is in operation.
// All nodes must use the same nonnegative random number distribution.
// Ticket collisions are not a problem as long as they are rare,
// which is why 63 bits of entropy is sufficient.
//
type Node struct {
	m Message // Template for messages we send

	thres int                          // TLC message and witness thresholds
	nnode int                          // Total number of nodes
	send  func(peer int, msg *Message) // Function to send message to a peer

	acks int // # acknowledgments we've received in this step
	wits int // # threshold witnessed messages seen this step

	Rand func() int64 // Function to generate random genetic fitness tickets
}

// Create and initialize a new Node with the specified group configuration.
// The parameters to NewNode are the mandatory Node configuration parameters:
// self is this node's number, thres is the TLC message and witness threshold,
// nnode is the total number of nodes,
// and send is a function to send a Message to a given peer node number.
//
// Optional configuration is represented by fields in the created Node struct,
// which the caller may modify before commencing the consensus protocol.
//
func NewNode(self, thres, nnode int, send func(peer int, msg *Message)) (n *Node) {
	return &Node{
		m: Message{From: self,
			QSC: make([]Round, 3)}, // "rounds" ending in steps 0-2
		thres: thres, nnode: nnode, send: send,
		Rand: rand.Int63}
}
