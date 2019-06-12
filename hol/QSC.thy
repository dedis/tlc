theory QSC
imports Main
begin




types foo = (nat * nat);

(* messages that nodes exchange within a round *)
datatype msg =
    mProp nat			(* proposal with ticket t *)
  | mSkip			(* No-op message just to wait a time-step *)

(* round history as seen by any single node *)
datatype round =
    rInit			(* empty initial round history at step 0 *)
  | rRecv nat nat msg round	(* received at step s from node i message m *)


types roundHist = msg set;	(* messages a node has received in a round *)

types nodesist = nat -> roundHist;



primrec roundBest :: "roundHist -> msg"





(* find the best reconfirmed proposal in the round history *)
primrec bestReconf ::  "roundHist -> opt msg"
    "bestReconf (rInit) == None"
  | "bestReconf (


primrec findSpoiler :: "roundHist -> msg -> bool"

(* check that no other proposal spoils the best reconfirmed proposal *)
primrec checkSpoiled :: "roundHist -> msg option -> msg option"
    "checkSpoiled h (Some m) = ..."
    "checkSpoiled h (None) = ..."


(* determine the round's successful consensus or failure result *)
primrec roundSucc :: "roundHist -> msg option"
	"roundSucc h == checkSpoiled h (bestReconf h)"




type adversary = history -> action


datatype pkt =		(* packet broadcast by some node *)
    tMsg nat nat	(* message sent by node i at at time t *)
  | tAck nat nat	(* acknowledgment at time t of message at ts<t *)

datatype step =
    sSkip nat		(* at time t nothing happens *)
  | sSend nat nat pkt	(* at time t node n broadcasts pkt)
  | sRecv nat nat pkt nat (* at time t node n receives pkt from at time ts<t *)


datatype hist = [step]



primrec sentStep :: "nat \<Rightarrow> step \<Rightarrow> pkt set"
    "sentStep t (sSkip) = {}"
  | "sentStep t (sSend t' node pkt) = (t' == t) && (p' == p)"
  | "sentStep t (sRecv x x x x) = False"


(* set of all messages that have been sent in history h *)
primrec sent :: "hist \<Rightarrow> pkt set"
    "sent (sSkip 


(* set of all valid histories *)
inductive_set hists = 
    hists_empty: "[] \<in> hists"
  | hists_send: "h \<in> hists \<Longrightarrow> (h # sSend (len h) node pkt) \<in> hists"
  | hists_recv: "h \<in> hists AND sent h pkt ts \<Longrightarrow> (h # sRecv (len h) node pkt ts"


end

