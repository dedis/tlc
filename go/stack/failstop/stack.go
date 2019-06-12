package failstop

import (
	"tlc"
	"tlc/consensus"
	"tlc/witness"
	"tlc/gossip"
	"tlc/vectime"
	"tlc/log"
)


// Generic lower-layer interface
// type lowerLayer interface {
// 	Send(msg tlc.Message)
// }

// Generic upper-layer interface
// type upperLayer interface {
// 	Recv(msg tlc.Message)
// }


type Stack struct {

	vec
	real realtimeLayer
	base Base
}

func (s *Stack) Init(base tlc.Lower) {

	// Initialize the layers progressively from bottom to top.
	s.base = base
	s.real.Init(s, base, &s.vec)
	s.vec.Init(s, &s.real, &s.log)
	...
}

