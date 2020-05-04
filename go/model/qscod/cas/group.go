package cas

import (
	"context"
	"sync"

	"github.com/dedis/tlc/go/lib/cas"
	"github.com/dedis/tlc/go/model/qscod/core"
)

// Group implements the cas.Store interface as a QSCOD consensus group.
// After creation, invoke Start to configure the consensus group state,
// then call CompareAndSet to perform CAS operations on the logical state.
type Group struct {
	c   core.Client     // consensus client core
	ctx context.Context // group operation context

	mut  sync.Mutex     // for synchronizing shutdown
	wg   sync.WaitGroup // counts active CAS operations
	done bool           // set after group shutdown

	// channel that CAS calls use to propose work to do
	ch chan func(core.Step, string, bool) (string, int64)
}

// Start initializes g to represent a consensus group comprised of
// particular member nodes, starts it operating, and returns g.
//
// Consensus thresholds are determined by the faulty parameter,
// the maximum number of faulty nodes the group should tolerate.
// For this implementation of QSCOD based on the TLCB and TLCR algorithms,
// faulty should be at most one-third of the total group size.
//
// Start launchers worker goroutines that help service CAS requests,
// which will run and consume resources forever unless cancelled.
// To define their lifetime, the caller should pass a cancelable context,
// and cancel it when operations on the Group are no longer required.
//
func (g *Group) Start(ctx context.Context, members []cas.Store, faulty int) *Group {

	// Calculate and sanity-check the threshold configuration parameters.
	// For details on where these calculations come from, see:
	// https://arxiv.org/abs/2003.02291
	N := len(members)
	Tr := N - faulty // receive threshold
	Ts := N - Tr + 1 // spread threshold
	if Tr <= 0 || Tr > N || Ts <= 0 || Ts > Tr || (Ts+Tr) <= N {
		panic("faulty threshold yields unsafe configuration")
	}
	if N*(Tr-Ts+1)-Tr*(N-Tr) <= 0 { // test if Tb <= 0
		panic("faulty threshold yields non-live configuration")
	}
	//println("N", N, "Tr", Tr, "Ts", Ts)

	// Create a consensus group state instance
	g.c = core.Client{Tr: Tr, Ts: Ts}
	g.ctx = ctx
	g.ch = make(chan func(s core.Step, p string, c bool) (string, int64))

	// Create a core.Store wrapper around each cas.Store group member
	g.c.KV = make([]core.Store, N)
	for i := range members {
		g.c.KV[i] = &coreStore{Store: members[i], g: g}
	}

	// Our proposal function normally just "punts" by waiting for
	// an actual proposal to get sent on the group's channel,
	// and then we call that to form the proposal as appropriate.
	// But we concurrently listen for channel cancellation
	// and return promptly with a no-op proposal in that case.
	g.c.Pr = func(s core.Step, p string, c bool) (prop string, pri int64) {
		for {
			select {
			case f := <-g.ch: // got a CAS work function to call
				if f == nil { // context cancelled
					println("Pr: channel closed")
					return p, 0 // no-op proposal
				}
				//println("got work function\n")
				prop, pri = f(s, p, c) // call work function
				if prop != "" {
					return prop, pri // return its result
				}
				//println("work function yielded no work")

			case <-ctx.Done(): // our context got cancelled
				//println("Pr: cancelled")
				return p, 0 // produce no-op proposal
			}
		}
	}

	// Launch the underlying consensus core as a separate goroutine.
	// Make sure the group's WaitGroup remains nonzero until
	// the context is cancelled and we're ready to shut down.
	g.wg.Add(1)
	go g.run(ctx)

	return g
}

// Run consensus in a goroutine
func (g *Group) run(ctx context.Context) {

	// Run the consensus protocol until our context gets cancelled
	g.c.Run(ctx)

	// Drain any remaining proposal function sends to the group's channel.
	// CompareAndSet won't add anymore after g.ctx has been cancelled.
	go func() {
		for range g.ch {
		}
	}()

	g.mut.Lock()

	// Wait until no threads are in active CompareAndSet calls.
	g.wg.Done()
	g.wg.Wait()

	// Now it's safe to close the group's channel.
	close(g.ch)
	g.done = true

	g.mut.Unlock()
}

