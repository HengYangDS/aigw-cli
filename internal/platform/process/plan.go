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
	// DebugProcess enables native Windows debug events for an untimed observer.
	// The observer must drain or detach its events before returning.
	DebugProcess bool
	// OnCreated runs once the operating system has created the process, before
	// the runner owns it: on Windows it is then suspended and outside its Job.
	// It must return at once and must not panic.
	OnCreated func()
	// OnStart observes the exact owned process before waiting; refusal still reclaims it.
	OnStart func(*os.Process) error
}
