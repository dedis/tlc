-module(qsc).
-export([qsc/1, test/0]).

% Node configuration is a tuple defined as a record.
% nn: node number from 1..len(pids)
% tr: receive threshold
% tb: broadcast threshold
% ts: spread threshold
% pids: list of process IDs of all nodes
% steps: maximum number of time steps to run, nil to run forever
% choose: function choose(Config, Step) -> Msg to choose application message
% random: function random() -> Value to choose a random priority value
% deliver: function deliver(Config, Step, History) to deliver a final history
-record(config, {nn, tr, tb, ts, pids, steps, choose, random, deliver}).

% A history is a record representing the most recent in a chain.
-record(hist, {step, nn, msg, pri, pred}).

% qsc(C) -> (never returns)
% Implements Que Sera Co nsensus (QSC) atop TLCB and TLCR.
qsc(C) -> qsc(C, 1, #hist{step=0}).	% start at step 1 with placeholder pred
qsc(#config{steps=Max}, S0, _) when S0 > Max -> {};	% stop after Max steps
qsc(#config{nn=I, choose=Ch, random=Rv, deliver=D} = C, S0, H0) ->
	H1 = #hist{step=S0, nn=I, msg=Ch(C, S0), pri=Rv(), pred=H0},
	{S1, R1, B1} = tlcb(C, S0, H1),	% Try to broadcast (confirm) proposal
	{H2, _} = best(B1),		% Choose some best eligible proposal
	{S2, R2, B2} = tlcb(C, S1, H2),	% Re-broadcast it to reconfirm proposal
	{Hn, _} = best(R2),		% Choose best eligible for next round
	{H3, Unique} = best(R1),	% What is the best potential history?
	Final = lists:member(Hn, B2) and (Hn == H3) and Unique,
	if	Final -> D(C, S2, Hn), qsc(C, S2, Hn);	% Deliver history Hn
		true -> qsc(C, S2, Hn)	% Just proceed to next consensus round
	end.

% best(L) -> {B, U}
% Find and return the best (highest-priority) history B in a nonempty list L,
% and a flag U indicating whether B is uniquely best (highest priority) in L.
best([H]) -> {H, true};			% trivial singleton case
best(L) ->
	Compare = fun(#hist{pri=AR}, #hist{pri=BR}) -> AR >= BR end,
	[#hist{pri=BR} = B, #hist{pri=NR} | _] = lists:sort(Compare, L),
	{B, (BR /= NR)}.


% tlcb(C, S, H) -> {S, R, B}
% Implements the TLCB algorithm for full-spread synchronous broadcast.
tlcb(#config{ts=Ts} = C, S0, H) ->
	{S1, R1, _} = tlcr(C, S0, H),	% Step 1: broadcast history H
	{S2, R2, _} = tlcr(C, S1, R1),	% Step 2: re-broadcast list we received
	R = sets:to_list(sets:union([sets:from_list(L) || L <- [R1 | R2]])),
	B = [Hc || Hc <- R, count(R2, Hc) >= Ts],
	{S2, R, B}.			% New state, receive and broadcast sets

% count(LL, H) -> N
% Return N the number of lists in list-of-lists LL that include history H.
count(LL, H) -> length([L || L <- LL, lists:member(H, L)]).


% tlcr(C, S, M) -> {S, R, nil}
% Implements the TLCR algorithm for receive-threshold synchronous broadcast.
tlcr(#config{pids=Pids} = C, S, M) ->
	[P ! {S, M} || P <- Pids],		% broadcast next message
	tlcr_wait(C, S, []).			% wait for receive threshold
tlcr_wait(#config{tr=Tr} = C, S, R) when length(R) < Tr ->
	receive	{RS, RM} when RS == S -> tlcr_wait(C, S, [RM | R]);
		{RS, _} when RS < S -> tlcr_wait(C, S, R)	% drop old msgs
	end;	% when RS > S message stays in the inbox to be received later
tlcr_wait(_, S, R) -> {S+1, R, nil}.


% Run a test-case configured for a given number of potentially-failing nodes F,
% then signal Parent process when done.
test_run(F, Steps) ->
	% Generate a standard valid configuration from number of failures F.
	N = 3*F, Tr = 2*F, Tb = F, Ts = F+1,
	io:fwrite("Test N=~p F=~p~n", [N, F]),

	% Function to choose message for node I to propose at TLC time-step S.
	Choose = fun(#config{nn=I}, S) -> {msg, S, I} end,

	% Choose a random value to attach to a proposal in time-step S.
	% This low-entropy random distribution is intended only for testing,
	% so as to ensure a significant rate of ties for best priority.
	% Production code should use high-entropy cryptographic randomness for
	% maximum efficiency and strength against intelligent DoS attackers.
	Random = fun() -> rand:uniform(N) end,

	% Spawn a process to receive and consistency-check committed histories.
	Checker = spawn(fun() -> test_checker(#hist{step=0}) end),

	% The nodes will "deliver" histories by sending them back to us.
	Deliver = fun(C, S, H) -> Checker ! {check, S, C#config.nn, H} end,

	% Launch a process representing each of the N nodes.
	Self = self(),
	Pids = [spawn(fun() -> test_node(Self) end) || _ <- lists:seq(1, N)],

	% Send each node its complete configuration record to get it started.
	C = #config{ tr = Tr, tb = Tb, ts = Ts, pids = Pids, steps = Steps,
		choose = Choose, random = Random, deliver = Deliver},
	[lists:nth(I, Pids) ! C#config{nn=I} || I <- lists:seq(1, N)],

	% Wait until all nodes run the designated number of time steps.
	[test_wait(I) || I <- lists:seq(1, N)],
	Checker ! {stop}.		% Terminate our checker process

% Receive a node configuration, run a QSC node simulation with it,
% then send a completion signal to our parent process.
test_node(Parent) ->
	receive #config{} = C -> qsc(C), Parent ! {done, C#config.nn} end.

% Wait to receive a signal that node I is finished.
test_wait(I) -> receive {done, I} -> {} end.

% test_checker() -> {}
% Receive committed histories from all nodes and consistency-check them
test_checker(Hp) ->
	receive	{check, S, I, H} ->
			io:fwrite("at ~p node ~p committed ~P~n", [S, I, H, 8]),
			test_checker(test_check(Hp, H));
		{stop} -> {}
	end.

% test_check(A, B) -> H
% Check two histories A and B for consistency, and return the longer one.
test_check(#hist{step=AC,pred=AP} = A, #hist{step=BC} = B) when AC > BC ->
	test_check(AP, B), A;		% compare shorter prefix of A with B
test_check(#hist{step=AC} = A, #hist{step=BC,pred=BP} = B) when BC > AC ->
	test_check(A, BP), B;		% compare A with shorter prefix of B
test_check(A, B) when A == B -> A;
test_check(A, B) -> erlang:error({inconsistency, A, B}).

% Run QSC and TLC through a test suite.
test() ->
	[test_run(F, 1000) || F <- [1,2,3,4,5]],	% simple test suite
	io:fwrite("Tests completed~n").

