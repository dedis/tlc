package qscod

import (
	"math/rand"
	"sync"
)

type Node int
type Step int64

type Hist struct {
	node Node
	pred *Hist
	msg  string
	pri  int64
}

type Set map[Node]*Hist

func (S Set) best() (*Hist, bool) {
	b, u := &Hist{pri: -1}, false
	for _, h := range S {
		if h.pri >= b.pri {
			u = !(h.pri == b.pri)
			b = h
		}
	}
	return b, u
}

type Key struct {
	q, s int // QSC consensus round and TLCR step, respectively
}

type Val struct {
	H, Hp *Hist
	R, B  Set
}

type Store interface {
	WriteRead(Step, Val) Val
}

type Client struct {
	tr, tb, ts int     // TLCB thresholds
	kv         []Store // Node state key/value stores

	mut  sync.Mutex
	cond *sync.Cond

	msg string               // message we want to commit, "" if none
	kvc map[Step]map[Node]Val // cache of key/value store values

	Deliver func(*Hist)
}

func (c *Client) Init(tr, tb, ts int, kv []Store) {
	c.tr, c.tb, c.ts, c.kv = tr, tb, ts, kv

	c.cond = sync.NewCond(&c.mut)
	c.kvc = make(map[Step]map[Node]Val)

	for i := range kv {
		go c.thread(Node(i))
	}
}

func (c *Client) thread(node Node) {
	c.mut.Lock()
	s := Step(0)
	h := &Hist{}
	for {
		for c.msg == "" {
			c.cond.Wait()
		}

		v0 := Val{H: h, Hp: &Hist{node, h, c.msg, rand.Int63()}}
		v0, R0, B0 := c.tlcb(node, s+0, v0)

		v2 := Val{R: R0, B: B0}
		v2.H, _ = B0.best()
		v2, R2, B2 := c.tlcb(node, s+2, v2)

		h, _ = R2.best()
		if b, u := R0.best(); B2[h.node] == h && b == h && u {
			c.Deliver(h)
		}

		s += 4
	}
}

func (c *Client) tlcb(node Node, s Step, v0 Val) (Val, Set, Set) {

	v0, v0r := c.tlcr(node, s+0, v0)

	v1 := Val{R: make(Set)}
	for i, v := range v0r {
		v1.R[i] = v.Hp
	}
	v1, v1r := c.tlcr(node, s+1, v1)

	R, B, Bc := make(Set), make(Set), make([]int, len(c.kv))
	for _, v := range v1r {
		for j, h := range v.R {
			R[j] = h
			Bc[j]++
			if Bc[j] >= c.ts {
				B[j] = h
			}
		}
	}
	return v0, R, B
}

func (c *Client) tlcr(node Node, s Step, v Val) (Val, map[Node]Val) {
	if _, ok := c.kvc[s]; !ok {
		c.kvc[s] = make(map[Node]Val)
	}
	v = c.kv[node].WriteRead(s, v)
	if len(c.kvc[s]) == c.tr {
		c.cond.Broadcast()
	}
	for len(c.kvc[s]) < c.tr {
		c.cond.Wait()
	}
	return v, c.kvc[s]
}
