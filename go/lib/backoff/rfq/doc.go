// Package rfq implements responsively-fair queueing (RFQ).
// or distributed queueing? RFDQ
//
// If a server has limited resources to serve a potentially-unlimited
// number of clients (especially in a flash crowd or DDoS attack setting),
// we wish to allocate the server's limited resources fairly among clients,
// so that (for example) fast clients cannot indefinitely starve slower ones.
// This definition of fairness implies wait-freedom, i.e., lack of starvation.
// We would like the server to be able to serve an arbitrary number of clients
// in a fair or at least starvation-free way, with only constant server state.
//
// One approach is for the server to organize the clients into a literal queue,
// with each client responsible for remembering who is next in the queue,
// so the space required for next pointers is distributed among the clients.
// This is what queue-based multiprocessor shared memory locking algorithms do.
// But it works only when the clients are perfectly reliable and trustworthy:
// if not, a single crashed client breaks the chain
// and leaves all clients waiting behind it "dangling" and blocked forever
// (at last without introducing timeouts or similar recovery mechanisms).
//
// Another baseline approach is to have all clients run a backoff algorithm
// when they submit a request that the server must reject due to a full queue.
// This approach might be statistically fair and starvation-free
// if all the clients have similar processing speed and connectivity,
// but clients that are much faster than others can starve slow clients.
// This is because if some number of fast clients can saturate the server,
// re-filling the server's queue only a brief moment after space opens up,
// and a slow client's network round-trip time and/or client-side delay
// add up to significantly more than the server's work-item processing time,
// then each time the slow client attempts to retry it will always find
// that the server's queue has already been filled again by the fast clients.
//
// A next-step solution to ensure approximate fairness across all clients
// would be for the server to propagate maximum backoff delays among clients.
// For example, suppose a slow client attempts to submit a request,
// is rejected due to a full server queue, resubmits it t ms later,
// and is again rejected, increasing its backoff timer to 2t ms.
// If the original t ms value was dominated by client-side or network delays,
// and the work-item processing times for fast clients is significantly less,
// then with independent backoff delays the slow client will be starved.
// But if the server notices that the slow client has backed off to 2ms,
// and in response forces *all* clients to use a maximum backoff of 2ms
// until the slow client's request has been satisfied,
// then the slow client will no longer be starved
// and allocation of the server's resources will be approximately fair.
//
// This approach fails to be responsive, however: the server's response times
// to fast clients are slowed to that of the slowest client at a given time.
// This approach can also greatly underutilize the server's resources:
// the server may be perfectly able to process many work-items every 2ms,
// but has slowed itself down to the rate of the slowest client for fairness.
// Pursuing such strong fairness also creates DoS attack vectors,
// since it is trivial for a misbehaved client simply to pretend to be slow.
// In practice we cannot achieve both perfect fairness and responsiveness:
// utilizing the server's full capacity to service fast clients quickly
// inherently means that fast clients obtain more resources than slow clients.
// But we would still like to be "reasonably" fair while also responsive,
// and particualrly to ensure that no client, however slow, is starved.
//
// RFQ thus provides "responsively-fair queueing",
// which ensures statistical fairness among clients that see similar delays,
// and similarly ensures fairness among clients in different delay classes.
// ...
//
// Server has a limited internal queue, which it keeps sorted
// oldest-request-first as judged by the server's own clock.
// An externally-queued request can bump an internally-queued request
// if the server has previously outsourced it to the client and forgotten it
// but its approximate service time has arrived and the client resubmitted it.
//
// Tolerating misbehaving (Byzantine) clients:
// use server-side MAC (and optionally encryption) to protect the state
// the server outsources to clients.
//
// Issue: replay attacks, since server doesn't have storage to remember
// which tokens have and haven't been "used" or how many times.
// Full processing might reveal and neutralize the effect of a replay --
// e.g., where a cryptocurrency server finds a UTXO was already spent --
// but full processing might be significantly more costly in resources.
// One simple defense is to have the server keep a record (e.g., hash table)
// of all the tokens that have been processed within some past epoch.
// The server can then detect and trivially discard replays within one epoch,
// thereby rate-limiting the effectiveness of token replays to one per epoch.
// This takes storage linear in the epoch length and server's processing rate,
// but is independent of the number clients contending to submit requests.
//
// If it is acceptable to impose a maximum round-trip delay on any client,
// denying service to clients that can't resubmit a request within one epoch,
// then the server can presume requests from earlier epochs to be replays
// and reject them unconditionally, thereby eliminating replay attacks.
//
package rfq
