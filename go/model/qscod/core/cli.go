// Package core implements the minimal core of the QSCOD consensus algorithm.
// for client-driven "on-demand" consensus.
//
// This implementation of QSCOD builds on the TLCB and TLCR
// threshold logical clock algorithms.
// These algorithms are extremely simple but do impose one constraint:
// the number of failing nodes must be at most one-third the group size.
//
// The unit tests for this package is in the test sub-package,
// so that useful test framework code can be shared with other packages
// without requiring any of it to be imported into development builds.
// (Too bad Go doesn't allow packages to export and import test code.)
//
package core

import "fmt"
import "sync"
import "context"

type Node int   // Node represents a node number from 0 through n-1
type Step int64 // Step represents a TLC time-step counting from 0

// Head represents a view of history proposed by some node in a QSC round.
type Head struct {
	Step Step   // TLC time-step of last successful commit in this view
	Data string // Application data committed at that step in this view
}

// Store represents an interface to one of the n key/value stores
// representing the persistent state of each of the n consensus group members.
// A Store's keys are integer TLC time-steps,
// and its values are Value structures.
//
// WriteRead(step, value) attempts to write tv to the store at step v.P.Step,
// returning the first value written by any client.
// WriteRead may also return a value from any higher time-step,
// if other clients have moved the store's state beyond v.P.Step.
//
// This interface intentionally provides no means to return an error.
// If WriteRead encounters an error that might be temporary or recoverable,
// then it should just keep trying (perhaps with appropriate backoff).
// This is the fundamental idea of asynchronous fault tolerant consensus:
// to tolerate individual storage node faults, persistently without giving up,
// waiting for as long as it takes for the store to become available again.
//
// If the application encounters an error that warrants a true global failure,
// then it should arrange for the Up function to return an error,
// which will eventually cause all the worker threads to terminate.
// In this case, the application can cancel any active WriteRead calls,
// which may simply return the value v that was requested to be written
// in order to allow the per-node worker thread to terminate cleanly.
//
type Store interface {
	WriteRead(v Value) Value
}

// Value represents the values that a consensus node's key/value Store maps to.
type Value struct {
	I       int64 // Random integer priority for this proposal
	L, C, P Head  // Last and current commit, and proposed history views
	R, B    Set   // Read set and broadcast set from TLCB
}

// Set represents a set of proposed values from the same time-step.
type Set map[Node]Value

// best returns some maximum-priority history in a Set,
// together with a flag indicating whether the returned history
// is uniquely the best, i.e., the set contains no history tied for best.
func (S Set) best() (bn Node, bv Value, bu bool) {
	for n, v := range S {
		if v.I >= bv.I {
			bn, bv, bu = n, v, v.I > bv.I
		}
	}
	return bn, bv, bu
}

// Client represents a logical client that can propose transactions
// to the consensus group and drive the QSC/TLC state machine forward
// asynchronously across the key/value storesu defining the group's state.
//
// The caller must initialize the public variables
// to represent a valid QSCOD configuration,
// before invoking Client.Run to run the consensus algorithm.
// The public configuration variables must not be changed
// after starting the client.
//
// KV is a slice containing interfaces to each of the key/value stores
// that hold the persistent state of each node in the consensus group.
// The total number of nodes N is defined to be len(KV).
//
// Tr and Ts are the receive and spread thresholds, respectively.
// To ensure liveness against up to F slow or crashed nodes,
// the receive threshold must exclude the F failed nodes: i.e., Tr <= N-F.
// To ensure consistency (safety), the constrant Tr+Ts > N must hold.
// Finally, to ensure that each round enjoys a high probability
// of successful commitment, it should be the case that N >= 3F.
// Thus, given F and N >= 3F, it is safe to set Tr = N-F and Ts = N-Tr+1.
// The precise minimum threshold requirements are slightly more subtle,
// but this is a safe and simpler configuration rule.
//
// Up is a callback function that the Client calls regularly while running,
// to update the caller's knowledge of committed transactions
// and to update the proposal data the client attempts to commit.
// Client passes to Up the Step numbers and proposal Data strings
// for the last (predecessor) and current states known to be committed.
// This known-committed proposal will change regularly across calls,
// but may not change on each call and may not even be monotonic.
// The Up function returns a string representing the new preferred
// proposal Data that the Client will subsequently attempt to commit.
// The Up function also returns an error which, if non-nil,
// causes the Client's operation to terminate and return that error.
//
// RV is a function to generate non-negatative random numbers
// for the symmetry-breaking priority values QSCOD requires.
// In a production system, these random numbers should have high entropy
// for maximum performance (minimum likelihood of collisions),
// and should be generated from a cryptographically strong private source
// for maximum protection against denial-of-service attacks in the network.
//
type Client struct {
	KV     []Store                // Node state key/value stores
	Tr, Ts int                    // Receive and spread thresholds
	Pr     func(i Node, L, C Head) (string, int64) // Proposal function
//	RV	func() int64	// Random priority generator

	mut sync.Mutex // Mutex protecting this client's state
}

