package dist

import (
	"sync"
)

// Threshold is the TLC and consensus threshold
var Threshold int

// MaxTicket is the Amount of entropy in lottery tickets
var MaxTicket int32 = 100

// Type of message
type Type int

const (
	// Prop is a raw unwitnessed proposal
	Prop Type = iota
	// Ack is an acknowledgment of a proposal
	Ack
	// Wit is a threshold witness confirmation of proposal
	Wit
)

// Message over the network
type Message struct {
	// Network/peering layer

	// From designates the node which originally sent this message
	From int

	// Causality layer
	// Seq is the Node-local sequence number for vector time
	Seq int
	// Vev is the Vector clock update from sender node
	Vec vec

	// Threshold time (TLC) layer
	// Step is the logical time step this message is for
	Step int
	// typ is the message type
	Typ Type
	// Prop is the proposal Seq this Ack or Wit is about
	Prop int
	// Ticket is the genetic fitness ticket for this proposal
	Ticket int32
}

// Node definition
type Node struct {
	// Network/peering layer
	self  int        // This node's participant number
	peer  []peer     // How to send messages to each peer
	mutex sync.Mutex // Mutex protecting node's protocol stack

	// Causal history layer
	mat    []vec        // Node's current matrix clock
	oom    [][]*Message // Out-of-order messages not yet delivered
	seqLog [][]*Message // Nodes' message received and delivered by seq
	saw    []set        // Messages each node saw recently
	wit    []set        // Witnessed messages each node saw recently

	// Threshold time (TLC) layer
	tmpl    Message      // Template for messages we send
	save    int          // Earliest step for which we maintain history
	acks    set          // Acknowledgments we've received in this step
	wits    set          // Threshold witnessed messages seen this step
	stepLog [][]logEntry // Nodes' messages seen by start of recent steps

	// This node's record of QSC consensus history
	choice []choice // Best proposal this node chose each round
}

type peer interface {
	Send(msg *Message)
}

// Info each node logs about other nodes' views at the start of each time-step
type logEntry struct {
	saw set // All nodes' messages the node had seen by then
	wit set // Threshold witnessed messages it had seen
}

// Record of one node's QSC decision in one time-step
type choice struct {
	best   int  // Best proposal this node chose in this round
	commit bool // Whether node observed successful commitment
}

func (n *Node) init(self int, peer []peer) {
	n.self = self
	n.peer = peer

	n.initCausal()
	n.initTLC()
}
