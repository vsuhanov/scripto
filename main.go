package main

import (
	"fmt"
	"os"
  "log"

	"scripto/commands"
)

// Version information (set during build)
var version = "dev"

func configureLogger() {
	logFilePath := "/tmp/scripto.log"
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Error creating log file: %v", err)
	}
	log.SetOutput(logFile)
}

func main() {
  configureLogger()
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("scripto version %s\n", version)
		return
	}

	commands.Execute()
}
