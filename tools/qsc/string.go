package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

func stringCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		usage(stringUsageStr)
	}
	switch args[0] {
	case "init":
		stringInitCommand(ctx, args[1:])
	case "get":
		stringGetCommand(ctx, args[1:])
	case "set":
		stringSetCommand(ctx, args[1:])
	default:
		usage(stringUsageStr)
	}
}

const stringUsageStr = `
Usage: qsc string <command> [arguments]

The commands for string-value consensus groups are:

	init	initialize a new consensus group
	get	output the current consensus state as a quoted string
	set	change the consensus state via atomic compare-and-set
`

func stringInitCommand(ctx context.Context, args []string) {
	if len(args) != 1 {
		usage(stringInitUsageStr)
	}

	// Create the consensus group state on each member node
	var g group
	err := g.Open(ctx, args[0], true)
	if err != nil {
		log.Fatal(err)
	}
}

const stringInitUsageStr = `
Usage: qsc string init <group>

where <group> specifies the consensus group
as a composable resource identifier (CRI).
For example:

	qsc git init qsc[host1:path1,host2:path2,host3:path3]
`

func stringGetCommand(ctx context.Context, args []string) {
	if len(args) != 1 {
		usage(stringGetUsageStr)
	}

	// Open the file stores
	var g group
	err := g.Open(ctx, args[0], false)
	if err != nil {
		log.Fatal(err)
	}

	// Find a consensus view of the last known commit.
	ver, val, err := g.CompareAndSet(ctx, "", "")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("version %d state %q\n", ver, val)
}

const stringGetUsageStr = `
Usage: qsc string get <group>

where <group> specifies the consensus group.
Reads and prints the version number and string last committed.
`

func stringSetCommand(ctx context.Context, args []string) {
	if len(args) != 3 {
		usage(stringSetUsageStr)
	}

	old := args[1]
	new := args[2]
	if new == "" {
		log.Fatal("The empty string is reserved for the starting state")
	}

	// Open the file stores
	var g group
	err := g.Open(ctx, args[0], false)
	if err != nil {
		log.Fatal(err)
	}

	// Invoke the request compare-and-set operation.
	ver, val, err := g.CompareAndSet(ctx, old, new)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("version %d state %q\n", ver, val)

	// Return success only if the next commit was what we wanted
	if val != new {
		os.Exit(1)
	}
	os.Exit(0)
}

const stringSetUsageStr = `
Usage: qsc string set <group> <old> <new>

where:
<group> specifies the consensus group
<old> is the expected existing value string
<new> is the new value to set if it hasn't yet changed from <old>

Prints the version number and string last committed,
regardless of success or failure.
`
