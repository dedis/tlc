// Package qscod implements a simple model verison of the QSCOD algorithm
// for client-driven "on-demand" consensus.
package qscod

import "sync"

type Node int   // Node represents a node number from 0 through n-1
type Step int64 // Step represents a TLC time-step counting from 0

// Hist represents a view of history proposed by some node in a QSC round.
type Hist struct {
	Step Step   // TLC time-step of last successful commit in this view
	Data string // Application data committed at that step in this view
}

// Store represents an interface to one of the n key/value stores
// representing the persistent state of each of the n consensus group members.
// A Store's keys are integer TLC time-steps,
// and its values are Value structures.
//
// WriteRead(step, value) attempts to write value to the store at step,
// returning the first value written by any client and Hist{} for comh.
// If the requested step has been aged out of the store,
// returns the input value unmodified and the last committed history.
// Value.C in each stored value indicates some recent committed history.
//
type Store interface {
	WriteRead(step Step, value Value) (actual Value, comh Hist)
}

// Value represents the values that a consensus node's key/value Store maps to.
type Value struct {
	Pri  int64 // Random priority for this proposal
	C, P Hist  // Last-committed and newly-proposed history views
	R, B Set   // Read set and broadcast set from TLCB
}

// Set represents a set of proposed values from the same time-step.
type Set map[Node]Value

// best returns some maximum-priority history in a Set,
// together with a flag indicating whether the returned history
// is uniquely the best, i.e., the set contains no history tied for best.
func (S Set) best() (bn Node, bv Value, bu bool) {
	for n, v := range S {
		if v.Pri >= bv.Pri {
			bn, bv, bu = n, v, v.Pri > bv.Pri
		}
	}
	return bn, bv, bu
}

// client represents one logical client issuing transactions
// to the consensus group and driving the QSC/TLC state machine forward
// asynchronously across the n key/value stores.
type client struct {
	tr, ts int                     // TLCB thresholds
	kv     []Store                 // Node state key/value stores
	rv     func() int64            // Random priority generator
	mut    sync.Mutex              // Mutex protecting this client's state
	cond   *sync.Cond              // For awaiting threshold conditions
	kvc    map[Step]map[Node]Value // Cache of key/value store values
	rcom   Hist                    // Committed history to return
}

// Commit starts a client with given configuration parameters,
// then requests the client to propose transactions containing
// application-defined message msg until some message commits.
// Returns the history that successfully committed msg.
func Commit(tr, ts int, kvStores []Store, randomPri func() int64,
	proposedData string, committed Hist) Hist {

	// Initialize the client's synchronization and key/value cache state.
	c := &client{tr: tr, ts: ts, kv: kvStores, rv: randomPri}
	c.cond = sync.NewCond(&c.mut)
	c.kvc = make(map[Step]map[Node]Value)

	// Launch one client thread to drive each of the n consensus nodes.
	for i := range kvStores {
		go c.thread(Node(i), proposedData, committed.Step, committed)
	}

	// Wait for some thread to discover the next committed history
	c.mut.Lock()
	for c.rcom.Step == 0 {
		c.cond.Wait()
	}
	c.mut.Unlock()

	// Any slow client threads will continue in the background
	// until they catch up with the others and terminate.

	// Return the next committed history we learned.
	return c.rcom
}

