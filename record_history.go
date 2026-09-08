package main

import (
	"flag"
	"io"
	"log"
	"strings"

	"github.com/vsuhanov/scripto/internal/services"
)

func handleRecordHistory(args []string) int {
	fs := flag.NewFlagSet("__record-history", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	id := fs.String("id", "", "unique identifier for the command invocation")
	cwd := fs.String("cwd", "", "working directory the command ran in")
	session := fs.String("session", "", "shell session identifier")
	started := fs.Int64("started", 0, "unix timestamp when the command started")
	finished := fs.Int64("finished", 0, "unix timestamp when the command finished")
	exitCode := fs.Int("exit", -1, "exit code of the finished command")

	if err := fs.Parse(args); err != nil {
		log.Printf("__record-history: failed to parse arguments: %v", err)
		return 0
	}

	if *id == "" {
		log.Printf("__record-history: missing --id")
		return 0
	}

	service, err := services.NewShellHistoryService()
	if err != nil {
		log.Printf("__record-history: failed to open shell history service: %v", err)
		return 0
	}
	defer service.Close()

	exitProvided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "exit" {
			exitProvided = true
		}
	})

	if exitProvided {
		if err := service.RecordFinish(*id, *exitCode, *finished); err != nil {
			log.Printf("__record-history: %v", err)
		}
		return 0
	}

	command := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(command) == "" {
		log.Printf("__record-history: empty command for id %q", *id)
		return 0
	}

	if err := service.RecordStart(*id, command, *cwd, *session, services.ShellHistorySourceShell, *started); err != nil {
		log.Printf("__record-history: %v", err)
	}
	return 0
}
