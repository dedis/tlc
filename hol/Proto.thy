theory Proto
imports Main
begin



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
