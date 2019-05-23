
Require Vector.
Require Extraction.


Inductive Sig : Set :=
  | sig : nat -> Sig.

Inductive Msg : Set :=
  | M : nat -> list Sig -> Msg.

Inductive Recv : Set :=
  | RecvNo : Recv			(* no message received yet *)
  | RecvMsg : Msg -> Recv		(* one unique message received *)
  | RecvEquiv : Msg -> Recv -> Recv.	(* equivocating messages received *)

Inductive Round : Set :=
  | Init : Round
  | Rnd : list Recv -> Round.

Inductive Log : Set :=
  | Log0 : Log					(* empty log *)
  | LogMsg : nat -> nat -> Msg -> Log -> Log.	(* at s from i recieved m *)



Inductive View : Set :=
  | View0 : View			(* empty view *)
  | ViewS : nat -> Recv -> View.	(* from node n message(s) received *)



Definition msg_valid (t : nat) (m : Msg) : Prop :=
  match m with
  | M s sigs => length sigs >= t
  end.


(*
Fixpoint confirmed (s : nat) (m : Msg) (v : View) : Prop :=
  match v with
  | View0 => False
  | ViewS j r => 
*)


Extraction Language Haskell.
Recursive Extraction Round View msg_valid.