type work struct {
	kvc  Set        // Key/value cache collected for this time-step
	cond *sync.Cond // For awaiting threshold conditions
	max  Value      // Value with highest time-step we must catch up to
	next *work      // Forward pointer to next work item
}

// Run starts a client running with its given configuration parameters,
// proposing transactions and driving the consensus state machine continuously
// forever or until the passed context is cancelled.
//
func (c *Client) Run(ctx context.Context) (err error) {

	// Keep the mutex locked whenever we're not waiting.
	c.mut.Lock()
	defer c.mut.Unlock()

	// Launch one client thread to drive each of the n consensus nodes.
	lw := &work{kvc: make(Set), cond: sync.NewCond(&c.mut)}
	for i := range c.KV {
		go c.thread(ctx, Node(i), 0, Value{}, lw)
	}

	// Drive consensus state forever or until our context gets cancelled.
	for ctx.Err() == nil {

		// Set the next work-item in the current last work-item.
		lw.next = &work{kvc: make(Set), cond: sync.NewCond(&c.mut)}

		// Wake up any threads waiting for a next item to appear
		lw.cond.Broadcast()

		// Wait for a threshold number of worker threads
		// to complete the last work item
		for len(lw.kvc) < c.Tr {
			lw.cond.Wait()
		}

		// Move on to process the next work item
		lw = lw.next
	}

	// Signal the worker threads to terminate with an all-nil work-item
	lw.next = &work{}
	lw.cond.Broadcast()

	// Any slow client threads will continue in the background
	// until they catch up with the others or successfully get cancelled.
	return ctx.Err()
}

