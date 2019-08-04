// This package implements a simple pedagogic model of TLC and QSC.
// It uses no cryptography and supports only failstop, non-Byzantine consensus,
// but should be usable in scenarios that would typically employ Paxos or Raft.
//
// This implementation is less than 200 lines of actual code as counted by CLOC,
// so a good way to understand it is to read the code directly at
// https://github.com/dedis/tlc/tree/master/go/model.
// You can test this implementation in a variety of consensus configurations
// using only goroutines and channels for communication via:
//
//	go test -v
//
// To read about the principles underlying TLC and QSC, please refer to
// https://arxiv.org/abs/1907.07010.
// For a high-level overview of the different implementations of TLC/QSC
// in different languages that live in this repository, please see
// https://github.com/dedis/tlc/.
//
// Configuring and launching consensus groups
//
// To use this implementation of QSC,
// a user of this package must first configure and launch
// a threshold group of nodes.
// This package handles only the core consensus logic,
// leaving matters such as node configuration, network names, connections,
// and wire-format marshaling and unmarshaling to the client of this package.
//
// The client using this package
// must assign each node a unique number from 0 through nnode-1,
// e.g., by configuring the group with a well-known ordering of its members.
// Only node numbers are important to this package; it is oblivious to names.
//
// When each node in the consensus group starts,
// the client calls NewNode to initialize the node's TLC and QSC state.
// The client may then change optional Node configuration parameters,
// such as Node.Rand, before actually commencing protocol message processing.
// The client then calls Node.Advance to launch TLC and the consensus protocol,
// advance to time-step zero, and broadcast a proposal for this time-step.
// Thereafter, the protocol self-clocks asynchronously using TLC
// based on network communication.
//
// Consensus protocol operation and results
//
// This package implements QSC in pipelined fashion, which means that
// a sliding window of three concurrent QSC rounds is active at any time.
// At the start of any given time step s when Advance broadcasts a Raw message,
// this event initiates a new consensus round starting at s and ending at s+3,
// and (in the steady state) completes a consensus round that started at s-3.
// Each Message a node broadcasts includes QSC state from four rounds:
// Message.QSC[0] holds the results of the consensus round just completed,
// while QSC[1] through QSC[3] hold the state of the three still-active rounds,
// with QSC[3] being the newest round just launched.
//
// If Message.QSC[0].Commit is true in the Raw message commencing a time-step,
// then this node saw the round ending at step Message.Step as fully committed.
// In this case, all nodes will have agreed on the same proposal in that round,
// which is the proposal made by node number Message.QSC[0].Conf.From.
// If the client was waiting for a particular transaction to be ordered
// or definitely committed/aborted according to the client's transaction rules,
// then seeing that Message.QSC[0].Commit is true means that the client may
// resolve the status of transactions proposed up to Message.Step-3.
// Other nodes might not have observed this same round as committed, however,
// so the client must not assume that other nodes also necessarily be aware
// that this consensus round successfully committed.
//
// If Message.QSC[0].Commit is false, the round may or may not have converged:
// this node simply cannot determine conclusively whether the round converged.
// Other nodes might have chosen different "best confirmed" proposals,
// as indicated in their respective QSC[0].Conf.From broadcasts for this step.
// Alternatively, the round may in fact have converged,
// and other nodes might observe that fact, even though this node did not.
//
// Message transmission, marshaling
//
// This package invokes the send function provided to NewNode to send messages,
// leaving any wire-format marshaling required to the provided function.
// This allows the client complete control over the desired wire format,
// and to include other information beyond the fields defined in Message,
// such as any semantic content on which the client wishes to achieve consensus.
// On receipt of a message from another node,
// the client must unmarshal it as appropriate
// and invoke Node.Receive with the unmarshalled Message.
//
// Concurrency control
//
// The consensus protocol logic in this package is not thread safe:
// it must be run in a single goroutine,
// or else the client must implement appropriate locking.
//
package model
