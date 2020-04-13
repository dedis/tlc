package main

import (
	"fmt"
	//"flag"
	//"log"
	"os"
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
	case "get":
		getCommand(os.Args[3:])
	case "set":
		setCommand(os.Args[3:])
	default:
		usage(usageStr)
	}

	if nerr > 0 {
		os.Exit(1)
	}
}

