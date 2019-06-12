package byzantine

import (
	"sync"
	"log"
	"time"

	"tlc"
	"tlc/consensus"
	// "tlc/keysensus"
	"tlc/randomness"
	"tlc/timerelease"
	"tlc/threstime"
	"tlc/witness"
	"tlc/gossip"
	"tlc/vectime"
	"tlc/logging"
	"tlc/realtime"
	"tlc/peering"
)


// Interface to some base messaging layer that the whole stack builds upon.
type Base interface {

	Send(msg Message)
}

// Interface to the application layer building on top of the stack.
type Application interface {

	// Ask the application for the next contribution it wishes to commit.
	Contribute(step arch.Step) arch.Message

	// Inform the application of a proposal that has been committed.
	// This is called with monotonically increasing time-steps,
	// but may skip time-steps in the case of lag.
	Committed(step arch.Step, msg arch.Message)

	// Inform the application of a proposal that might or might not commit.
	// This information is provided in case the application
	// wishes to speculate or pre-compute based on possibilities,
	// but it can also simply be ignored.
	// Potential(step arch.Step, msg arch.Message)
}

type Stack struct {
	mut sync.Mutex		// mutex to single-thread the entire TLC stack

	cons consensus.Layer
	rand randomness.Layer
	rel timerelease.Layer
	thres threstime.Layer
	wit witness.Layer
	gos gossip.Layer
	vec vectime.Layer
	real realtime.Layer
	peer peering.Layer
}

func (s *Stack) Init(app Application) {

	// Initialize the layers progressively from bottom to top.
	s.peer.Init(s, &s.real)
	s.real.Init(s, &s.peer, &s.vec)
	s.vec.Init(s, &s.real, &s.log)
	s.gos.Init(s, &s.vec, &s.wit)
	s.wit.Init(s, &s.gos, &s.thres)
	s.thres.Init(s, &s.wit, &s.rel)
	s.rel.Init(s, &s.thres, &s.rand)
	s.rand.Init(s, &s.rel, &s.cons)
	s.cons.Init(s, &s.rand, app)
}

func (s *Stack) Now() time.Time {
	return time.Now()
}

func (s *Stack) At(t time.Time, f func()) arch.Timer {

	// Wrap the callback to preserve TLC stack single-threading,
	// since Go timers call the callback in another goroutine.
	func fire() {
		s.Call(f)
	}

	return time.AfterFunc(Until(t), fire)
}

func (s *Stack) Warnf(format string, v ...interface{}) {
	log.Printf(string, v...)
}

func (s *Stack) Call(f func()) {
	s.mut.Lock()
	defer func() {
		s.mut.Unlock()
	}
	f()
}

func (s *Stack) Run() {
	s.mut.Lock()
	defer func() {
		s.mut.Unlock()
	}
	for {
		XXX
	}
}

