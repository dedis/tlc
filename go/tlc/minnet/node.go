package minnet

import (
	"time"
	"sync"
	"io"
	"bufio"
	"encoding/gob"
)


var Threshold int		// TLC and consensus threshold
var All []*Node			// List of all nodes

var MaxSteps int
var MaxSleep time.Duration
var MaxTicket int32 = 100	// Amount of entropy in lottery tickets


type Type int			// Type of message
const (
	Prop Type = iota	// Raw unwitnessed proposal
	Ack			// Acknowledgment of a proposal
	Wit			// Threshold witness confirmation of proposal
)

type Message struct {
	// Network/peering layer
	From	int		// Which node originally sent this message

	// Causality layer
	Seq	int		// Node-local sequence number for vector time
	Vec	vec		// Vector clock update from sender node

	// Threshold time (TLC) layer
	Step	int		// Logical time step this message is for
	Typ	Type		// Message type
	Prop	int		// Proposal Seq this Ack or Wit is about
	Ticket	int32		// Genetic fitness ticket for this proposal
}

type Node struct {
	self	int		// This node's participant number
	peer	[]peer		// Channels to send messages to this node
	recv	chan *Message	// Node-internal message receive channel
	mutex	sync.Mutex	// Mutex protecting node's protocol stack

	tmpl	Message		// Template for messages we send
	save	int		// Earliest step for which we maintain history
	acks	set		// Acknowledgments we've received in this step
	wits	set		// Threshold witnessed messages seen this step

	mat	[]vec		// Node's current matrix clock
	oom	[][]*Message	// Out-of-order messages not yet delivered
	log	[][]*logEntry	// Nodes' message received and delivered by seq
	saw	[]set		// Messages each node saw recently
	wit	[]set		// Witnessed messages each node saw recently

	// This node's record of QSC consensus history
	choice	[]*Message	// Best proposal this node chose each round
	commit	[]bool		// Whether we observed successful commitment

	done	sync.WaitGroup	// Barrier to synchronize goroutine termination
}

// Per-sequence info each node tracks and logs about all other nodes' histories
type logEntry struct {
	msg	*Message	// Message the node broadcast at this seq
	saw	set		// All nodes' messages the node had seen by then
	wit	set		// Threshold witnessed messages it had seen
}

type peer struct {
	wr	*io.PipeWriter	// Write end of communication pipe
	rd	*io.PipeReader	// Read end of communication pipe
	bwr	*bufio.Writer	// Buffer for write end
	brd	*bufio.Reader	// Buffer for read end
	enc	*gob.Encoder	// Encoder into write end of communication pipe
	dec	*gob.Decoder	// Decoder from read end of communication pipe
}

func newNode(self int) (n *Node) {
	n = &Node{}
	n.self = self
	n.peer = make([]peer, len(All))
	for i := range(All) {
		n.peer[i].rd, n.peer[i].wr = io.Pipe()
		n.peer[i].bwr = bufio.NewWriter(n.peer[i].wr)
		n.peer[i].brd = bufio.NewReader(n.peer[i].rd)
		n.peer[i].enc = gob.NewEncoder(n.peer[i].bwr)
		n.peer[i].dec = gob.NewDecoder(n.peer[i].brd)
	}

	n.initGossip()
	n.tmpl = Message{From: self, Step: 0}
	return
}

// Initialize and run the model for a given threshold and number of nodes.
func Run(threshold, nnodes int) {

	//println("Run config", threshold, "of", nnodes)

	// Initialize the nodes
	Threshold = threshold
	All = make([]*Node, nnodes)
	for i := range All {
		All[i] = newNode(i)
	}

	// Run all the nodes asynchronously on separate goroutines
	for i, n := range All {
		n.done.Add(1)
		go n.runGossip(i)
	}

	// Wait for all the nodes to complete their execution
	for _, n := range All {
		n.done.Wait()
	}
}

