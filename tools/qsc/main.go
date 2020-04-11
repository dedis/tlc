package main

import (
	"fmt"
	//"flag"
	"errors"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/bford/cofo/cri"

	"github.com/dedis/tlc/go/model/qscod/fs/store"
)

var verbose bool = false

const usageStr = `
The qsc command provides tools using Que Sera Consensus (QSC).

Usage:

	qsc <kind> <command> [arguments]

The consensus group kinds are:

	string		Consensus on simple strings
	git		Consensus on Git repositories
	hg		Consensus on Mercurial repositories

The commands are:
	init

`

func usage(usageString string) {
	fmt.Println(usageString)
	os.Exit(1)
}

type kind int

const (
	stringKind kind = iota
	gitKind
	hgKind
)

func main() {
	if len(os.Args) < 3 {
		usage(usageStr)
	}

	// Parse consensus group kind
	var k kind
	switch os.Args[1] {
	case "string":
		k = stringKind
	case "git":
		k = gitKind
	case "hg":
		k = hgKind
	default:
		usage(usageStr)
	}
	_ = k // XXX

	nerr := 0
	switch os.Args[2] {
	case "init":
		initCommand(os.Args[3:])
	default:
		usage(usageStr)
	}

	if nerr > 0 {
		os.Exit(1)
	}
}

func initCommand(args []string) {
	if len(args) < 1 {
		usage(initUsageStr)
	}

	paths, err := parseGroup(args[0])
	if err != nil {
		log.Fatal(err)
	}

	_ = paths
	store := make([]store.FileStore, len(paths))
	for i, path := range paths {
		if err := store[i].Init(path, true, true); err != nil {
			log.Fatal(err)
		}
	}
}

const initUsageStr = `
Usage: qsc <kind> init <group>

where <kind> is the consensus group kind
and <group> specifies the consensus group
as a composable resource identifier (CRI).
For example:

	qsc git init qsc[host1:path1,host2:path2,host3:path3]
`

func parseGroup(group string) ([]string, error) {

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
	println("rawurl:", rawurl)
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
