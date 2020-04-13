package main

import (
	"os"
	"log"
	"fmt"
)


func stringCommand(args []string) {
	switch args[0] {
	case "init":
		stringInitCommand(args[1:])
	case "get":
		stringGetCommand(args[1:])
	case "set":
		stringSetCommand(args[1:])
	default:
		usage(usageStr)
	}
}


func stringInitCommand(args []string) {
	if len(args) < 1 {
		usage(stringInitUsageStr)
	}

	// Create the consensus group state on each member node
	var g Group
	err := g.Open(args[0], true)
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


func stringGetCommand(args []string) {
	if len(args) < 1 {
		usage(stringGetUsageStr)
	}

	// Open the file stores
	var g Group
	err := g.Open(args[0], false)
	if err != nil {
		log.Fatal(err)
	}

	// Find a consensus view of the last known commit
	h := g.LastCommit()
	fmt.Printf("commit %d state %q\n", h.Step, h.Data)
}

const stringGetUsageStr = `
Usage: qsc string get <group>

where <group> specifies the consensus group.
Reads and prints the version number and string last committed.
`


func stringSetCommand(args []string) {
	if len(args) < 3 {
		usage(stringSetUsageStr)
	}
	old := args[1]
	new := args[2]

	// Open the file stores
	var g Group
	err := g.Open(args[0], false)
	if err != nil {
		log.Fatal(err)
	}

	// Find a consensus view of the last known commit
	h := g.LastCommit()

	// If the commit changed since the <old> data, report it and abort.
	if h.Data != old {
		fmt.Printf("commit %d state %q\n", h.Step, h.Data)
		os.Exit(1)
	}

	// Try to commit the new value.
	hn := g.TryCommit(h, new)
	fmt.Printf("commit %d state %q\n", hn.Step, hn.Data)

	// Return success only if the next commit was what we wanted
	if hn.Data != new {
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

