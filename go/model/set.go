package model

// Use a map to represent a set of messages
type set map[*Message]struct{}

// Test if msg is in set s.
func (s set) has(msg *Message) bool {
	_, present := s[msg]
	return present
}

// Add msg to set s.
func (s set) add(msg *Message) {
	s[msg] = struct{}{}
}

