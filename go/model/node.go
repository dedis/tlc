package model

var Threshold int // TLC and consensus threshold
var All []*Node   // List of all nodes

var MaxSteps int          // Max number of consensus rounds to run
var MaxTicket int32 = 100 // Amount of entropy in lottery tickets

type Type int // Type of message
const (
	Raw Type = iota // Raw unwitnessed proposal
	Ack             // Acknowledgment of a proposal
	Wit             // Threshold witness confirmation of proposal
)

type Message struct {
	From int     // Which node sent this message
	Step int     // Logical time step this message is for
	Type Type    // Message type: Prop, Ack, or Wit
	Tkt  int     // Genetic fitness ticket for consensus
	QSC  []Round // qsc[s] is consensus state for round ending at step s
}

type Node struct {
	Message               // Template for messages we send
	comm    chan *Message // Channel to send messages to this node
	acks    int           // # acknowledgments we've received in this step
	wits    int           // # threshold witnessed messages seen this step
}

func newNode(self int) (n *Node) {
	return &Node{
		Message: Message{From: self,
			QSC: make([]Round, 3)}, // "rounds" ending in steps 0-2
		comm: make(chan *Message, 3*len(All)*MaxSteps)}
}
