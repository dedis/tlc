// XXX rename causality or vectime layer
package minnet

import (
	"time"
	"math/rand"
)

// Broadcast a copy of our current message template to all nodes.
func (n *Node) broadcastGossip(msg *Message) {

	// Assign the new message a sequence number
	//println(n.self, "broadcastGossip step", msg.step)
	n.mutex.Lock()
	msg.seq = len(n.log[n.self])		// Assign sequence number
	msg.vec = append(vec{}, n.mat[n.self]...) // Include vector time update
	n.log[n.self] = append(n.log[n.self], msg) // Log the message
	n.mat[n.self][n.self] = len(n.log[n.self]) // Update our matrix clock
	n.mutex.Unlock()

	// We always receive our own message first.
	n.receiveTLC(msg)

	for d, dn := range All {
		if d != n.self {
			dn.comm[n.self] <- msg
		}
	}
}

// Receive a message from the underlying network into the gossip layer.
func (n *Node) receiveGossip(peer int) {
	for  {
		msg := <-n.comm[peer]	// Get next message from this peer
		if msg.typ == Done {
			break
		}
		//println(n.self, "receive from", msg.from, "type", msg.typ)

		// Optionally insert random delays on a message basis
		time.Sleep(time.Duration(rand.Int63n(int64(MaxSleep+1))))

		n.enqueueGossip(msg)
	}
}

// Enqueue a possibly out-of-order message for delivery,
// and actually deliver messages as soon as we can.
func (n *Node) enqueueGossip(msg  *Message) {

	n.mutex.Lock()
	defer func() { n.mutex.Unlock() }()

	// Unicast acknowledgments don't get sequence numbers or reordering.
	if msg.typ == Ack {
		n.recv <- msg	// Pass to node's main goroutine
		// XXX just dispatch to upper layers directly?
		return
	}

	// Update our matrix clock row for the message's sender.
	n.mat[msg.from].max(n.mat[msg.from], msg.vec)

	// Enqueue broadcast message for delivery in causal order.
	for len(n.oom[msg.from]) <= (msg.seq - n.mat[n.self][msg.from]) {
		n.oom[msg.from] = append(n.oom[msg.from], nil)
	}
	n.oom[msg.from][msg.seq - n.mat[n.self][msg.from]] = msg

	// Deliver whatever messages we can consistently with causal order.
	for progress := true; progress; {
		progress = false
		for i := range All {
			if len(n.oom[i]) > 0 && n.oom[i][0] != nil &&
					n.oom[i][0].vec.le(n.mat[n.self]) {
				msg = n.oom[i][0]
				n.log[i] = append(n.log[i], msg)
				n.mat[n.self][i] = len(n.log[i])
				n.oom[i] = n.oom[i][1:]
				n.recv  <- msg
				progress = true
			}
		}
	}
}

func (n *Node) initGossip(self int) {
	n.self = self

	n.recv = make(chan *Message, 3 *  len(All) * MaxSteps)

	n.comm = make([]chan *Message, len(All)) // A comm channel per peer
	for i := range(All) {
		n.comm[i] = make(chan *Message, 3 * len(All) * MaxSteps)
	}

	n.mat = make([]vec, len(All))
	for i := range(n.mat) {
		n.mat[i] = make(vec, len(All))
	}

	n.log = make([][]*Message, len(All))
	n.oom = make([][]*Message, len(All))
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

		n.receiveTLC(msg)
	}

	// Kill all the peer connections
	for i := range(All) {
		n.comm[i] <- &Message{typ: Done}
	}

	n.done <- struct{}{}	// signal that we're done
}