// thread represents the main loop of a client's thread
// that represents and drives a particular consensus group node.
func (c *Client) thread(ctx context.Context, node Node, ls Step, lv Value, lw *work) {

	c.mut.Lock() // Keep state locked while we're not waiting

	// Process work-items defined by the main thread in sequence,
	// terminating when we encounter a work-item with a nil kvc.
	for ; lw.kvc != nil; lw = lw.next {

		if lv.P.Step < ls {
			println("lv at", lv.P.Step, "should be", ls)
		}

		// Collect a threshold number of last-step values in lw.kvc,
		// after which work-item lw will be considered complete.
		// Don't modify kvc or max after reaching the threshold tr,
		// because other code expects these to be immutable afterwards.
		if len(lw.kvc) < c.Tr {

			// Save the actual value read into our local cache
			lw.kvc[node] = lv

			// Track the highest last-step value read on any node,
			// which may be higher than the one we tried to write
			// if we need to catch up with a faster node.
			if lv.P.Step > lw.max.P.Step {
				lw.max = lv
			}
		}

		// Wait until the main thread has created a next work item,
		// and until we have reached the receive threshold.
		for lw.next == nil || len(lw.kvc) < c.Tr {
			lw.cond.Wait()
		}

		// Wake up everyone else once we reach the receive threshold
		lw.cond.Broadcast()

		str := fmt.Sprintf("%v kvc at %v contains:\n", node, ls)
		for i, v := range lw.kvc {
			str += fmt.Sprintf("%v step %v data %q pri %v C step %v data %q \n",
				i, v.P.Step, v.P.Data, v.I, v.C.Step, v.C.Data)
		}
		println(str)

		// Decide on the next step number and value to broadcast,
		// based on the threshold set we collected,
		// which is now immutable and consistent across threads.
		s, v := ls+1, Value{L: lv.L, C: lv.C, P: Head{Step: ls + 1}}
		switch {
		case ctx.Err() != nil:

			// The client Run's context has been cancelled.
			// The worker threads should now do nothing
			// except catch up to the final work-item,
			// releasing other threads waiting on the kvc threshold
			// along the way until all catch up and can terminate.
			println("context cancelled")
			continue

		case lw.max.P.Step > ls:

			// The last work-item failed to reach the threshold
			// because some node had already reached a higher step.
			// Our next work item is simply to catch up all nodes
			// at least to the highest-known step we discovered.
			println(node, "catching up from", ls, "to", lw.max.P.Step)
			s, v = lw.max.P.Step, lw.max

		case (ls & 1) == 0: // completing an even-numbered step

			// Complete the first TLCR broadcast
			// and start the second within a TLCB round.
			// The value for the second broadcsast is simply
			// the (any) threshold receive set from the first.
			v.R = lw.kvc

		case (ls & 3) == 1:

			// Complete the first TLCB call in a QSCOD round
			// and start the second TLCB call for the round.

			// Calculate valid potential (still tentative)
			// R and B sets from the first TLCB call in this round,
			// and include them in the second TLCB broadcast.
			R0, B0 := c.tlcbRB(lw.kvc)

			// Pick any best confirmed proposal from B0
			// as this node's broadcast for the second TLCB round.
			_, v2, _ := B0.best()

			// Set the value for the second TLCB call to broadcast
			v.I, v.R, v.B = v2.I, R0, B0

		case (ls & 3) == 3:

			// Complete a prior QSCOD round and start a new one.

			// First, calculate valid potential R2 and B2 sets from
			// the second TLCB call in the completed QSCOD round.
			R2, B2 := c.tlcbRB(lw.kvc)

			// We always adopt some best confirmed proposal from R2
			// as our own (still tentative so far) view of history.
			// If this round successfully commits,
			// then our b2 will be the same as everyone else's,
			// even if we fail below to realize that fact.
			_, b2, _ := R2.best()

			// Find the best-known proposal b0 in some node's R0.
			// We can get an R0 set from the first round in b2.R.
			// Also determine if b0 was uniquely best in this R0.
			// Our R2 and B2 sets will be subsets of any valid R0.
			n0, b0, u0 := b2.R.best()

			// See if we can determine b2 to have been committed:
			// if b0==b2 is the uniquely-best eligible proposal.
			// This test may succeed only for some nodes in a round.
			// If b is uniquely-best in R0 we can compare priorities
			// to see if two values are the same node's proposal.
//			// Never commit proposals that don't change the Data,
//			// since we use those to represent "no-op" proposals.
			if u0 && b0.I == b2.I && b0.I == B2[n0].I { //&&
//				b0.P.Data != v.C.Data {

				// b0.P is the original proposal with data,
				// which becomes the new current commit C.
				// The previous current commit
				// becomes the last commit L.
				println(node, "committed", b0.P.Step,
					"on", v.C.Step, "data", b0.P.Data)
				v.L, v.C = v.C, b0.P
			}

			// Set the value for the first TLCB call
			// in the next QSCOD round to broadcast,
			// containing a proposal for the next round.
			v.P.Data, v.I = c.Pr(node, v.L, v.C)
		}

		if v.P.Step <= ls || v.P.Step < lw.max.P.Step {
			println("no progress: ls", ls, "lv", lw.max.P.Step,
				"to", v.P.Step)
		}
		if v.P.Step != s {
			println("value has wrong step!?", v.P.Step, s)
		}

		// Try to write new value, then read whatever the winner wrote.
		// But don't write any state changes if our kvc has been
		// contaminated with values from future time-steps.
		c.mut.Unlock()
		v = c.KV[node].WriteRead(v)
		c.mut.Lock()

		if v.P.Step < s {
			println("read back value from wrong step", v.P.Step, s)
		}

		// Note that the newly-returned value v
		// may be from a higher time-step than expected (s).
		// We'll deal with that above in the next iteration.
		// The returned value v can also be Value{}
		// but only after our context ctx has been cancelled.

		// Proceed to the next work item
		ls, lv = s, v
	}

	c.mut.Unlock()
}

// tlcbRB calculates the receive (R) and broadcast (B) sets
// returned by the TLCB algorithm after its second TLCR call.
//
// The returned R and B sets are only tentative,
// representing possible threshold receive-set and broadcast-set outcomes
// from this TLCB invocation, computed locally by this client.
// These locally-computed sets cannot be relied on to be definite for this node
// until the values computed from them are committed via Store.WriteRead.
//
func (c *Client) tlcbRB(kvc Set) (Set, Set) {

	// Using the tentative client-side receive-set from the second TLCR,
	// compute potential receive-set (R) and broadcast-set (B) sets
	// to return from TLCB.
	R, B, Bc := make(Set), make(Set), make([]int, len(c.KV))
	for _, v := range kvc {
		for j, vv := range v.R {
			R[j] = vv          // R has all values we've seen
			Bc[j]++            // How many nodes have seen vv?
			if Bc[j] >= c.Ts { // B has only those reaching ts
				B[j] = vv
			}
		}
	}
	return R, B
}
