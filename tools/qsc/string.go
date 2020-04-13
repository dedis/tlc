package main

import (
	"os"
	"log"
	"fmt"
)

func initCommand(args []string) {
	if len(args) < 1 {
		usage(initUsageStr)
	}

	// Create the consensus group state on each member node
	var g Group
	err := g.Open(args[0], true)
	if err != nil {
		log.Fatal(err)
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


func getCommand(args []string) {
	if len(args) < 1 {
		usage(getUsageStr)
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

const getUsageStr = `
Usage: qsc <kind> get <group>

Returns...
`


func setCommand(args []string) {
	if len(args) < 3 {
		usage(setUsageStr)
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

const setUsageStr = `
Usage: qsc <kind> set <group> <old> <new>

...
`

