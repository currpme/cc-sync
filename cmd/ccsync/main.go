package main

import (
	"fmt"
	"os"

	"ccsync/internal/app"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := app.Run(os.Args[1:], app.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
