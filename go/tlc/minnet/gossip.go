// XXX rename causality or vectime layer
package minnet

import (
//	"fmt"
)

// Broadcast a copy of our current message template to all nodes.
func (n *Node) broadcastGossip(msg *Message) {

	//println(n.self, n.tmpl.Step, "broadcastGossip",
	//	"mat", len(n.mat))

	// Assign the new message a sequence number
	msg.Seq = len(n.log[n.self])		// Assign sequence number
	msg.Vec = n.mat[n.self].copy()		// Include vector time update
	n.logGossip(n.self, msg)		// Add msg to our log
	//println(n.self, n.tmpl.Step, "broadcastGossip step", msg.Step,
	//		"typ", msg.Typ, "seq", msg.Seq,
	//		"vec", fmt.Sprintf("%v", msg.Vec))

	// We always receive our own message first.
	n.receiveTLC(msg)

	// Send it to all other peers.
	for dest := range n.peer {
		if dest != n.self {
			n.sendGossip(dest, msg)
		}
	}
}

// Log a peer's message, either our own (just sent)
// or another node's (received and ready to be delivered).
func (n *Node) logGossip(peer int, msg *Message) *logEntry {

	// Update peer's matrix clock and our record of what it saw by msg
	for i := range n.peer {
		//println(i, "mat", len(n.mat), "vec", len(msg.Vec))
		for n.mat[peer][i] < msg.Vec[i] {
			n.sawGossip(peer, n.log[i][n.mat[peer][i]].msg)
			n.mat[peer][i]++
		}
	}
	n.sawGossip(peer, msg)	// msg has been seen by the peer that sent it
	n.sawGossip(n.self, msg) // and now we've seen the message too

	ent := logEntry{msg, n.saw[peer].copy(0), n.wit[peer].copy(0)}
	n.log[peer] = append(n.log[peer], &ent)	// record log entry
	n.mat[n.self][peer] = len(n.log[peer])	// update our vector time
	return &ent
}

// Record the fact that a given peer is now known to have seen a given message.
// For Wit messages, record the fact that the proposal was threshold witnessed.
func (n *Node) sawGossip(peer int, msg *Message) {
	n.saw[peer].add(msg)
	if msg.Typ == Wit {
		prop := n.log[msg.From][msg.Prop].msg
		if prop.Typ != Prop { panic("not a proposal!") }
		n.wit[peer].add(prop)
	}
}

// Transmit a message to a particular node.
func (n *Node) sendGossip(dest int, msg *Message) {
	//println(n.self, n.tmpl.Step, "sendGossip to", dest, "typ", msg.Typ,
	//	"seq", msg.Seq)
	n.peer[dest].Send(msg)
}

// Receive a possibly out-of-order message from the network.
// Enqueue it and actually deliver messages as soon as we can.
func (n *Node) receiveGossip(msg *Message) {

	// Unicast acknowledgments don't get sequence numbers or reordering.
	if msg.Typ == Ack {
		n.receiveTLC(msg)	// Just send it up the stack
		return
	}

	// Ignore duplicate message deliveries
	if msg.Seq < n.mat[n.self][msg.From] {
		println(n.self, n.tmpl.Step, "duplicate message from", msg.From,
			"seq", msg.Seq)
		panic("XXX")
		return
	}

	// Enqueue broadcast message for delivery in causal order.
	//println(n.self, n.tmpl.Step, "receiveGossip from", msg.From,
	//	"type", msg.Typ, "seq", msg.Seq,
	//	"vec", fmt.Sprintf("%v", msg.Vec))
	//if len(n.oom[msg.From]) <= msg.Seq - n.mat[n.self][msg.From] - 1000 {
	//	panic("huge jump")
	//}
	for len(n.oom[msg.From]) <= msg.Seq - n.mat[n.self][msg.From] {
		n.oom[msg.From] = append(n.oom[msg.From], nil)
	}
	n.oom[msg.From][msg.Seq - n.mat[n.self][msg.From]] = msg

	// Deliver whatever messages we can consistently with causal order.
	for progress := true; progress; {
		progress = false
		for i := range n.peer {
			if len(n.oom[i]) > 0 && n.oom[i][0] != nil &&
					n.oom[i][0].Vec.le(n.mat[n.self]) {
				//println(n.self, n.tmpl.Step, "enqueueGossip",
				//	"deliver type", msg.Typ,
				//	"seq", msg.Seq, "#oom", len(n.oom[i]))
				if n.oom[i][0].Seq != len(n.log[i]) {
					panic("out of sync")
				}
				ent := n.logGossip(i, n.oom[i][0])
				if len(n.log[i]) != ent.msg.Seq+1 {
					panic("out of sync")
				}
				n.oom[i] = n.oom[i][1:]
				//println("  new #oom", len(n.oom[i]))
				n.receiveTLC(ent.msg)
				progress = true
			}
		}
	}
}

func (n *Node) initGossip() {
	n.mat = make([]vec, len(n.peer))
	n.oom = make([][]*Message, len(n.peer))
	n.log = make([][]*logEntry, len(n.peer))
	n.saw = make([]set, len(n.peer))
	n.wit = make([]set, len(n.peer))
	for i := range n.peer {
		n.mat[i] = make(vec, len(n.peer))
		n.saw[i] = make(set)
		n.wit[i] = make(set)
	}

	n.initTLC()
}

