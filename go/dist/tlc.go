package dist

import (
	"math/rand"
)

// Initialize the TLC layer state in a Node
func (n *Node) initTLC() {
	n.tmpl = Message{From: n.self, Step: -1}
	n.stepLog = make([][]logEntry, len(n.peer))
}

// Broadcast a copy of our current message template to all nodes
func (n *Node) broadcastTLC() *Message {

	//println(n.self, n.tmpl.Step, "broadcast", msg, "typ", msg.Typ)
	msg := n.tmpl
	n.broadcastCausal(&msg)
	return &msg
}

// Unicast an acknowledgment of a given proposal to its sender
func (n *Node) acknowledgeTLC(prop *Message) {

	msg := n.tmpl
	msg.Typ = Ack
	msg.Prop = prop.Seq
	n.sendCausal(prop.From, &msg)
}

// Advance to a new time step.
func (n *Node) advanceTLC(step int) {
	//println(n.self, step, "advanceTLC",
	//	"saw", len(n.saw[n.self]), "wit", len(n.wit[n.self]))

	// Initialize our message template for new time step
	n.tmpl.Step = step                     // Advance to new time step
	n.tmpl.Typ = Prop                      // Raw unwitnessed proposal message initially
	n.tmpl.Ticket = rand.Int31n(MaxTicket) // Choose a ticket

	n.acks = make(set) // No acknowledgments received yet in this step
	n.wits = make(set) // No threshold witnessed messages received yet

	// Notify the upper (QSC) layer of the advancement of time,
	// and let it fill in its part of the new message to broadcast.
	n.advanceQSC(n.saw[n.self], n.wit[n.self])

	prop := n.broadcastTLC() // broadcast our raw proposal
	n.tmpl.Prop = prop.Seq   // save proposal's sequence number
	n.acks.add(prop)         // automatically self-acknowledge  it
}

func (n *Node) receiveTLC(msg *Message) {

	// Now process this message according to type.
	//println(n.self, n.tmpl.Step, "receivedTLC from", msg.From,
	//	"step", msg.Step, "typ", msg.Typ)
	switch msg.Typ {
	case Prop: // A raw unwitnessed proposal broadcast.

		// Record the set of messages this node had seen
		// by the time it advanced to this new time-step.
		if len(n.stepLog[msg.From]) != msg.Step {
			panic("out of sync")
		}
		n.stepLog[msg.From] = append(n.stepLog[msg.From],
			logEntry{n.saw[msg.From], n.wit[msg.From]})

		// Continue from pruned copies in the next time step
		n.saw[msg.From] = n.saw[msg.From].copy(n.save)
		n.wit[msg.From] = n.wit[msg.From].copy(n.save)

		if msg.Step == n.tmpl.Step {
			//println(n.self, n.tmpl.Step, "ack", msg.From)
			n.acknowledgeTLC(msg)
		}

	case Ack: // An acknowledgment. Collect a threshold of acknowledgments.
		if msg.Prop == n.tmpl.Prop { // only if it acks our proposal
			n.acks.add(msg)
			//println(n.self, n.tmpl.Step,  "got ack", len(n.acks))
			if n.tmpl.Typ == Prop && len(n.acks) >= Threshold {

				// Broadcast a threshold-witnesed certification
				n.tmpl.Typ = Wit
				n.broadcastTLC()
			}
		}

	case Wit: // A threshold-witnessed message. Collect a threshold of them.
		prop := n.seqLog[msg.From][msg.Prop]
		if prop.Typ != Prop {
			panic("doesn't refer to a proposal!")
		}
		if msg.Step == n.tmpl.Step {

			// Collect a threshold of Wit witnessed messages.
			n.wits.add(prop) // witnessed messages in this step
			if len(n.wits) >= Threshold {

				// We've met the condition to advance time.
				n.advanceTLC(n.tmpl.Step + 1)
			}
		}
	}
}
