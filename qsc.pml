
#define N	4		// total number of nodes
#define Fa	1		// max number of availability failures
#define Fcu	1		// max number of unknown correctness failures
#define T	(Fa+Fcu+1)	// consensus threshold required

#define STEPS	3		// TLC time-steps per consensus round
#define ROUNDS	1		// number of consensus rounds to run
#define TICKETS	10		// proposal lottery ticket space


typedef Round {
	bit sent[STEPS];	// whether we've sent yet each time step
	byte ticket;		// lottery ticket assigned to proposal at t+0
	byte seen[STEPS];	// bitmask of msgs we've seen from each step
}

typedef Node {
	Round round[ROUNDS];	// each node's per-consensus-round information
}

Node node[N];			// all state of each node


proctype NodeProc(byte n) {
	byte rnd, tkt, step, seen, scnt, nn;
	bool correct = (n < T);

	printf("Node %d correct %d\n", n, correct);

	for (rnd : 0 .. ROUNDS-1) {

		// select a "random" (here just arbitrary) ticket
		select (tkt : 1 .. TICKETS);
		node[n].round[rnd].ticket = tkt;

		// Run the round to completion
		for (step : 0 .. STEPS-1) {

			// "send" the broadcast for this time-step
			node[n].round[rnd].sent[step] = 1;

			// collect a threshold of other nodes' broadcasts
			seen = 1 << n		// we've already seen our own
			scnt = 1
			do
			::	// Pick another node to try to 'receive' from
				select (nn : 1 .. N); nn--;
				if
				:: (((seen & (1 << nn)) == 0) && 
				    (node[nn].round[rnd].sent[step] != 0)) ->
					printf("%d received from %d\n", n, nn);
					seen = seen | 1 << nn;
					scnt++;
				:: else -> skip
				fi

				// Threshold test: have we seen enough?
				if
				:: scnt >= T -> break;
				:: else -> skip;
				fi
			od

			printf("%d step %d complete\n", n, step);
		}

		printf("%d round %d complete\n", n, rnd);
	}
}


init {
	atomic {
		int i;
		for (i : 0 .. N-1) {
			run NodeProc(i)
		}
	}
}

