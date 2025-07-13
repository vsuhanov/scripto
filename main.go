package main

import (
	"fmt"
	"os"

	"scripto/commands"
)

// Version information (set during build)
var version = "dev"

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("scripto version %s\n", version)
		return
	}

	commands.Execute()
}
