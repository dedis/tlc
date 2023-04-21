package quepaxa

import (
	"context"
	"sync"
	//	"github.com/dedis/tlc/go/model/quepaxa/isr"
)

type Node int32
type Choice int64
type Step int32

//const Decided = Step(math.MaxInt32)

// A logical time consists of
// a Choice (consensus decision or slot number) and
// a Step (consensus attempt number within a turn).
type Time struct {
	c Choice
	s Step
}

// Returns true if logical time T1 is strictly less than T2.
func (t1 Time) LT(t2 Time) bool {
	return t1.c < t2.c || (t1.c == t2.c && t1.s < t2.s)
}

//type integer interface {
//	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64
//}

// The Proposal interface defines constraints for a concrete proposal type P.
//
// The Rank method must return the same proposal with rank set appropriately:
// - to the maximum rank High if leader is set (this replica is the leader)
// - to a freshly-chosen random rank between 1 and High-1 otherwise
//
// In addition, if proposal ranks are low-entropy so there is a chance of ties,
// and P is using replica numbers for tiebreaking,
// then the Rank function also sets the replica number in the proposal.
type Proposal[P any] interface {
	Nil() P                           // the nil proposal
	Best(other P) P                   // best of this and other
	Rank(replica Node, leader bool) P // randomly rank proposal
	EqD(other P) bool                 // equality for deciding
}

//type Proposal[D any] struct {
//	R Rank			// random rank to prioritize this proposal
//	I Node			// identity of initial proposing replica
//	D Data			// decision data associated with proposal
//}

//func (x Proposal[D]).Cmp(y Proposal[D]) bool {
//	switch {
//	case x.R < y.R || (x.R == y.R && x.I < y.I):
//		return -1
//	case x.R > y.R || (x.R == y.R && x.I > y.I):
//		return 1
//	default:
//		return 0
//	}
//}

type Replica[P Proposal[P]] interface {
	Record(ctx context.Context, t Time, p P) (
		rt Time, rf P, rl P, err error)
}

type Proposer[P Proposal[P]] struct {

	// configuration state
	w  []worker[P] // one worker per replica
	th int         // consensus threshold (n-f)

	// synchronization state
	m sync.Mutex
	c sync.Cond

	t Time // proposer's current logical time

	// per-choice state
	ld Node // which replica is the leader, -1 if none
	dp P    // decision proposal from last choice
	nf int  // number of fast-path responses this choice

	// per-step state
	pp P   // preferred proposal for this step
	bp P   // best of appropriate replies this step
	nr int // number of responses seen so far this step

	// graceful termination state
	stop   bool               // signal when workers should shut down
	ctx    context.Context    // cancelable context for all of our workers
	cancel context.CancelFunc // cancellation function
}

func (p *Proposer[P]) Init(replicas []Replica[P]) {

	if p.w != nil {
		panic("Proposer.Init must not be invoked twice")
	}

	// set up a cancelable context for when we want to stop
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// set the threshold appropriately for group size
	p.th = len(replicas)/2 + 1

	p.w = make([]worker[P], len(replicas))
	for i := range replicas {
		p.w[i].p = p
		p.w[i].r = replicas[i]
		p.w[i].i = Node(i)

		go p.w[i].work()
	}
}

func (p *Proposer[P]) Agree(preferred P) (choice Choice, decision P) {

	// keep our mutex locked except while waiting on a condition
	p.m.Lock()
	defer p.m.Unlock()

	c := p.t.c
	if p.t.s < 4 {
		p.advance(Time{p.t.c, 4}, preferred)
	}
	for !p.stop && p.t.c == c {
		// signal any non-busy workers that there's new work to do
		p.c.Broadcast()
	}

	// return choice at which last decision was made, and that decision
	return p.t.c - 1, p.dp
}

// Advance to time t with preferred proposap pp.
// Proposer's mutex must be locked.
func (p *Proposer[P]) advance(t Time, pp P) {
	p.t = t         // new time step
	p.pp = pp       // preferred proposal entering new step
	p.bp = pp.Nil() // initial best proposal from new step
	p.nr = 0        // count responses toward threshold

	if t.s == 4 { // only when advancing to fast-path step...
		p.nf = 0 // initialize fast-path response count
	}
}

