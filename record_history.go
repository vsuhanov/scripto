package main

import (
	"flag"
	"io"
	"log"
	"strings"
	"time"

	"github.com/vsuhanov/scripto/internal/services"
	"github.com/vsuhanov/scripto/internal/tui"
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

	command := strings.Join(fs.Args(), " ")
	hasCommand := strings.TrimSpace(command) != ""

	switch {
	case exitProvided && hasCommand:
		err = service.RecordComplete(*id, command, *cwd, *session, services.ShellHistorySourceShell, *started, *finished, *exitCode)
	case exitProvided:
		err = service.RecordFinish(*id, *exitCode, *finished)
	case hasCommand:
		err = service.RecordStart(*id, command, *cwd, *session, services.ShellHistorySourceShell, *started)
	default:
		log.Printf("__record-history: empty command for id %q", *id)
		return 0
	}
	if err != nil {
		log.Printf("__record-history: %v", err)
	}
	return 0
}

func handleRecordExecution(args []string) int {
	fs := flag.NewFlagSet("__record-execution", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	id := fs.String("id", "", "execution id written by scripto to SCRIPTO_EXEC_ID_FILE")
	exitCode := fs.Int("exit", 0, "exit code of the executed command")
	finished := fs.Int64("finished", 0, "unix timestamp when the command finished")

	if err := fs.Parse(args); err != nil {
		log.Printf("__record-execution: failed to parse arguments: %v", err)
		return 0
	}
	if *id == "" {
		log.Printf("__record-execution: missing --id")
		return 0
	}
	if *finished == 0 {
		*finished = time.Now().Unix()
	}

	executions, err := services.NewExecutionHistoryService()
	if err != nil {
		log.Printf("__record-execution: failed to open execution history: %v", err)
		return 0
	}
	defer executions.Close()

	if err := executions.RecordResult(*id, *exitCode, *finished); err != nil {
		log.Printf("__record-execution: %v", err)
	}
	if err := services.NewShellHistoryServiceWithDB(executions.DB()).MarkFinished(tui.ScriptoRunHistoryID(*id), *exitCode, *finished); err != nil {
		log.Printf("__record-execution: %v", err)
	}
	return 0
}
