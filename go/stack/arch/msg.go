package arch

// Node represents the node number of a participant in a coordination group.
type Node int

// Step represents a 64-bit logical time-step number produced by TLC.
type Step uint64

type Type int			// Type of message
const (
	Prop Type = iota	// Raw unwitnessed proposal
	Ack			// Acknowledgment of a proposal
	Wit			// Threshold witness confirmation of proposal
)

// Returns true if t is a broadcast message type, false if unicast.
func (t Type) Broadcast() bool {
	switch t {
	case Prop, Wit:
		return true
	case Ack:
		return false
	default:
		panic("undefined message type")
	}
}

// Genetic fitness lottery ticket (XXX should be []byte).
type Ticket int32

// Message represents a unicast or broadcast message within the TLC stack.
type Message struct {
	// Network/peering layer
	From	Node		// Which node originally sent this message

	// Causality layer
	Seq	Seq		// Node-local sequence number for vector time
	Vec	Vec		// Vector clock update from sender node

	// Threshold time (TLC) layer
	Step	Step		// Logical time step this message is for
	Type	Type		// Message type
	Prop	Node		// Proposal Seq this Ack or Wit is about
	Ticket	Ticket		// Genetic fitness ticket for this proposal
}

