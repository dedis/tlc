
This repository contains multiple prototype implementations of
Threshold Logical Clocks (TLC) and Que Sera Consensus (QSC),
as described in the following papers:

* [Threshold Logical Clocks for Asynchronous Distributed Coordination and Consensus](https://arxiv.org/abs/1907.07010)
* [Que Sera Consensus: Simple Asynchronous Agreement with Private Coins and Threshold Logical Clocks](https://arxiv.org/abs/2003.02291)

The following prototype implementations of TLC and QSC are available
in multiple languages:

* [erlang/model](erlang/model/) contains a minimalistic model implementation
  of the QSC, TLCB, and TLCR algorithms detailed in the
  [new QSC preprint](https://arxiv.org/abs/2003.02291).
  This model implements QSC using Erlang processes and communication 
  on a single machine for illustrative simplicity, although
  [distributed Erlang](https://erlang.org/doc/reference_manual/distributed.html)
  should make it straightforward to extend this model
  to true distributed consensus.
  Erlang's [selective receive](https://ndpar.blogspot.com/2010/11/erlang-explained-selective-receive.html)
  is particularly well-suited to implementing TLCR concisely.
  The model consists of only 73 lines of code
  as measured by [cloc](https://github.com/AlDanial/cloc),
  including test code,
  or only 37 lines comprising the consensus algorithm alone.

* [go/model](go/model/) contains a minimalistic model implementation in Go
  of TLC and QSC as described in the
  [original TLC preprint](https://arxiv.org/abs/1907.07010).
  This model illustrates the key concepts
  using goroutines and shared memory communication for simplicity.
  It is not useful in an actual distributed context,
  but being less than 200 code lines long
  as measured by [cloc](https://github.com/AlDanial/cloc),
  it is ideal for studying and understanding TLC and QSC.

* [go/model/qscod](go/model/qscod/)
  contains a model implementation in Go of QSCOD,
  the client-driven "on-demand" consensus algorithm outlined in the 
  [new QSC preprint](https://arxiv.org/abs/2003.02291).
  This formulation of QSC consumes no bandwidth or computation
  when there is no work to be done (hence on-demand),
  and incurs only <i>O(n<sup>2</sup>)</i> communication complexity
  per client-driven agreement.

* [go/dist](go/dist/) contains a simple but working
  "real" distributed implementation of TLC and QSC in Go
  for a fail-stop (Paxos-like) threat model.
  It uses TCP, TLS encryption and authentication,
  and Go's native Gob encoding for inter-node communication.
  At less than 1000 code lines long
  as measured by [cloc](https://github.com/AlDanial/cloc),
  it is still probably one of the simplest implementations
  of asynchronous consensus around.

* [spin](spin/) contains a simple Promela model of the core of TLC and QSC
  for the [Spin model checker](https://spinroot.com/spin/whatispin.html).
  Although this implementation models TLC and QSC only at a
  very high, abstract level, it captures the basic logic enough
  to lend confidence to the correctness of the algorithm.

All of this code is still extremely early and experimental;
use at your own risk.

[![Build Status](https://travis-ci.com/dedis/tlc.svg?branch=master)](https://travis-ci.com/dedis/tlc)

