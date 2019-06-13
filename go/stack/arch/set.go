package arch


// MessageSet uses a Go map to represent a set of messages.
type MessageSet map[*Message] struct{}

// Test if msg is in set s.
func (s MessageSet) Has(msg *Message) bool {
	_, present := s[msg]
	return present
}

// Add msg to set s and return s.
func (s MessageSet) Add(msg *Message) MessageSet {
	s[msg] = struct{}{}
	return s
}

// Return a copy of message set s,
// dropping any messages before specified earliest time-step.
func (s MessageSet) Prune(earliest Step) MessageSet {
	n := make(MessageSet)
	for k, v := range s {
		if k.Step >= earliest {
			n[k] = v
		}
	}
	return n
}

// Return an identical copy of message set s.
func (s MessageSet) copy() MessageSet {
	return s.Prune(0)
}

