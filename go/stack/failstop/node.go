package failstop

import (
	"sync"

	. "github.com/dedis/tlc/go/stack/arch"
)


var Threshold int		// TLC and consensus threshold

var MaxSteps int
var MaxTicket int32 = 100	// Amount of entropy in lottery tickets


type Node struct {
	// Network/peering layer
	self	int		// This node's participant number
	peer	[]peer		// How to send messages to each peer
	mutex	sync.Mutex	// Mutex protecting node's protocol stack

	// Causal history layer
	mat	[]vec		// Node's current matrix clock
	oom	[][]*Message	// Out-of-order messages not yet delivered
	log	[][]*logEntry	// Nodes' message received and delivered by seq
	saw	[]set		// Messages each node saw recently
	wit	[]set		// Witnessed messages each node saw recently

	// Threshold time (TLC) layer
	tmpl	Message		// Template for messages we send
	save	int		// Earliest step for which we maintain history
	acks	set		// Acknowledgments we've received in this step
	wits	set		// Threshold witnessed messages seen this step

	// This node's record of QSC consensus history
	choice	[]choice	// Best proposal this node chose each round
}

type peer interface {
	Send(msg *Message)
}

// Per-sequence info each node tracks and logs about all other nodes' histories
type logEntry struct {
	msg	*Message	// Message the node broadcast at this seq
	saw	set		// All nodes' messages the node had seen by then
	wit	set		// Threshold witnessed messages it had seen
}

func (n *Node) init(self int, peer []peer) {
	n.self = self
	n.peer = peer

	n.initCausal()
	n.initTLC()
}

