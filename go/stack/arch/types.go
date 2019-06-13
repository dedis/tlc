// Shared definitions common to the entire TLC protocol stack architecture.
// This package effectively defines the "common language"
// that all the layers must speak in order to coordinate.
package arch

import (
	"time"
	"io"
	"encoding/binary"
	"crypto/rand"
)

// Common cross-layer interface to abstract Message objects.
// XXX move this and message-specific types to 'tlc/protocol' package?
type MessageInterface interface {

	// Return the message's uniquely-identifying label,
	// consisting of originating and destination node number,
	// time-step number, and node-local sequence number.
	// The destination is -1 for undirected broadcast messages.
	GetLabel() (from, dest int, step Step, seq Seq)

	// Set the message's label.
	SetLabel(from, dest int, step Step, seq Seq)

	// Obtain the wall-clock timestamp the message contains, if any.
	// Returns a zero time if the message contains no wall-clock timestamp.
	GetTime() time.Time

	// Set the message's wall-clock timestamp to a particular time.
	SetTime(wallclock time.Time)

	// Get the message's vector timestamp.
	GetVec(vector Vec)

	// Set the message's vector timestamp.
	SetVec(vector Vec)

	//BroadcastMessage() *BroadcastMessage
	//ViewReqMessage() *ViewReqMessage
	//ViewstampMessage() *ViewstampMessage
}

// A BroadcastMessage represents a node's main TLC broadcast each time-step.
// It may optionally contain application-provided payload.
type BroadcastMessage struct {
	Payload []byte
}

type LogEventMessage struct {
	Sig []byte		// Signature on this log head
}

// A node sends a ViewReqMessage to its peer to request the peer's Viewstamp,
// or list of log-head hashes, corresponding to a given vector timestamp.
// Peers need to do this to figure out who is to blame for any equivocation.
type ViewReqMessage struct {
	View Vec
}

// A ViewstampMessage is a response to a ViewReqMessage from a peer.
type ViewstampMessage struct {
	View Vec
	Viewstamp [][]byte	// One log entry hash per participant.
				// Entries for participants already caught
				// equivocating may be nil.
}

// Common systems functionality that a whole TLC protocol stack needs
//type Stack interface {

	// Return this stack's configuration,
	// which should never change after the stack's creation.
//	Config() *Config

	// Return the basic dimension parameters of this TLC configuration
//	Dim() (Tm, Tw, N int)

	// Obtain the current time (which may be real or simulated)
//	Now() time.Time

	// Request callback to function f as soon as possible after time t
//	At(t time.Time, f func()) Timer

	// Log a non-fatal warning
//	Warnf(format string, v ...interface{})

	// Safely call into the TLC stack's single-threaded environment
	// from an independent goroutine running asynchronously from the stack.
//	Call(f func())
//}

type Config struct {
	N int			// Total number of participating nodes
	Tm int			// Message threshold
	Tw int			// Witnessing threshold

	// How to connect to each peer: e.g., TLS with PKCS keys, etc.
	Peer []Peer		// Array of peer connection configurations

	// Max time to wait to retry failed connections,
	// or 0 to set no limit on exponential backoff.
	MaxBackoff time.Duration

	// hash algorithm, signing algorithm, etc.

	// Source of entropy for all private randomness needed in the TLC stack.
	Rand Entropy
}


// Entropy represents a source of randomness for a TLC stack instance.
// Since TLC stacks are single-threaded,
// an Entropy source likewise needs to support only single-threaded access.
// an Entropy object satisfies both the io.Reader raw byte interface
// and math/rand.Source interfaces for convenience.
type Entropy struct {
	Stream io.Reader
}

func (e Entropy) Read(b []byte) (n int, err error) {
	if e.Stream != nil {
		n, err = e.Stream.Read(b)
	} else {
		n, err = rand.Reader.Read(b)
	}
	return
}

func (e Entropy) Int63() int64 {
	var buf [8]byte
	n, err := io.ReadFull(e, buf[:])
	if n != 8 || err != nil {
		panic("error reading random entropy source: " + err.Error())
	}
	buf[0] &= 0x7f	// clear bit 63 (the sign bit)
	return int64(binary.BigEndian.Uint64(buf[:]))
}

func (e Entropy) Seed(seed int64) {
	panic("cannot re-seed cryptographic random entropy source")
}


type Peer interface {

	// Attempt to open a message-oriented channel-based connection.
	// This approach gives the Peer implementation maximum control
	// over message encoding, framing, channel management, etc.
	// If successful, the TLC stack assumes that the provided channels
	// are good and usable for the lifetime of this TLC stack instance;
	// there is no error recovery and reconnection mechanism.
	// Returns nil, nil if message channels not supported.
	Channel() (recv <-chan Message, send chan<- Message)

	// Attempt to open a stream-oriented connection.
	// Returns a read/write stream if the connection attempt succeeded,
	// or nil and an error if the connection attempt failed.
	// On connection failure, the peering module
	// will attempt to reconnect automatically with exponential backoff
	// to limit resource consumption during persistent unreachability.
	//Stream() (rw io.Stream, err error)

	// ...
}

// Generic interface to a lower layer as seen by the immediately higher layer.
//type Lower interface {

	// Downcall the lower layer to broadcast a message to all participants.
//	Send(msg Message)

	// Signal all lower layers that all information regarding timesteps
	// before min are now obsolete and may be garbage-collected.
//	Obsolete(min Step)
//}

// Generic interface to a higher layer as seen by the immediately lower layer.
//type Upper interface {

	// Upcall the upper layer to process a received message.
//	Recv(msg Message)
//}

// Basic interface to the minimal Timer functionality the TLC stack needs.
// This is compatible with, but simpler than, the time.Timer interface
// to make it easy to substitute real time with simulated time if needed.
type Timer interface {
	Stop() bool
}

