// Package process owns bounded child-process plans and captured execution.
package process

// Plan is a complete child-process invocation, including its explicit environment and standard input.
type Plan struct {
	Executable string
	Args       []string
	Env        []string
	Stdin      string
}
