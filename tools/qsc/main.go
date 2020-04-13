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

func main() {
	if len(os.Args) < 3 {
		usage(usageStr)
	}

	// Parse consensus group kind
	switch os.Args[1] {
	case "string":
		stringCommand(os.Args[2:])
//	case "git":
//		k = gitKind
//	case "hg":
//		k = hgKind
	default:
		usage(usageStr)
	}
}

