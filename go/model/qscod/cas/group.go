package cas

import (
	"context"
	"sync"

	"github.com/dedis/tlc/go/lib/cas"
	"github.com/dedis/tlc/go/model/qscod/core"
)

// group implements the cas.Store interface as a QSCOD consensus group.
type group struct {
	c   core.Client     // consensus client core
	ctx context.Context // group operation context

	mut  sync.Mutex     // for synchronizing shutdown
	wg   sync.WaitGroup // counts active CAS operations
	done bool           // set after group shutdown

	// channel that CAS calls use to propose work to do
	ch chan func(i core.Node, L, C core.Head) (string, int64)
}

// Group creates a cas.Store implementation representing
// a consensus group comprised of particular members,
//
// Consensus thresholds are determined by the faulty parameter,
// the maximum number of faulty nodes the group should tolerate.
// For this implementation of QSCOD based on the TLCB and TLCR algorithms,
// faulty should be at most one-third of the total group size.
//
//
func Group(ctx context.Context, members []cas.Store, faulty int) cas.Store {

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
	println("N", N, "Tr", Tr, "Ts", Ts)

	// Create a consensus group state instance
	g := &group{c: core.Client{Tr: Tr, Ts: Ts}, ctx: ctx}
	g.ch = make(chan func(i core.Node, L, C core.Head) (string, int64))

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
	g.c.Pr = func(i core.Node, L, C core.Head) (prop string, pri int64) {
		for {
			select {
			case f := <-g.ch: // got a CAS work function to call
				if f == nil { // context cancelled
					println("Pr: channel closed")
					return C.Data, 0 // no-op proposal
				}
				prop, pri = f(i, L, C) // call work function
				if prop != "" {
					return prop, pri // return its result
				}

			case <-ctx.Done(): // our context got cancelled
				println("Pr: cancelled")
				return C.Data, 0 // produce no-op proposal
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
func (g *group) run(ctx context.Context) {

	// Run the consensus protocol until our context gets cancelled
	g.c.Run(ctx)

	// Drain any remaining proposal function sends to the group's channel.
	// CheckAndSet won't add anymore after g.ctx has been cancelled.
	go func() {
		for range g.ch {
		}
	}()

	g.mut.Lock()

	// Wait until no threads are in active CheckAndSet calls.
	g.wg.Done()
	g.wg.Wait()

	// Now it's safe to close the group's channel.
	println("closing channel")
	close(g.ch)
	g.done = true

	g.mut.Unlock()
}

// CheckAndSet conditionally writes a new version and reads the latest,
// implementing the cas.Store interface.
//
func (g *group) CheckAndSet(ctx context.Context, lastVer int64, reqVal string) (
	actualVer int64, actualVal string, err error) {

	//println("CAS lastVer", lastVer, "reqVal", reqVal)

	// Record active CheckAndSet calls in a WaitGroup
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
	pr := func(i core.Node, L, C core.Head) (prop string, pri int64) {
		mut.Lock()
		defer mut.Unlock()

		println("CAS prop lastVer", lastVer, "reqVal", reqVal, "C.Step", C.Step, "C.Data", C.Data)

		// Now check the situation of what's known to be committed.
		switch {

		// If we haven't yet committed anything atop lastVer,
		// it's safe to propose reqVal as the new data to commit.
		case int64(C.Step) == lastVer:
			prop, pri = reqVal, randValue()

		// Our CAS has succeeded if we've committed a new version
		// that builds immediately on the version we were expecting
		// and that commits the reqVal we were trying to propose.
		// Return "" in prop to have this worker thread keep waiting
		// for another CAS operation to propose something useful.
		case int64(L.Step) == lastVer && C.Data == reqVal:
			println("proposal committed at step", C.Step)
			if int64(C.Step) <= lastVer {
				panic("XXX")
			}
			actualVer = int64(C.Step)
			actualVal = reqVal

		// Otherwise, our CAS fails with a Changed error as soon as
		// anything else gets committed on top of lastVer.
		// Return "" in prop to keep this worker thread waiting.
		case int64(C.Step) > lastVer:
			println("proposal overridden at step", C.Step)
			actualVer = int64(C.Step)
			actualVal = C.Data
			err = cas.Changed

		// If C.Step < lastVer, we're choosing a proposal for a node
		// that doesn't yet "know" that lastVer was committed.
		// Just return a "safe" no-op proposal for this node,
		// although we know it has no chance of being committed.
		case int64(C.Step) < lastVer:
			println(i, "outdated at", C.Step, "<", lastVer,
				"data", C.Data)
			prop, pri = C.Data, 0

		default:
			panic("XXX")
		}
		return
	}

	// A simple helper function to test if we've completed our work.
	done := func() bool {
		mut.Lock()
		defer mut.Unlock()
		return actualVer > 0 || err != nil
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
	return
}
