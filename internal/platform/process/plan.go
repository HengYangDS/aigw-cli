// Package process owns bounded child-process plans and captured execution.
package process

import "os"

// Plan is a complete child-process invocation, including its explicit environment and standard input.
type Plan struct {
	Executable string
	Directory  string
	Args       []string
	Env        []string
	Stdin      string
	// OnCreated runs once the operating system has created the process in its
	// own process group, before the runner observes it through OnStart.
	// It must return at once and must not panic.
	OnCreated func()
	// OnStart observes the exact owned process before waiting; refusal still reclaims it.
	OnStart func(*os.Process) error
}
