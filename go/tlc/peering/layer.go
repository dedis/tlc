package peering/

import (
	"time"
	"math"
	"math/rand"

	"tlc/arch"
)

// Object encapsulating the functionality of this layer
type Layer struct {
	stack arch.Stack
	upper arch.Upper

	peer []peerstate
}

// Per-peer state for this layer
type peerstate struct {

	// For channel-based peers, channel to send message to peer
	sendChan chan<- arch.Message
}

func (l *Layer) Init(stack arch.Stack, upper arch.Upper) {
	l.stack = stack
	l.upper = upper

	c := stack.Config()
	l.peer = make([]peerstate, c.N)

	assert(c.N == len(c.Peer))
	for i := range c.Peer {

		// Launch a goroutine to run the message receive loop
		go l.runPeer(i)
	}
}

func (l *Layer) runPeer(i int) {
	conf := l.stack.Config()
	peer := config.Peer[i]

	// If this peer supports channel-mode operation, use that.
	recv, send := peer.Channel()
	if recv != nil && send != nil {

		// Stash the send channel for sendTo() to use
		l.peer[i].sendChan = send

		// Go into an infinite message receive-and-dispatch loop.
		// XXX provide a clean way to stop an instance gracefully
		for {
			msg := <-recv
			l.recv(msg)
		}
	}

	// Otherwise, use stream-oriented operation.
	rand := rand.New(conf.Rand)
	var backoff Duration
	for {
		// Try to open a connection to this peer.
		before := time.Now()
		err := l.connectPeer(i)

		// If the connection was successful, reset the backoff timer.
		if err == nil {
			backoff = Duration(0)
			continue
		}

		// After the first unsuccessful connection attempt,
		// initialize the backoff duration for the next attempt
		// to the time it took to attempt the first connection.
		if backoff == 0 {
			backoff = time.Now().Sub(before)
			continue
		}

		// Wait for the current backoff time before trying again.
		// The backoff time starts from zero but then increases
		// with successive unsuccessful connection attempts.
		time.Sleep(backoff)

		// Exponentially back off further unsuccessful attempts,
		// up to MaxRetryPeriod in the configuration.
		// Increase the backoff timer by
		// a random factor between 1 and 2.
		mult := 1.0 + rand.Float64()
		backoff = time.Duration(float64(backoff) * mult)
		if conf.MaxBackoff > 0 && newb > conf.MaxBackoff {
			backoff = conf.MaxBackoff
		}
	}
}

func (l *Layer) connectPeer(i int) error {
	conf := l.stack.Config()
	peerconf := conf.Peer[i]

	// Try to connect the stream.
	rw, err := peerconf.Stream()
	if err != nil {
		return err
	}

	l.ProcessStream(i, rw)
	return nil
}

func (l *Layer) ProcessStream(node int, rw io.Stream) {

	for {
		...
	}
}

// Hand a received message up to the next higher layer
func (l *Layer) recv(msg arch.Message) {

	// Protect our main message loop from panics for robustness.
	defer func() {
		if r := recover(); r != nil {
			l.Warnf("panic processing received message: %v", r)
		}
	}()

	// Hand the received message up the stack,
	// using appropriate synchronization since we're on our own goroutine.
	dispatch := func() {
		l.upper.Recv(msg)
	}
	l.stack.Call(dispatch)
}

// The upper layer calls this method to broadcast a message to all peers.
func (l *Layer) Send(msg arch.Message) {

	XXX broadcast or unicast
}

func (l *Layer) sendTo(msg arch.Message, i int) {
	peer := l.peer[i]

	// If the peer has a send channel, just use that.
	if peer.sendChan != nil {
		peer.sendChan <- msg
		return
	}

	XXX
}

