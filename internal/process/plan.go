// Package process owns bounded child-process plans, execution, capture, and
// platform-specific process replacement.
package process

type Plan struct {
	Executable string
	Args       []string
	Env        []string
	Stdin      string
	Replace    bool
}
