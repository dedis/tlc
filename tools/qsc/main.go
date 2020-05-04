package main

import (
	"fmt"
	//"flag"
	//"log"
	"context"
	"os"
)

var verbose bool = false

const usageStr = `
The qsc command provides tools using Que Sera Consensus (QSC).

Usage:

	qsc <type> <command> [arguments]

The types of consensus groups are:

	string		Consensus on simple strings
	git		Consensus on Git repositories
	hg		Consensus on Mercurial repositories

Run qsc <type> help for commands that apply to each type.
`

func usage(usageString string) {
	fmt.Println(usageString)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage(usageStr)
	}

	// Create a cancelable top-level context and cancel it when we're done,
	// to shut down asynchronous consensus access operations cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse consensus group kind
	switch os.Args[1] {
	case "string":
		stringCommand(ctx, os.Args[2:])
	default:
		usage(usageStr)
	}
}
