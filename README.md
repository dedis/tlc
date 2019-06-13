
This repository contains multiple prototype implementations of
Threshold Logical Clocks (TLC) and Que Sera Consensus (QSC).

* [go/model](go/model/) contains a minimalistic "model" implementation
  of TLC and QSC in Go, which illustrates the key concepts
  using goroutines and shared memory communication for simplicity.
  It is not useful in an actual distributed context,
  but being less than 250 code lines long,
  it is ideal for studying and understanding TLC and QSC.
  Porting this model implementation to some other language may
  be the best way to get a handle on the fundamentals.

* [go/dist](go/dist/) contains a simple but functional
  "real" distributed implementation of TLC and QSC
  for a fail-stop (Paxos-like) threat model.
  It uses TCP, TLS encryption and authentication,
  and Go's native Gob encoding for inter-node communication.
  At leass than 1000 code lines long
  as measured by [cloc](https://github.com/AlDanial/cloc),
  it is still probably one of the simplest implementations
  of asynchronous consensus around.

* [spin](spin/) contains a simple Promela model of the core of TLC and QSC
  for the [Spin model checker](http://spinroot.com/spin/whatispin.html).
  Although this implementation models TLC and QSC only at a
  very high, abstract level, it captures the basic logic enough
  to lend confidence to the correctness of the algorithm.

All of this code is still extremely early and experimental.