// thread represents the main loop of a client's thread
// that represents and drives a particular consensus group node.
func (c *client) thread(node Node, pref string, step Step, lcom Hist) {
	c.mut.Lock() // Keep state locked while we're not waiting
	defer c.mut.Unlock()

	// Run until we find a commitment at a sufficiently high step number,
	// and then until we're even with all the other client threads.
	// Another thread might deadlock waiting for us if we finish too soon.
	for c.rcom.Step == 0 || c.kvc[step] != nil {

		// Prepare a proposal containing the message msg
		// that this client would like to commit,
		// and invoke TLCB to (try to) issue that proposal on this node.
		v0 := Value{Pri: c.rv(), C: lcom, P: Hist{step, pref}}
		v0, R0, B0 := c.tlcb(node, step, v0)

		// Invoke TLCB again to re-broadcast the best eligible proposal
		// we see in the B0 returned from the first TLCB instance,
		// in attempt to reconfirm (double-confirm) that proposal
		// so that all nodes will *know* that it's been confirmed.
		_, v2, _ := B0.best() // some best confirmed proposal from B0
		v2.R, v2.B = R0, B0
		v2, R2, B2 := c.tlcb(node, step+2, v2)
		R0, B0 = v2.R, v2.B // correct our state from v2 read

		// Unconditionally adopt the best confirmed proposal from R2
		// as our own new (still tentative so far) view of history.
		_, v4, _ := R2.best() // some best confirmed proposal from R2

		// We can't (yet) know for certain if this view is committed,
		// but the last commit it records definitely is committed.
		// Return that as the next commit if we hadn't yet seen it.
		if v4.C.Step > lcom.Step && c.rcom.Step == 0 {
			c.rcom = v4.C      // record that v4.C is committed
			c.cond.Broadcast() // signal the main thread
		}

		// Finally, see if we can determine v4 to have been committed,
		// which happens if v4 is the uniquely-best eligible proposal.
		// This test may succeed only for some nodes even in a round.
		n, b, u := R0.best() // is there a uniquely-best proposal in R0?

		// If b is uniquely-best in R0, we can compare priorities
		// to determine if two values are the same node's proposal.
		if u && b.Pri == v4.Pri && b.Pri == B2[n].Pri &&
			c.rcom.Step == 0 && b.P.Step > lcom.Step {
			c.rcom = b.P       // record that b is committed
			c.cond.Broadcast() // signal the main thread
		}

		step += 4 // Two TLCB instances took two time-steps each
	}
}

// tlcb implements the TLCB algorithm for full-spread threshold broadcast,
// each requiring two TLCR invocations and hence two TLC time-steps.
//
// The provided v0 represents a potential next state for this node,
// but other clients may of course race with this one to set the next state.
// The returned Value represents the next-state value successfully registered
// by whatever client won this race to initiate TLCB at step s.
//
// The returned R and B sets, in contrast, are tentative,
// representing possible threshold receive-set and broadcast-set outcomes
// from this TLCB invocation, computed locally by this client.
// These locally-computed sets cannot be relied on to be definite for this node
// until the values computed from them are committed via Store.WriteRead.
//
func (c *client) tlcb(node Node, step Step, v0 Value) (Value, Set, Set) {

	// First invoke TLCR to (try to) record the desired next-state value,
	// and record the definite winning value and a tentative receive-set.
	v0, v0r := c.tlcr(node, step+0, v0)

	// In the second TLCR invocation,
	// broadcast the set of proposals we received in the first.
	_, v1r := c.tlcr(node, step+1, Value{R: v0r})

	// Using the tentative client-side receive-set from the second TLCR,
	// compute potential receive-set (R) and broadcast-set (B) sets
	// to return from TLCB.
	R, B, Bc := make(Set), make(Set), make([]int, len(c.kv))
	for _, v := range v1r {
		for j, vv := range v.R {
			R[j] = vv          // R has all values we've seen
			Bc[j]++            // How many nodes have seen vv?
			if Bc[j] >= c.ts { // B has only those reaching ts
				B[j] = vv
			}
		}
	}
	return v0, R, B
}

// tlcr implements the TLCR algorithm for receive-threshold broadcast,
// each requiring a single TLC time-step.
//
func (c *client) tlcr(node Node, step Step, v Value) (Value, Set) {

	// Create our key/value cache map for step s if not already created
	if _, ok := c.kvc[step]; !ok {
		c.kvc[step] = make(map[Node]Value)
	}

	// Try to write potential value v, then read that of the client who won.
	// If we found a committed history, either ourselves or virally,
	// then just pretend to do writes until we can synchronize and finish.
	if c.rcom.Step == 0 {
		v, c.rcom = c.kv[node].WriteRead(step, v)
	}

	// Wait until tr client threads insert values into kvc[s],
	// waking up all waiting threads once this condition is satisfied.
	// Don't modify kvc[step] after it reaches the threshold tr,
	// because the code above expects threshold sets to be immutable.
	for len(c.kvc[step]) < c.tr {
		c.kvc[step][node] = v // save value v in our local cache
		if len(c.kvc[step]) < c.tr {
			c.cond.Wait() // wait to reach receive threshold
		} else { // threshold reached
			c.cond.Broadcast() // wake up waiting threads
		}
	}
	return v, c.kvc[step] // return the satisfying value set we found
}
