package main

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/bford/cofo/cri"

	"github.com/dedis/tlc/go/lib/cas"
	"github.com/dedis/tlc/go/lib/fs/casdir"
	"github.com/dedis/tlc/go/model/qscod/qscas"
)

// Group represents a QSC consensus group.
// XXX move to a suitable generic package.
type group struct {
	qscas.Group
}

// Open a consensus group identified by the resource identifier ri.
// Creates the group if create is true; otherwise opens existing group state.
//
// Supports composable resource identifier (CRI) as preferred group syntax
// because CRIs cleanly suppport nesting of resource identifiers.
//
func (g *group) Open(ctx context.Context, ri string, create bool) error {

	// Parse the group resource identifier into individual members
	paths, err := parseGroupRI(ri)
	if err != nil {
		return err
	}
	n := len(paths) // number of members in the consensus group

	// Create a POSIX directory-based CAS interface to each store
	stores := make([]cas.Store, n)
	for i, path := range paths {
		st := &casdir.Store{}
		if err := st.Init(path, create, create); err != nil {
			return err
		}
		stores[i] = st
	}

	// Start a CAS-based consensus group across this set of stores,
	// with the default threshold configuration.
	// (XXX make this configurable eventually.)
	g.Group.Start(ctx, stores, -1)

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
