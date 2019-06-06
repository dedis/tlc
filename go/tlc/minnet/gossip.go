package minnet

import (
	"time"
	"math/rand"
)

// Broadcast a copy of our current message template to all nodes.
func (n *Node) broadcastGossip(msg *Message) {

	// Assign the new message a sequence number
	self := n.tmpl.sender
	n.mutex.Lock()
	msg.seq = len(n.log[self])		// Assign sequence number
	msg.vec = append(vec{}, n.mat[self]...)	// Include a vector time update
	n.log[self] = append(n.log[self], msg)	// Log the message
	n.mat[self][self] = len(n.log[self])	// Update our matrix clock cell
	n.mutex.Unlock()

	// We always receive our own message first.
	n.receiveTLC(msg)

	for _, dest := range All {
		dest.comm[self] <- msg
	}
}

// Receive a message from the underlying network into the gossip layer.
func (n *Node) receiveGossip(sender int) {
	for  {
		msg := <-n.comm[sender]	// Get next message from this peer
		if msg.typ == Done {
			break
		}

		// Optionally insert random delays on a message basis
		time.Sleep(time.Duration(rand.Int63n(int64(MaxSleep+1))))

		n.recv  <- msg		// Send to node's main goroutine
	}
}

func (n *Node) initGossip(self int) {

	n.recv = make(chan *Message)

	n.comm = make([]chan *Message, len(All)) // A comm channel per sender
	for i := range(All) {
		n.comm[i] = make(chan *Message, 3 * len(All) * MaxSteps)
	}

	n.mat = make([]vec, len(All))
	for i := range(n.mat) {
		n.mat[i] = make(vec, len(All))
	}

	n.log = make([][]*Message, len(All))
}

// This function implements each node's main event-loop goroutine.
func (n *Node) runGossip(self int) {

	// Spawn a receive goroutine for each peer
	for i := range(All) {
		go n.receiveGossip(i)
	}

	n.advanceTLC(0) // broadcast message for initial time step

	// Run the main loop until we hit the step limit if any
	for MaxSteps == 0 || n.tmpl.step < MaxSteps {

		msg := <-n.recv	// Accept a message from a receive goroutine
		if msg.typ == Done {
			break
		}

		// Update our matrix clock row for the message's sender.
		if msg.typ != Ack {
			n.mutex.Lock()
			n.mat[msg.sender].max(n.mat[msg.sender], msg.vec)
			n.mutex.Unlock()
		}

		n.receiveTLC(msg)
	}

	// Kill all the peer connections
	for i := range(All) {
		n.comm[i] <- &Message{typ: Done}
	}

	n.done <- struct{}{}	// signal that we're done
}

