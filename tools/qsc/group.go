package main

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"math/rand"

	"github.com/bford/cofo/cri"

	"github.com/dedis/tlc/go/model/qscod"
	"github.com/dedis/tlc/go/model/qscod/fs/store"
)

// Group represents a QSC consensus group.
// XXX move to a suitable generic package.
type Group struct {
	ri string            // Resource identifier for this group
	fs []store.FileStore // Persistent store for each member
	kv []qscod.Store	// Store interfrace pointers for qscod

	f, tr, ts int // Threshold parameters for this group
}

// Open a consensus group identified by the resource identifier ri.
// Creates the group if create is true; otherwise opens existing group state.
//
// Supports composable resource identifier (CRI) as preferred group syntax
// because CRIs cleanly suppport nesting of resource identifiers.
//
func (g *Group) Open(ri string, create bool) error {

	// Parse the group resource identifier into individual members
	paths, err := parseGroupRI(ri)
	if err != nil {
		return err
	}
	n := len(paths) // number of members in the consensus group

	// Open or create the file stores they use
	g.fs = make([]store.FileStore, n)
	g.kv = make([]qscod.Store, n)
	for i, path := range paths {
		if err := g.fs[i].Init(path, create, create); err != nil {
			return err
		}
		g.kv[i] = &g.fs[i]
	}

	// Calculate default threshold parameters.
	// XXX make these configurable via CRI options
	g.f = n / 3    // Tolerate at most one-third of nodes failing
	g.tr = n - g.f // Receive threshold
	g.ts = g.f + 1 // Spread threshold

	return nil
}

// Parse a group resource identifier into individual member identifiers.
func parseGroupRI(group string) ([]string, error) {

	// Allow just '[...]' as a command-line shorthand for 'qsc[...]'
	if len(group) > 0 && group[0] == '[' {
		group = "qsc" + group
	}

	// Parsing it as an actual CRI/URI is kind of unnecessary so far,
	// but may get more interesting with query-string options and such.
	rawurl, err := cri.URI.From(group)
	if err != nil {
		return nil, err
	}
	//println("rawurl:", rawurl)
	url, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if url.Scheme != "qsc" {
		return nil, errors.New("consensus groups must use qsc scheme")
	}

	// Parse the nested member paths from the opaque string in the URL.
	str, path := url.Opaque, ""
	var paths []string
	for str != "" {
		if i := strings.IndexByte(str, ','); i >= 0 {
			path, str = str[:i], str[i+1:]
		} else {
			path, str = str, ""
		}
		paths = append(paths, path)
	}
	if len(paths) < 3 {
		return nil, errors.New(
			"consensus groups must have minimum three members")
	}

	return paths, nil
}

// Determine the last commit that was written to the Store,
// based on the perspectives of a quorum of the member nodes.
func (g *Group) LastCommit() qscod.Head {

	mut := &sync.Mutex{}
	cond := sync.NewCond(mut)
	count := 0
	var lastCommit qscod.Head

	thread := func(i int) {

		// Asynchronously find the last known committed value
		// according to this underlying persistent store
		h := g.fs[i].LastCommit()
		//println("thread", i, "lastCommit", h.Step)

		// Find the best (most recent) consensus across stores
		mut.Lock()
		if h.Step > lastCommit.Step {
			lastCommit = h
		}
		count++
		if count == g.tr {
			cond.Broadcast()
		}
		mut.Unlock()
	}

	// Launch a thread for each underlying FileStore
	for i := range g.fs {
		go thread(i)
	}

	// Wait until enough of them have read their latest committed Head
	mut.Lock()
	for count < g.tr {
		cond.Wait()
	}
	mut.Unlock()

	return lastCommit
}

// Commit new string newData, if it hasn't change since lastCommit.
// Returns whatever was next committed after lastCommit in either case.
//
func (g *Group) TryCommit(lastCommit qscod.Head, newData string) qscod.Head {

	rv := rand.Int63	// XXX use crypto randomness in production

	return qscod.Commit(g.tr, g.ts, g.kv, rv, newData, lastCommit)
}
