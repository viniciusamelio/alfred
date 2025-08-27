package main

import (
	"github.com/viniciusamelio/alfred/cmd"
)

// Version information (set by build flags)
var (
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "unknown"
)

func main() {
	// Set version info for the CLI
	cmd.SetVersionInfo(Version, BuildTime, Commit)
	cmd.Execute()
}