// CompareAndSet conditionally writes a new version and reads the latest,
// implementing the cas.Store interface.
//
func (g *Group) CompareAndSet(ctx context.Context, old, new string) (
	version int64, actual string, err error) {

	//println("CAS lastVer", lastVer, "reqVal", reqVal)

	// Record active CompareAndSet calls in a WaitGroup
	// so that the group's main goroutine can wait for them to complete
	// when shutting down gracefully in response to context cancellation.
	// Atomically check that the group is still active before wg.Add.
	g.mut.Lock()
	if g.done {
		//println("CAS after done")
		// This should only ever happen once the context is cancelled
		if g.ctx.Err() == nil {
			panic("group done but context not cancelled?")
		}
		g.mut.Unlock()
		return 0, "", g.ctx.Err()
	}
	g.wg.Add(1)
	g.mut.Unlock()
	defer g.wg.Done()

	// We'll need a mutex to protect concurrent accesses to our locals.
	mut := sync.Mutex{}

	// Define the proposal formulation function that will do our work.
	// Returns the empty string to keep this worker thread waiting
	// for something to propose while letting other threads progress.
	pr := func(s core.Step, cur string, com bool) (prop string, pri int64) {
		mut.Lock()
		defer mut.Unlock()

		//		println("CAS prop lastVer", lastVer, "reqVal", reqVal, "C.Step", C.Step, "C.Data", C.Data)

		// Now check the situation of what's known to be committed.
		switch {

		// It's safe to propose new as the new string to commit
		// only if the prior value we're building on is equal to old.
		case cur == old:
			prop, pri = new, randValue()

		// Complete the CAS operation as soon as we commit anything,
		// whether it was our new proposal or some other string.
		case com:
			version, actual = int64(s), cur

		// Otherwise, if the current proposal isn't the same as old
		// but also isn't committed, we have to make no-op proposals
		// until we manage to get something committed.
		default:
			prop, pri = cur, randValue()

			//case int64(s) > lastVer && c && p != prop:
			//	err = cas.Changed
			//	fallthrough
			//// XXX get rid of Changed?

			//case int64(s) > lastVer && c:
			//	actualVer = int64(s)
			//	actualVal = reqVal

			//case int64(s) > lastVer:
			//	// do nothing

			// Our CAS has succeeded if we've committed a new version
			// that builds immediately on the version we were expecting
			// and that commits the reqVal we were trying to propose.
			// Return "" in prop to have this worker thread keep waiting
			// for a future CAS operation to propose something useful.
			//		case int64(L.Step) == lastVer && C.Data == reqVal:
			//			println("proposal committed at step", C.Step)
			//			if int64(C.Step) <= lastVer {
			//				panic("XXX")
			//			}
			//			actualVer = int64(s)
			//			actualVal = reqVal

			// Otherwise, our CAS fails with a Changed error as soon as
			// anything else gets committed on top of lastVer.
			// Return "" in prop to keep this worker thread waiting.
			//		case int64(C.Step) > lastVer:
			//			println("proposal overridden at step", C.Step)
			//			actualVer = int64(C.Step)
			//			actualVal = C.Data
			//			err = cas.Changed

			// If C.Step < lastVer, we're choosing a proposal for a node
			// that doesn't yet "know" that lastVer was committed.
			// Just return a "safe" no-op proposal for this node,
			// although we know it has no chance of being committed.
			//		case int64(C.Step) < lastVer:
			//			println(i, "outdated at", C.Step, "<", lastVer,
			//				"data", C.Data)
			//			prop, pri = C.Data, 0

			//default:
			//	panic("lastVer appears to be from the future")
		}
		return
	}

	// A simple helper function to test if we've completed our work.
	done := func() bool {
		mut.Lock()
		defer mut.Unlock()
		return actual != "" || err != nil
	}

	// Continuously send references to our proposal function
	// to the group's channel so it will get called until it finishes
	// or until one of the contexts (ours or the group's) is cancelled.
	// Since the channel is unbuffered, each send will block
	// until some consensus worker thread is ready to receive it.
	for !done() && ctx.Err() == nil && g.ctx.Err() == nil {
		//println("CAS sending lastVer", lastVer, "reqVal", reqVal)
		g.ch <- pr
	}
	//	println("CAS done", lastVer, "reqVal", reqVal,
	//		"actualVer", actualVer, "actualVal", actualVal, "err", err)
	return version, actual, err
}
