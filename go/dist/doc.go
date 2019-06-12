// This package implements a minimalistic distributed implementation
// of TLC and QSC for the non-Byzantine (fail-stop) threat model.
// It uses TLS/TCP for communication, gob encoding for serialization, and
// vector time and a basic gossip protocol for causal message propagation.
package minnet


