
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
	byte prsn[STEPS];	// bitmaks of proposals we've seen after each
	byte best[STEPS];
	byte btkt[STEPS];
}

typedef Node {
	Round round[ROUNDS];	// each node's per-consensus-round information
}

Node node[N];			// all state of each node


proctype NodeProc(byte n) {
	byte rnd, tkt, step, seen, scnt, prsn, best, btkt, nn, bseen;
	//bool correct = (n < T);

	//printf("Node %d correct %d\n", n, correct);

	for (rnd : 0 .. ROUNDS-1) {

		// select a "random" (here just arbitrary) ticket
		select (tkt : 1 .. TICKETS);
		node[n].round[rnd].ticket = tkt;

		// we've already seen our own proposal
		prsn  = 1 << n;

		// the "best proposal" starts with our own...
		best = n;
		btkt = tkt;

		// Run the round to completion
		for (step : 0 .. STEPS-1) {

			// "send" the broadcast for this time-step
			node[n].round[rnd].sent[step] = 1;

			// collect a threshold of other nodes' broadcasts
			seen = 1 << n;		// we've already seen our own
			scnt = 1;
			do
			::	// Pick another node to try to 'receive' from
				select (nn : 1 .. N); nn--;
				if
				:: (((seen & (1 << nn)) == 0) && 
				    (node[nn].round[rnd].sent[step] != 0)) ->
					printf("%d received from %d\n", n, nn);
					seen = seen | (1 << nn);
					scnt++;

					// Track the best proposal we've seen
					if
					:: step == 0 ->
						prsn = prsn | (1 << nn);
						if
						:: node[nn].round[rnd].ticket < btkt ->
							best = nn;
							btkt = node[nn].round[rnd].ticket;
						:: node[nn].round[rnd].ticket == btkt ->
							best = 255;	// means tied
						:: else -> skip
						fi

					// Track proposals we've seen indirectly
					:: step > 0 ->
						prsn = prsn | node[nn].round[rnd].prsn[step-1];
						if
						:: node[nn].round[rnd].btkt[step-1] < btkt ->
							best = node[nn].round[rnd].best[step-1];
							btkt  = node[nn].round[rnd].btkt[step-1];
						:: (node[nn].round[rnd].btkt[step-1] == btkt) && (node[nn].round[rnd].best[step-1] != best) ->
							best = 255;	// tied
						:: else -> skip
						fi
					:: else -> skip
					fi

				:: else -> skip
				fi

				// Threshold test: have we seen enough?
				if
				:: scnt >= T -> break;
				:: else -> skip;
				fi
			od

			// Record what we've seen for the benefit of others
			node[n].round[rnd].seen[step] = seen;
			node[n].round[rnd].prsn[step] = prsn;
			node[n].round[rnd].best[step] = best;
			node[n].round[rnd].btkt[step] = btkt;

			printf("%d step %d complete: seen %x best %d ticket %d\n", n, step, seen, best, btkt);
		}

		// How many nodes we received time t+2 messages from
		// saw the best proposal we saw?
		bseen = 0;
		for (nn : 0 .. N-1) {
			if
			:: best < 255 &&
			   ((node[n].round[rnd].seen[2] & (1 << nn)) != 0) &&
			   ((node[nn].round[rnd].prsn[1] & (1 << best)) != 0) ->
				bseen++;
			:: else -> skip
			fi
		}
		printf("%d round %d best %d bseen %d\n", n, rnd, best, bseen);

		if
		:: best == 255 ->
			printf("%d round %d failed due to tie\n", n, rnd);
		:: best < 255 && bseen < T ->
			printf("%d round %d may not have committed\n", n, rnd);
		:: else ->
			printf("%d round %d definitely committed\n", n, rnd);

			// Make sure all nodes actually agreed
			// XXX doesn't properly implement eligibility
			for (nn  : 0 .. N-1) {
				if	// wait until node nn finishes
				:: node[nn].round[rnd].btkt[2] != 0 -> skip
				fi
				assert(node[nn].round[rnd].best[2] == best);
			}
		fi
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