// Each worker thread calls workDone when it gets a response from a recorder.
//
// This function gets called at most once per recorder per time step,
// so it can count responses without worrying about duplicates.
func (p *Proposer[P]) workDone(rt Time, rf, rl P) {

	// When we receive fast-path responses from phase 4 of current choice,
	// count them towards the fast-path threshold even if they come late.
	if rt.c == p.t.c && rt.s == 4 {
		p.nf++
		if p.nf == p.th {
			p.decided(rf) // fast-path decision
		}
	}

	// where is the proposer with respect to the response in logical time?
	if rt.LT(p.t) { // is the response behind the proposer?
		return // the work done is obsolete - just discard
	}
	if p.t.LT(rt) { // is the response ahead of the proposer?
		p.advance(rt, rf) // advance to newer time in response
		return
	}
	// the response is from proposer's current time step exactly

	// what we do with the response depends on which phase we're in
	if rt.s&3 == 0 {
		p.bp = p.bp.Best(rf) // Phase 0: best of first proposals
	} else if rt.s&2 != 0 {
		p.bp = p.bp.Best(rl) // Phase 2-3: best of last aggregate
	}

	// have we reached the response threshold for this step?
	p.nr++
	if p.nr < p.th {
		return // not yet, wait for more responses
	}
	// threshold reached, so we can complete this time step

	// in phase 2, check if we've reached a consensus decision
	if rt.s&3 == 2 && p.pp.EqD(p.bp) {
		p.decided(p.pp)
		return
	}
	// no decision yet but still end of current time step

	// in phases 0 and 3, new preferred proposal is best from replies
	pp := p.pp
	if rt.s&3 == 0 || rt.s&3 == 3 {
		pp = p.bp
	}

	// advance to next logical time step
	p.advance(Time{p.t.c, p.t.s + 1}, pp)
}

func (p *Proposer[P]) decided(dp P) {

	// record the decision in local state
	p.t.c++   // last choice is decided, now on to next
	p.t.s = 0 // idle but ready for a new agreement
	p.dp = dp // record decision proposal from last choice
	p.ld = -1 // default to no leader, but caller can change

	// signal the main proposer thread to return the decision,
	// while the workers inform the recorders asynchronously.
	p.c.Broadcast()
}

// Immediately after observing a decision being made,
// the application can select a new leader based on that decision.
// If SetLeader is not called, the next choice is leaderless.
// The choice of leader (or lack thereof) must be deterministic
// based on prior decisions and set the same on all nodes.
func (p *Proposer[P]) SetLeader(leader Node) {
	p.ld = leader
}

// Stop permanently shuts down this proposer and its worker threads.
func (p *Proposer[P]) Stop() {

	p.stop = true   // signal that workers should stop
	p.c.Broadcast() // wake them up to see the signal
	p.cancel()      // also cancel all Record calls in progress
}

// We create one worker per replica.
type worker[P Proposal[P]] struct {
	p *Proposer[P] // back pointer to Proposer
	r Replica[P]   // Replica interface of this replica
	i Node         // replica number of this replica
}

func (w *worker[P]) work() {
	p := w.p // keep handy pointer back to proposer
	p.m.Lock()
	for !p.stop {
		// we're done with prior steps so wait until proposer advances
		t := p.t // save proposer's current time
		for p.t == t {
			p.c.Wait()
		}

		pp := p.pp      // save proposer's preferred proposal
		if t.s&3 == 0 { // in phase zero we must re-rank proposals
			pp = pp.Rank(w.i, t.s == 4 && w.i == p.ld)
		}

		// asychronously record the proposal with mutex unlocked
		p.m.Unlock()
		rt, rf, rl, err := w.r.Record(p.ctx, t, pp)
		if err != nil { // canceled
			return
			// XXX backoff retry?
		}
		p.m.Lock()

		// inform the Proposer that this recorder's work is done
		p.workDone(rt, rf, rl)
	}
	p.m.Unlock()
}
