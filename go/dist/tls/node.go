package dist

import (
	"sync"
)

var Threshold int // TLC and consensus threshold

var MaxSteps int
var MaxTicket int32 = 100 // Amount of entropy in lottery tickets

type Type int // Type of message
const (
	Prop Type = iota // Raw unwitnessed proposal
	Ack              // Acknowledgment of a proposal
	Wit              // Threshold witness confirmation of proposal
)

type Message struct {
	// Network/peering layer
	From int // Which node originally sent this message

	// Causality layer
	Seq int // Node-local sequence number for vector time
	Vec vec // Vector clock update from sender node

	// Threshold time (TLC) layer
	Step   int   // Logical time step this message is for
	Typ    Type  // Message type
	Prop   int   // Proposal Seq this Ack or Wit is about
	Ticket int32 // Genetic fitness ticket for this proposal
}

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
