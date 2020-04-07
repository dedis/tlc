// Package qscod implements a simple model verison of the QSCOD algorithm
// for client-driven "on-demand" consensus.
package qscod

import "sync"

type Node int   // Node represents a node number from 0 through n-1
type Step int64 // Step represents a TLC time-step counting from 0

// Hist represents the head of a node's history, either tentative or finalized.
type Hist struct {
	node Node   // Node that proposed this history
	step Step   // TLC time-step number this history is for
	pred *Hist  // Predecessor in previous time-step
	msg  string // Application message in this proposal
	pri  int64  // Random priority value
}

// Set represents a set of proposed histories from the same time-step.
type Set map[Node]*Hist

// best returns some maximum-priority history in a Set,
// together with a flag indicating whether the returned history
// is uniquely the best, i.e., the set contains no history tied for best.
func (S Set) best() (*Hist, bool) {
	b, u := &Hist{pri: -1}, false
	for _, h := range S {
		if h.pri >= b.pri {
			b, u = h, !(h.pri == b.pri)
		}
	}
	return b, u
}

// Store represents an interface to one of the n key/value stores
// representing the persistent state of each of the n consensus group members.
// A Store's keys are integer TLC time-steps,
// and its values are Val structures.
type Store interface {
	WriteRead(step Step, value Val, commit bool) (Step, Val)
}

// Val represents the values that a consensus node's key/value Store maps to.
type Val struct {
	H, Hp *Hist
	R, B  Set
}

// Client represents one logical client issuing transactions
// to the consensus group and driving the QSC/TLC state machine forward
// asynchronously across the n key/value stores.
type Client struct {
	tr, ts int          // TLCB thresholds
	kv     []Store      // Node state key/value stores
	rv     func() int64 // Function to generate random priority values

	mut  sync.Mutex // Mutex protecting the state of this client
	cond *sync.Cond // Condition variable for cross-thread synchronization

	kvc map[Step]map[Node]Val // Cache of key/value store values
	comh *Hist  // Last committed history

	msg  string // message we want to commit, "" if none
	msgh *Hist  // history when our msg was committed
}

// Commit starts a Client with given configuration parameters,
// then requests the client to propose transactions containing
// application-defined message msg, repeatedly if necessary,
// until some proposal containing msg successfully commits.
// Returns the history that successfully committed msg.
func (c *Client) Commit(tr, ts int, kv []Store, rv func() int64, msg string) *Hist {
	c.tr, c.ts, c.kv, c.rv, c.msg = tr, ts, kv, rv, msg

	// Initialize the client's synchronization and key/value cache state.
	c.cond = sync.NewCond(&c.mut)
	c.kvc = make(map[Step]map[Node]Val)
	c.comh = &Hist{} // placeholder for last committed history

	// Launch one client thread to drive each of the n consensus nodes.
	for i := range kv {
		go c.thread(Node(i))
	}

	c.mut.Lock() // keep state locked while we're not waiting
	for c.msgh == nil {
		c.cond.Wait() // wait until msg has been committed
	}
	c.mut.Unlock()

	return c.msgh
}

// thread represents the main loop of a Client's thread
// that represents and drives a particular consensus group node.
func (c *Client) thread(node Node) {
	c.mut.Lock() // Keep state locked while we're not waiting
	s := Step(0)
	h := (*Hist)(nil) // First proposal has no predecessor

	// Run as long as needed until we can commit our assigned message.
	for c.msgh == nil {

		// Prepare a proposal containing the message msg
		// that this Client would like to commit,
		// and invoke TLCB to (try to) issue that proposal on this node.
		v0 := Val{H: h, Hp: &Hist{node, s, h, c.msg, c.rv()}}
		v0, R0, B0 := c.tlcb(node, s+0, v0)
		h = v0.H // correct our state from v0 read

		// Invoke TLCB again to re-broadcast the best eligible proposal
		// we see emerging from the first TLCB instance,
		// in attempt to reconfirm (double-confirm) that proposal
		// so that all nodes will *know* that it's been confirmed.
		v2 := Val{R: R0, B: B0}
		v2.Hp, _ = B0.best() // some best confirmed proposal from B0
		v2, R2, B2 := c.tlcb(node, s+2, v2)
		R0, B0 = v2.R, v2.B // correct our state from v2 read

		h, _ = R2.best()  // some best confirmed proposal from R2
		b, u := R0.best() // is there a uniquely-best proposal in R0?
		if B2[h.node] == h && b == h && u {
			c.comh = h         // record that h is committed
			if h.msg == c.msg {	// committed our message!
				c.msgh = h
				c.cond.Broadcast() // signal Commit method
			}
		}

		s += 4 // Two TLCB instances took two time-steps each
	}
	c.mut.Unlock()
}

// tlcb implements the TLCB algorithm for full-spread threshold broadcast,
// each requiring two TLCR invocations and hence two TLC time-steps.
//
// The provided v0 represents a potential next state for this node,
// but other clients may of course race with this one to set the next state.
// The returned Val represents the next-state value successfully registered
// by whatever client won this race to initiate TLCB at step s.
//
// The returned R and B sets, in contrast, are tentative,
// representing possible threshold receive-set and broadcast-set outcomes
// from this TLCB invocation, computed locally by this client.
// These locally-computed sets cannot be relied on to be definite for this node
// until the values computed from them are committed via Store.WriteRead.
//
func (c *Client) tlcb(node Node, s Step, v0 Val) (Val, Set, Set) {

	// First invoke TLCR to (try to) record the desired next-state value,
	// and record the definite winning value and a tentative receive-set.
	v0, v0r := c.tlcr(node, s+0, v0)

	// Prepare a value to broadcast in the second TLCR invocation,
	// indicating which proposals we received from the first.
	v1 := Val{R: make(Set)}
	for i, v := range v0r {
		v1.R[i] = v.Hp
	}
	v1, v1r := c.tlcr(node, s+1, v1)

	// Using the tentative client-side receive-set from the second TLCR,
	// compute potential receive-set (R) and broadcast-set (B) sets
	// to return from TLCB.
	R, B, Bc := make(Set), make(Set), make([]int, len(c.kv))
	for _, v := range v1r {
		for j, h := range v.R {
			R[j] = h           // R has all histories we've seen
			Bc[j]++            // How many nodes have seen h?
			if Bc[j] >= c.ts { // B has only those reaching ts
				B[j] = h
			}
		}
	}
	return v0, R, B
}

// tlcr implements the TLCR algorithm for receive-threshold broadcast,
// each requiring a single TLC time-step.
//
func (c *Client) tlcr(node Node, s Step, v Val) (Val, map[Node]Val) {

	// Create our key/value cache map for step s if not already created
	if _, ok := c.kvc[s]; !ok {
		c.kvc[s] = make(map[Node]Val)
	}

	// Try to write potential value v, then read that of the client who won
	_, v = c.kv[node].WriteRead(s, v, v.H == c.comh)
	c.kvc[s][node] = v // save value v in our local cache

	// Wait until tr client threads insert values into kvc[s],
	// waking up all waiting threads once this condition is satisfied.
	if len(c.kvc[s]) == c.tr {
		c.cond.Broadcast() // wake up waiting threads
	}
	for len(c.kvc[s]) < c.tr {
		c.cond.Wait() // wait to reach receive threshold
	}
	return v, c.kvc[s] // return some satisfying value set
}
